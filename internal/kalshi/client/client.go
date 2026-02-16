package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/bryan-buckman/infovore/internal/kalshi/auth"
	"github.com/bryan-buckman/infovore/internal/kalshi/config"
	"github.com/bryan-buckman/infovore/internal/kalshi/models"
)

const (
	apiVersion = "/trade-api/v2"

	maxRetries    = 3
	baseBackoff   = 1 * time.Second
	maxBackoff    = 10 * time.Second
	backoffFactor = 2.0
)

// Client is an authenticated Kalshi API client with rate limiting.
type Client struct {
	baseURL    string
	signer     *auth.Signer
	httpClient *http.Client

	// Rate limiting
	rateLimiter *rateLimiter
}

// rateLimiter implements a simple token bucket rate limiter.
type rateLimiter struct {
	mu             sync.Mutex
	requestsPerSec int
	lastRequest    time.Time
	tokens         float64
}

func newRateLimiter(requestsPerSec int) *rateLimiter {
	return &rateLimiter{
		requestsPerSec: requestsPerSec,
		tokens:         float64(requestsPerSec),
		lastRequest:    time.Now(),
	}
}

func (rl *rateLimiter) Wait() {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(rl.lastRequest).Seconds()

	// Refill tokens based on elapsed time
	rl.tokens += elapsed * float64(rl.requestsPerSec)
	if rl.tokens > float64(rl.requestsPerSec) {
		rl.tokens = float64(rl.requestsPerSec)
	}

	// If we don't have a token, wait
	if rl.tokens < 1 {
		waitTime := time.Duration((1 - rl.tokens) / float64(rl.requestsPerSec) * float64(time.Second))
		time.Sleep(waitTime)
		rl.tokens = 0
	} else {
		rl.tokens--
	}

	rl.lastRequest = time.Now()
}

// New creates a new authenticated Kalshi API client.
func New(cfg *config.Config) (*Client, error) {
	var signer *auth.Signer
	var err error
	if len(cfg.PrivateKeyData) > 0 {
		signer, err = auth.NewSignerFromBytes(cfg.APIKeyID, cfg.PrivateKeyData)
	} else {
		signer, err = auth.NewSigner(cfg.APIKeyID, cfg.PrivateKeyPath)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to create signer: %w", err)
	}

	return &Client{
		baseURL: cfg.BaseURL,
		signer:  signer,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		rateLimiter: newRateLimiter(20), // 20 requests/sec for Basic tier
	}, nil
}

// isRetryable returns true for status codes that warrant a retry.
func isRetryable(statusCode int) bool {
	return statusCode == http.StatusServiceUnavailable ||
		statusCode == http.StatusBadGateway ||
		statusCode == http.StatusGatewayTimeout ||
		statusCode == http.StatusTooManyRequests
}

// Request makes an authenticated request to the Kalshi API.
func (c *Client) Request(method, path string, query url.Values, body io.Reader) (*http.Response, error) {
	// Buffer the body so we can replay it on retries
	var bodyBytes []byte
	if body != nil {
		var err error
		bodyBytes, err = io.ReadAll(body)
		if err != nil {
			return nil, fmt.Errorf("failed to read request body: %w", err)
		}
	}

	fullPath := apiVersion + path
	reqURL := c.baseURL + fullPath
	if len(query) > 0 {
		reqURL += "?" + query.Encode()
	}

	backoff := baseBackoff

	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			time.Sleep(backoff)
			backoff = time.Duration(float64(backoff) * backoffFactor)
			if backoff > maxBackoff {
				backoff = maxBackoff
			}
		}

		// Wait for rate limiter
		c.rateLimiter.Wait()

		// Build body reader for this attempt
		var bodyReader io.Reader
		if bodyBytes != nil {
			bodyReader = bytes.NewReader(bodyBytes)
		}

		req, err := http.NewRequest(method, reqURL, bodyReader)
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}

		// Re-sign on each attempt (timestamp must be fresh)
		headers, err := c.signer.SignRequest(method, fullPath)
		if err != nil {
			return nil, fmt.Errorf("failed to sign request: %w", err)
		}
		for k, v := range headers {
			req.Header.Set(k, v)
		}
		if bodyBytes != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		req.Header.Set("Accept", "application/json")

		resp, err := c.httpClient.Do(req)
		if err != nil {
			if attempt < maxRetries {
				fmt.Printf("    Retry %d/%d: network error: %v\n", attempt+1, maxRetries, err)
				continue
			}
			return nil, err
		}

		if isRetryable(resp.StatusCode) && attempt < maxRetries {
			resp.Body.Close()
			fmt.Printf("    Retry %d/%d: got %d, backing off %v\n", attempt+1, maxRetries, resp.StatusCode, backoff)
			continue
		}

		return resp, nil
	}

	// Unreachable, but satisfies the compiler
	return nil, fmt.Errorf("exhausted retries for %s %s", method, path)
}

