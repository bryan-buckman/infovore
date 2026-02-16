package models

// Settlement represents a settled market position from GET /portfolio/settlements.
type Settlement struct {
	Ticker       string `json:"ticker"`
	EventTicker  string `json:"event_ticker"`
	MarketResult string `json:"market_result"` // "yes", "no", "scalar", "void"
	YesCount     int    `json:"yes_count"`
	YesTotalCost int    `json:"yes_total_cost"` // Cost basis in cents
	NoCount      int    `json:"no_count"`
	NoTotalCost  int    `json:"no_total_cost"` // Cost basis in cents
	Revenue      int    `json:"revenue"`       // Payout in cents
	SettledTime  string `json:"settled_time"`  // ISO 8601
	FeeCost      string `json:"fee_cost"`      // FixedPointDollars e.g. "0.3400"
}

// GetSettlementsResponse is the response from GET /portfolio/settlements.
type GetSettlementsResponse struct {
	Settlements []Settlement `json:"settlements"`
	Cursor      string       `json:"cursor"`
}

// Fill represents a matched trade from GET /portfolio/fills.
type Fill struct {
	FillID      string `json:"fill_id"`
	OrderID     string `json:"order_id"`
	Ticker      string `json:"ticker"`
	Side        string `json:"side"`   // "yes" or "no"
	Action      string `json:"action"` // "buy" or "sell"
	Count       int    `json:"count"`
	YesPrice    int    `json:"yes_price"` // Fill price for YES side in cents
	NoPrice     int    `json:"no_price"`  // Fill price for NO side in cents
	IsTaker     bool   `json:"is_taker"`
	CreatedTime string `json:"created_time"` // ISO 8601
	FeeCost     string `json:"fee_cost"`     // FixedPointDollars
}

// GetFillsResponse is the response from GET /portfolio/fills.
type GetFillsResponse struct {
	Fills  []Fill `json:"fills"`
	Cursor string `json:"cursor"`
}
