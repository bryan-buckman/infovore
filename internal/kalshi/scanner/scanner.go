package scanner

import (
	"fmt"
	"strconv"
	"time"

	"github.com/bryan-buckman/infovore/internal/kalshi/models"
)

// ContractSide indicates whether this is a YES or NO contract.
type ContractSide string

const (
	SideYes ContractSide = "YES"
	SideNo  ContractSide = "NO"
)

// FilteredMarket contains a market with parsed price information.
type FilteredMarket struct {
	Market     models.Market
	Side       ContractSide // YES or NO
	AskPrice   float64      // The ask price for this side (cost to buy)
	BidPrice   float64      // The bid price for this side (what you'd get selling)
	Expiration time.Time    // Contract expiration/close time
	Category   string       // Source category (e.g., "Politics", "Sports")
}

// CategoryMarkets pairs a category name with its markets for filtering.
type CategoryMarkets struct {
	Category string
	Markets  []models.Market
}

// FilterByPriceRange filters markets where either the YES or NO ask price is within the given range.
// Prices are in dollars (e.g., 0.75 for 75 cents).
// Returns separate entries for YES and NO sides that match.
func FilterByPriceRange(markets []models.Market, minPrice, maxPrice float64) []FilteredMarket {
	return FilterByPriceRangeWithCategory(CategoryMarkets{Markets: markets}, minPrice, maxPrice)
}

// FilterByPriceRangeMulti filters markets from multiple categories by price range.
func FilterByPriceRangeMulti(catMarkets []CategoryMarkets, minPrice, maxPrice float64) []FilteredMarket {
	var filtered []FilteredMarket
	for _, cm := range catMarkets {
		filtered = append(filtered, FilterByPriceRangeWithCategory(cm, minPrice, maxPrice)...)
	}
	return filtered
}

// FilterByPriceRangeWithCategory filters markets from a single category by price range.
func FilterByPriceRangeWithCategory(cm CategoryMarkets, minPrice, maxPrice float64) []FilteredMarket {
	var filtered []FilteredMarket

	for _, m := range cm.Markets {
		expiration := parseTime(m.CloseTime)

		// Check YES side
		yesAsk, err := parsePrice(m.YesAskDollars)
		if err == nil && yesAsk > 0 && yesAsk >= minPrice && yesAsk <= maxPrice {
			yesBid, _ := parsePrice(m.YesBidDollars)
			filtered = append(filtered, FilteredMarket{
				Market:     m,
				Side:       SideYes,
				AskPrice:   yesAsk,
				BidPrice:   yesBid,
				Expiration: expiration,
				Category:   cm.Category,
			})
		}

		// Check NO side
		noAsk, err := parsePrice(m.NoAskDollars)
		if err == nil && noAsk > 0 && noAsk >= minPrice && noAsk <= maxPrice {
			noBid, _ := parsePrice(m.NoBidDollars)
			filtered = append(filtered, FilteredMarket{
				Market:     m,
				Side:       SideNo,
				AskPrice:   noAsk,
				BidPrice:   noBid,
				Expiration: expiration,
				Category:   cm.Category,
			})
		}
	}

	return filtered
}

// parsePrice parses a FixedPointDollars string (e.g., "0.7500") to float64.
func parsePrice(s string) (float64, error) {
	if s == "" {
		return 0, fmt.Errorf("empty price string")
	}
	return strconv.ParseFloat(s, 64)
}

// parseTime parses an ISO 8601 timestamp string.
func parseTime(s string) time.Time {
	if s == "" {
		return time.Time{}
	}
	// Try common formats
	formats := []string{
		time.RFC3339,
		time.RFC3339Nano,
		"2006-01-02T15:04:05Z",
		"2006-01-02T15:04:05-07:00",
	}
	for _, format := range formats {
		if t, err := time.Parse(format, s); err == nil {
			return t
		}
	}
	return time.Time{}
}

// SortByAskPrice sorts filtered markets by ask price (lowest first).
func SortByAskPrice(markets []FilteredMarket) {
	for i := 0; i < len(markets)-1; i++ {
		for j := i + 1; j < len(markets); j++ {
			if markets[j].AskPrice < markets[i].AskPrice {
				markets[i], markets[j] = markets[j], markets[i]
			}
		}
	}
}

// SortByExpiration sorts filtered markets by expiration date (soonest first).
func SortByExpiration(markets []FilteredMarket) {
	for i := 0; i < len(markets)-1; i++ {
		for j := i + 1; j < len(markets); j++ {
			if !markets[j].Expiration.IsZero() && (markets[i].Expiration.IsZero() || markets[j].Expiration.Before(markets[i].Expiration)) {
				markets[i], markets[j] = markets[j], markets[i]
			}
		}
	}
}

// SortBySpread sorts filtered markets by bid-ask spread (tightest first).
func SortBySpread(markets []FilteredMarket) {
	for i := 0; i < len(markets)-1; i++ {
		for j := i + 1; j < len(markets); j++ {
			spreadI := markets[i].AskPrice - markets[i].BidPrice
			spreadJ := markets[j].AskPrice - markets[j].BidPrice
			if spreadJ < spreadI {
				markets[i], markets[j] = markets[j], markets[i]
			}
		}
	}
}
