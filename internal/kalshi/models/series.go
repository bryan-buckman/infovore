package models

// SettlementSource represents an official source for market determination.
type SettlementSource struct {
	Name string `json:"name,omitempty"`
	URL  string `json:"url,omitempty"`
}

// Series represents a Kalshi series (template for recurring events).
type Series struct {
	Ticker            string             `json:"ticker"`
	Title             string             `json:"title"`
	Category          string             `json:"category"`
	Frequency         string             `json:"frequency"`
	Tags              []string           `json:"tags"`
	SettlementSources []SettlementSource `json:"settlement_sources"`
}

// GetSeriesListResponse is the response from GET /series.
type GetSeriesListResponse struct {
	Series []Series `json:"series"`
}