// Get makes an authenticated GET request.
func (c *Client) Get(path string, query url.Values) (*http.Response, error) {
	return c.Request(http.MethodGet, path, query, nil)
}

// Post makes an authenticated POST request with JSON body.
func (c *Client) Post(path string, body interface{}) (*http.Response, error) {
	var bodyReader io.Reader
	if body != nil {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
		bodyReader = strings.NewReader(string(jsonBody))
	}
	return c.Request(http.MethodPost, path, nil, bodyReader)
}

// ReadJSON reads a JSON response body into the target struct.
func ReadJSON(resp *http.Response, target interface{}) error {
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(body))
	}

	return json.NewDecoder(resp.Body).Decode(target)
}

// GetSeriesList retrieves all series, optionally filtered by category.
func (c *Client) GetSeriesList(category string) ([]models.Series, error) {
	query := url.Values{}
	if category != "" {
		query.Set("category", category)
	}

	resp, err := c.Get("/series", query)
	if err != nil {
		return nil, err
	}

	var result models.GetSeriesListResponse
	if err := ReadJSON(resp, &result); err != nil {
		return nil, err
	}

	return result.Series, nil
}

// GetMarkets retrieves markets with optional filters.
func (c *Client) GetMarkets(seriesTicker, status string) ([]models.Market, error) {
	var allMarkets []models.Market
	cursor := ""

	for {
		query := url.Values{}
		query.Set("limit", "1000")

		if seriesTicker != "" {
			query.Set("series_ticker", seriesTicker)
		}
		if status != "" {
			query.Set("status", status)
		}
		if cursor != "" {
			query.Set("cursor", cursor)
		}

		resp, err := c.Get("/markets", query)
		if err != nil {
			return nil, err
		}

		var result models.GetMarketsResponse
		if err := ReadJSON(resp, &result); err != nil {
			return nil, err
		}

		allMarkets = append(allMarkets, result.Markets...)

		if result.Cursor == "" {
			break
		}
		cursor = result.Cursor
	}

	return allMarkets, nil
}

// GetAllOpenMarkets retrieves all open markets across all series in a category.
func (c *Client) GetAllOpenMarkets(category string) ([]models.Market, error) {
	series, err := c.GetSeriesList(category)
	if err != nil {
		return nil, fmt.Errorf("failed to get series list: %w", err)
	}

	var allMarkets []models.Market

	for _, s := range series {
		markets, err := c.GetMarkets(s.Ticker, "open")
		if err != nil {
			fmt.Printf("Warning: failed to get markets for series %s: %v\n", s.Ticker, err)
			continue
		}
		allMarkets = append(allMarkets, markets...)
	}

	return allMarkets, nil
}

// GetBalance retrieves the member's available balance and portfolio value.
func (c *Client) GetBalance() (*models.GetBalanceResponse, error) {
	resp, err := c.Get("/portfolio/balance", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get balance: %w", err)
	}

	var result models.GetBalanceResponse
	if err := ReadJSON(resp, &result); err != nil {
		return nil, fmt.Errorf("failed to parse balance response: %w", err)
	}

	return &result, nil
}

