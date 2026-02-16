package scanner

import (
	"math"
	"sort"
)

// KalshiFee calculates the taker fee per contract on Kalshi.
// Formula: ceil(0.07 × P × (1-P) × 100) / 100
// where P is the contract price in dollars (e.g., 0.80 for an 80¢ contract).
// Returns the fee in dollars per contract.
// Max fee is at P=0.50 → 1.75¢ per contract.
func KalshiFee(price float64) float64 {
	if price <= 0 || price >= 1 {
		return 0
	}
	// Fee in cents, rounded up
	feeCents := math.Ceil(0.07 * price * (1 - price) * 100)
	return feeCents / 100.0
}

// KellyRecommendation contains the Kelly Criterion analysis for a single market.
type KellyRecommendation struct {
	Ticker               string
	Title                string
	Side                 string  // "YES" or "NO"
	AskPrice             float64 // Cost to buy (in dollars, e.g., 0.75)
	BidPrice             float64 // Bid price for this side
	TrueProb             float64 // Estimated true probability (ask + bias edge)
	Edge                 float64 // Fee-adjusted edge (biasEdge - fee)
	Fee                  float64 // Kalshi taker fee per contract
	KellyFraction        float64 // Optimal fraction of bankroll (full Kelly)
	HalfKellyFraction    float64 // Conservative: half Kelly
	QuarterKellyFraction float64 // Very conservative: quarter Kelly
	RecommendedDollars   float64 // Full Kelly dollar amount
	HalfKellyDollars     float64 // Half Kelly dollar amount
	QuarterKellyDollars  float64 // Quarter Kelly dollar amount
}

// FavoriteLongshotEdge returns the estimated edge based on the favorite-longshot
// bias observed in Kalshi prediction markets. The edge is the difference between
// the empirically observed true probability and the market-implied probability (ask price).
//
// Based on calibration studies (Karl Whelan/UCD, CW Data Solutions):
//   - Longshots (<50¢) are systematically overpriced (negative edge)
//   - Favorites (>50¢) are systematically underpriced (positive edge)
//   - The bias is strongest at the extremes and approximately linear through the middle
//
// Returns a value in dollars (e.g., 0.04 means the true probability is 4¢ higher than ask).
func FavoriteLongshotEdge(askPrice float64) float64 {
	switch {
	case askPrice <= 0 || askPrice >= 1:
		return 0
	case askPrice < 0.10:
		// Extreme longshots: heavily overpriced, ~7¢ overvaluation
		return -0.07
	case askPrice < 0.50:
		// Longshots: linear from -5¢ at 10¢ to -2¢ at 50¢
		return lerp(askPrice, 0.10, 0.50, -0.05, -0.02)
	case askPrice < 0.55:
		// Crossover zone: linear from -2¢ at 50¢ to +1¢ at 55¢
		return lerp(askPrice, 0.50, 0.55, -0.02, 0.01)
	case askPrice < 0.85:
		// Favorites: linear from +1¢ at 55¢ to +5¢ at 85¢
		return lerp(askPrice, 0.55, 0.85, 0.01, 0.05)
	default:
		// Strong favorites (85¢+): plateau at +4¢ (diminishing room)
		return 0.04
	}
}

// lerp performs linear interpolation between two values.
func lerp(x, x0, x1, y0, y1 float64) float64 {
	if x1 == x0 {
		return y0
	}
	t := (x - x0) / (x1 - x0)
	return y0 + t*(y1-y0)
}

// CalculateKelly computes the Kelly Criterion fraction for a binary outcome contract.
//
// Uses the favorite-longshot bias to estimate the true probability:
//   - trueProb = askPrice + FavoriteLongshotEdge(askPrice)
//   - You pay askPrice to potentially win $1
//   - Net profit on win = 1 - askPrice
//   - Payout ratio b = (1 - askPrice) / askPrice
//   - Kelly fraction f* = (b*p - q) / b, where p = trueProb, q = 1-p
//
// Returns the Kelly fraction (0 if negative edge), the edge, and the estimated true probability.
func CalculateKelly(askPrice, bidPrice float64) (kellyFraction, edge, trueProb float64) {
	// Guard against invalid inputs
	if askPrice <= 0 || askPrice >= 1 || bidPrice <= 0 || bidPrice >= 1 {
		return 0, 0, 0
	}

	// True probability from favorite-longshot bias, adjusted for fees
	biasEdge := FavoriteLongshotEdge(askPrice)
	fee := KalshiFee(askPrice)
	effectiveEdge := biasEdge - fee
	trueProb = askPrice + effectiveEdge
	edge = effectiveEdge

	// Clamp trueProb to valid range
	if trueProb <= 0 {
		trueProb = 0.001
	}
	if trueProb >= 1 {
		trueProb = 0.999
	}

	// Payout ratio: how much you win per dollar risked
	b := (1 - askPrice) / askPrice

	// Kelly formula: f* = (b*p - q) / b
	p := trueProb
	q := 1 - p
	kelly := (b*p - q) / b

	// Never recommend negative Kelly (no edge = no bet)
	if kelly <= 0 {
		return 0, edge, trueProb
	}

	// Cap at 1.0 (never bet more than 100% of bankroll)
	kelly = math.Min(kelly, 1.0)

	return kelly, edge, trueProb
}

// GenerateRecommendations calculates Kelly Criterion recommendations for a set of markets.
// Only markets with positive edge are included.
// Results are sorted by Kelly fraction descending.
func GenerateRecommendations(markets []FilteredMarket, bankroll float64) []KellyRecommendation {
	var recs []KellyRecommendation

	for _, m := range markets {
		kelly, edge, trueP := CalculateKelly(m.AskPrice, m.BidPrice)
		if kelly <= 0 {
			continue
		}

		recs = append(recs, KellyRecommendation{
			Ticker:               m.Market.Ticker,
			Title:                m.Market.Title,
			Side:                 string(m.Side),
			AskPrice:             m.AskPrice,
			BidPrice:             m.BidPrice,
			TrueProb:             trueP,
			Edge:                 edge,
			Fee:                  KalshiFee(m.AskPrice),
			KellyFraction:        kelly,
			HalfKellyFraction:    kelly / 2,
			QuarterKellyFraction: kelly / 4,
			RecommendedDollars:   kelly * bankroll,
			HalfKellyDollars:     (kelly / 2) * bankroll,
			QuarterKellyDollars:  (kelly / 4) * bankroll,
		})
	}

	// Sort by Kelly fraction descending (best opportunities first)
	sort.Slice(recs, func(i, j int) bool {
		return recs[i].KellyFraction > recs[j].KellyFraction
	})

	return recs
}
