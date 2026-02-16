package models

// MarketPosition represents a position in a single market.
type MarketPosition struct {
	Ticker                string `json:"ticker"`
	Position              int    `json:"position"` // Positive = YES contracts, Negative = NO contracts
	PositionFP            string `json:"position_fp"`
	MarketExposure        int    `json:"market_exposure"`         // Cost of position in cents
	MarketExposureDollars string `json:"market_exposure_dollars"` // Cost of position in dollars
	RealizedPnl           int    `json:"realized_pnl"`            // Locked P&L in cents
	RealizedPnlDollars    string `json:"realized_pnl_dollars"`
	TotalTraded           int    `json:"total_traded"` // Total spent in cents
	TotalTradedDollars    string `json:"total_traded_dollars"`
	FeesPaid              int    `json:"fees_paid"` // Fees in cents
	FeesPaidDollars       string `json:"fees_paid_dollars"`
	RestingOrdersCount    int    `json:"resting_orders_count"`
}

// EventPosition represents a position in an event (group of markets).
type EventPosition struct {
	EventTicker          string `json:"event_ticker"`
	TotalCost            int    `json:"total_cost"`
	TotalCostDollars     string `json:"total_cost_dollars"`
	EventExposure        int    `json:"event_exposure"`
	EventExposureDollars string `json:"event_exposure_dollars"`
	RealizedPnl          int    `json:"realized_pnl"`
	RealizedPnlDollars   string `json:"realized_pnl_dollars"`
	FeesPaid             int    `json:"fees_paid"`
	FeesPaidDollars      string `json:"fees_paid_dollars"`
}

// GetPositionsResponse is the response from GET /portfolio/positions.
type GetPositionsResponse struct {
	MarketPositions []MarketPosition `json:"market_positions"`
	EventPositions  []EventPosition  `json:"event_positions"`
	Cursor          string           `json:"cursor"`
}

// GetBalanceResponse is the response from GET /portfolio/balance.
type GetBalanceResponse struct {
	Balance        int `json:"balance"`         // Available balance in cents
	PortfolioValue int `json:"portfolio_value"` // Total portfolio value in cents
}

// GetMarketResponse is the response from GET /markets/{ticker}.
type GetMarketResponse struct {
	Market Market `json:"market"`
}