// GetPositions retrieves all market positions with non-zero position counts.
func (c *Client) GetPositions() ([]models.MarketPosition, error) {
	var allPositions []models.MarketPosition
	cursor := ""

	for {
		query := url.Values{}
		query.Set("limit", "200")
		query.Set("count_filter", "position")
		if cursor != "" {
			query.Set("cursor", cursor)
		}

		resp, err := c.Get("/portfolio/positions", query)
		if err != nil {
			return nil, fmt.Errorf("failed to get positions: %w", err)
		}

		var result models.GetPositionsResponse
		if err := ReadJSON(resp, &result); err != nil {
			return nil, fmt.Errorf("failed to parse positions response: %w", err)
		}

		allPositions = append(allPositions, result.MarketPositions...)

		if result.Cursor == "" {
			break
		}
		cursor = result.Cursor
	}

	return allPositions, nil
}

// GetMarket retrieves a single market by its ticker.
func (c *Client) GetMarket(ticker string) (*models.Market, error) {
	resp, err := c.Get("/markets/"+ticker, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get market %s: %w", ticker, err)
	}

	var result models.GetMarketResponse
	if err := ReadJSON(resp, &result); err != nil {
		return nil, fmt.Errorf("failed to parse market response for %s: %w", ticker, err)
	}

	return &result.Market, nil
}

// GetSettlements retrieves portfolio settlements since minTs.
func (c *Client) GetSettlements(minTs time.Time) ([]models.Settlement, error) {
	var all []models.Settlement
	cursor := ""

	for {
		query := url.Values{}
		query.Set("limit", "200")
		query.Set("min_ts", strconv.FormatInt(minTs.Unix(), 10))
		if cursor != "" {
			query.Set("cursor", cursor)
		}

		resp, err := c.Get("/portfolio/settlements", query)
		if err != nil {
			return nil, fmt.Errorf("failed to get settlements: %w", err)
		}

		var result models.GetSettlementsResponse
		if err := ReadJSON(resp, &result); err != nil {
			return nil, fmt.Errorf("failed to parse settlements response: %w", err)
		}

		all = append(all, result.Settlements...)

		if result.Cursor == "" {
			break
		}
		cursor = result.Cursor
	}

	return all, nil
}

// KnownCategories is the fallback list of Kalshi market categories.
var KnownCategories = []string{
	"Climate and Weather", "Companies", "Crypto", "Economics", "Elections",
	"Entertainment", "Financials", "Health", "Mentions", "Politics",
	"Science and Technology", "Social", "Sports", "World",
}

// GetCategories fetches available market categories from the Kalshi API.
func (c *Client) GetCategories() ([]string, error) {
	resp, err := c.Get("/search/tags_by_categories", nil)
	if err != nil {
		return KnownCategories, nil
	}

	var result struct {
		TagsByCategories map[string]interface{} `json:"tags_by_categories"`
	}
	if err := ReadJSON(resp, &result); err != nil {
		return KnownCategories, nil
	}

	if len(result.TagsByCategories) == 0 {
		return KnownCategories, nil
	}

	categories := make([]string, 0, len(result.TagsByCategories))
	for cat := range result.TagsByCategories {
		categories = append(categories, cat)
	}

	sort.Strings(categories)
	return categories, nil
}

// GetFills retrieves portfolio fills since minTs.
func (c *Client) GetFills(minTs time.Time) ([]models.Fill, error) {
	var all []models.Fill
	cursor := ""

	for {
		query := url.Values{}
		query.Set("limit", "200")
		query.Set("min_ts", strconv.FormatInt(minTs.Unix(), 10))
		if cursor != "" {
			query.Set("cursor", cursor)
		}

		resp, err := c.Get("/portfolio/fills", query)
		if err != nil {
			return nil, fmt.Errorf("failed to get fills: %w", err)
		}

		var result models.GetFillsResponse
		if err := ReadJSON(resp, &result); err != nil {
			return nil, fmt.Errorf("failed to parse fills response: %w", err)
		}

		all = append(all, result.Fills...)

		if result.Cursor == "" {
			break
		}
		cursor = result.Cursor
	}

	return all, nil
}
