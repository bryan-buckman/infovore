package scanner

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/bryan-buckman/infovore/internal/kalshi/client"
	"github.com/bryan-buckman/infovore/internal/kalshi/config"
	"github.com/bryan-buckman/infovore/internal/kalshi/models"
)

const (
	// Default price range in dollars (75-90 cents).
	DefaultMinPrice = 0.75
	DefaultMaxPrice = 0.90
)

// ScanConfig holds options for a scan run.
type ScanConfig struct {
	Categories []string // Categories to scan (default: ["Politics"])
	MinPrice   float64  // Min contract price in dollars (default: 0.75)
	MaxPrice   float64  // Max contract price in dollars (default: 0.90)
}

// ScanResult holds the output of a completed scan.
type ScanResult struct {
	HTML       string // Rendered HTML report
	Log        string // Text log of what happened during the scan
	NumMarkets int    // Number of filtered markets
}

// RunScan executes a full market scan and returns the HTML report string.
//
// The caller is responsible for storing the result (e.g., in the database).
func RunScan(cfg *config.Config, scanCfg ScanConfig) (*ScanResult, error) {
	var log strings.Builder

	// Defaults
	if len(scanCfg.Categories) == 0 {
		scanCfg.Categories = []string{"Politics"}
	}
	if scanCfg.MinPrice == 0 {
		scanCfg.MinPrice = DefaultMinPrice
	}
	if scanCfg.MaxPrice == 0 {
		scanCfg.MaxPrice = DefaultMaxPrice
	}

	log.WriteString(fmt.Sprintf("Scanning categories: %s\n", strings.Join(scanCfg.Categories, ", ")))
	log.WriteString(fmt.Sprintf("Price range: %.0f¢ - %.0f¢\n", scanCfg.MinPrice*100, scanCfg.MaxPrice*100))

	// Create authenticated client
	apiClient, err := client.New(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create client: %w", err)
	}

	// Fetch all available categories for the report settings UI
	allCategories, _ := apiClient.GetCategories()
	activeSet := make(map[string]bool)
	for _, c := range scanCfg.Categories {
		activeSet[c] = true
	}

	// Fetch portfolio data
	log.WriteString("Fetching portfolio data...\n")
	balance, err := apiClient.GetBalance()
	if err != nil {
		log.WriteString(fmt.Sprintf("Warning: failed to fetch balance: %v\n", err))
		balance = nil
	}

	var positions []models.MarketPosition
	if balance != nil {
		positions, err = apiClient.GetPositions()
		if err != nil {
			log.WriteString(fmt.Sprintf("Warning: failed to fetch positions: %v\n", err))
			positions = nil
		}
	}

	// Enrich positions with market titles and current prices
	var enrichedPositions []EnrichedPosition
	if balance != nil {
		log.WriteString(fmt.Sprintf("Cash balance: $%.2f\n", float64(balance.Balance)/100.0))
		log.WriteString(fmt.Sprintf("Portfolio value: $%.2f\n", float64(balance.PortfolioValue)/100.0))
		log.WriteString(fmt.Sprintf("Open positions: %d\n", len(positions)))

		for _, p := range positions {
			ep := EnrichedPosition{
				Position: p,
				Title:    p.Ticker,
			}
			market, err := apiClient.GetMarket(p.Ticker)
			if err != nil {
				log.WriteString(fmt.Sprintf("  Warning: could not look up %s: %v\n", p.Ticker, err))
			} else {
				ep.Title = market.Title
				if market.Subtitle != "" {
					ep.Title = market.Title + " — " + market.Subtitle
				}
				if p.Position > 0 {
					ep.AskPrice, _ = strconv.ParseFloat(market.YesAskDollars, 64)
					ep.BidPrice, _ = strconv.ParseFloat(market.YesBidDollars, 64)
				} else {
					ep.AskPrice, _ = strconv.ParseFloat(market.NoAskDollars, 64)
					ep.BidPrice, _ = strconv.ParseFloat(market.NoBidDollars, 64)
				}
				ep.SeriesTicker = market.SeriesTicker
				if market.CloseTime != "" {
					if t, err := time.Parse(time.RFC3339, market.CloseTime); err == nil {
						ep.ExpirationStr = t.Format("Jan 2, 2006")
						ep.ExpirationUnix = t.Unix()
					}
				}
				if ep.ExpirationStr == "" && market.ExpirationTime != "" {
					if t, err := time.Parse(time.RFC3339, market.ExpirationTime); err == nil {
						ep.ExpirationStr = t.Format("Jan 2, 2006")
						ep.ExpirationUnix = t.Unix()
					}
				}
				kelly, edge, _ := CalculateKelly(ep.AskPrice, ep.BidPrice)
				ep.Edge = edge
				ep.KellyFraction = kelly
				ep.HalfKellyFraction = kelly / 2
				ep.QuarterKellyFraction = kelly / 4
			}
			enrichedPositions = append(enrichedPositions, ep)
		}
	} else {
		log.WriteString("Portfolio data unavailable — report will omit portfolio section\n")
	}

	// Scan markets from each selected category
	var catMarkets []CategoryMarkets
	totalSeries := 0

	for _, cat := range scanCfg.Categories {
		log.WriteString(fmt.Sprintf("Fetching series for %s...\n", cat))
		series, err := apiClient.GetSeriesList(cat)
		if err != nil {
			log.WriteString(fmt.Sprintf("  Warning: failed to get series for %s: %v\n", cat, err))
			continue
		}
		log.WriteString(fmt.Sprintf("  Found %d series in %s\n", len(series), cat))
		totalSeries += len(series)

		var markets []models.Market
		for i, s := range series {
			log.WriteString(fmt.Sprintf("  [%d/%d] Scanning %s...\n", i+1, len(series), s.Ticker))
			m, err := apiClient.GetMarkets(s.Ticker, "open")
			if err != nil {
				log.WriteString(fmt.Sprintf("    Warning: %v\n", err))
				continue
			}
			if len(m) > 0 {
				log.WriteString(fmt.Sprintf("    Found %d open markets\n", len(m)))
			}
			markets = append(markets, m...)
		}

		catMarkets = append(catMarkets, CategoryMarkets{
			Category: cat,
			Markets:  markets,
		})
	}

	// Filter by price range
	log.WriteString(fmt.Sprintf("Filtering for contracts priced between %.0f¢ and %.0f¢...\n",
		scanCfg.MinPrice*100, scanCfg.MaxPrice*100))
	filtered := FilterByPriceRangeMulti(catMarkets, scanCfg.MinPrice, scanCfg.MaxPrice)
	log.WriteString(fmt.Sprintf("Found %d contracts in price range\n", len(filtered)))
	SortByAskPrice(filtered)

	// Fetch transactions for current calendar year
	yearStart := time.Date(time.Now().Year(), 1, 1, 0, 0, 0, 0, time.UTC)
	var settlements []models.Settlement
	var fills []models.Fill
	marketTitles := make(map[string]string)

	if balance != nil {
		log.WriteString("Fetching settlements and fills for current year...\n")
		settlements, err = apiClient.GetSettlements(yearStart)
		if err != nil {
			log.WriteString(fmt.Sprintf("  Warning: could not fetch settlements: %v\n", err))
		} else {
			log.WriteString(fmt.Sprintf("  Found %d settlements\n", len(settlements)))
		}

		fills, err = apiClient.GetFills(yearStart)
		if err != nil {
			log.WriteString(fmt.Sprintf("  Warning: could not fetch fills: %v\n", err))
		} else {
			log.WriteString(fmt.Sprintf("  Found %d fills\n", len(fills)))
		}

		uniqueTickersMap := make(map[string]bool)
		for _, s := range settlements {
			uniqueTickersMap[s.Ticker] = true
		}
		for _, f := range fills {
			if f.Action == "sell" {
				uniqueTickersMap[f.Ticker] = true
			}
		}
		for ticker := range uniqueTickersMap {
			mkt, err := apiClient.GetMarket(ticker)
			if err == nil {
				title := mkt.Title
				if mkt.Subtitle != "" {
					title = mkt.Title + " — " + mkt.Subtitle
				}
				marketTitles[ticker] = title
			}
		}
	}

	// Build category views
	categoryViews := make([]CategoryView, 0, len(allCategories))
	for _, cat := range allCategories {
		categoryViews = append(categoryViews, CategoryView{
			Name:   cat,
			Active: activeSet[cat],
		})
	}

	// Generate HTML report string
	log.WriteString("Generating report...\n")
	html, err := GenerateHTMLReportString(
		filtered, totalSeries, scanCfg.MinPrice, scanCfg.MaxPrice,
		balance, enrichedPositions, settlements, fills, marketTitles, categoryViews,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to generate report: %w", err)
	}

	log.WriteString("Report generated successfully!\n")

	return &ScanResult{
		HTML:       html,
		Log:        log.String(),
		NumMarkets: len(filtered),
	}, nil
}
