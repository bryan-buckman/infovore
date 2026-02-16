package models

// Market represents a Kalshi market (tradable contract).
type Market struct {
	Ticker       string `json:"ticker"`
	EventTicker  string `json:"event_ticker"`
	SeriesTicker string `json:"series_ticker"`
	MarketType   string `json:"market_type"`
	Title        string `json:"title"`
	Subtitle     string `json:"subtitle"`
	Status       string `json:"status"`

	// Price fields (FixedPointDollars format, e.g., "0.7500")
	YesBidDollars    string `json:"yes_bid_dollars"`
	YesAskDollars    string `json:"yes_ask_dollars"`
	NoBidDollars     string `json:"no_bid_dollars"`
	NoAskDollars     string `json:"no_ask_dollars"`
	LastPriceDollars string `json:"last_price_dollars"`

	// 24h comparison
	PreviousYesBidDollars string `json:"previous_yes_bid_dollars"`
	PreviousYesAskDollars string `json:"previous_yes_ask_dollars"`
	PreviousPriceDollars  string `json:"previous_price_dollars"`

	// Volume and liquidity
	Volume24hFP      string `json:"volume_24h_fp"`
	VolumeFP         string `json:"volume_fp"`
	OpenInterestFP   string `json:"open_interest_fp"`
	LiquidityDollars string `json:"liquidity_dollars"`

	// Timestamps
	OpenTime       string `json:"open_time"`
	CloseTime      string `json:"close_time"`
	ExpirationTime string `json:"expiration_time"`

	// Rules
	YesSubTitle string `json:"yes_sub_title"`
	NoSubTitle  string `json:"no_sub_title"`
	RulesURL    string `json:"rules_primary"`
}

// GetMarketsResponse is the response from GET /markets.
type GetMarketsResponse struct {
	Markets []Market `json:"markets"`
	Cursor  string   `json:"cursor"`
}
