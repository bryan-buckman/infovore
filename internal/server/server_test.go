package server

import (
	"bytes"
	"encoding/json"
	"html/template"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/bryan-buckman/infovore/internal/database"
	"github.com/bryan-buckman/infovore/internal/model"
	"github.com/bryan-buckman/infovore/internal/rss"
)

// mockStore is a test double for database.Store. Fields on the struct configure
// return values; slices record the arguments passed to write methods so tests
// can assert on them.
type mockStore struct {
	// Folder data
	folders          []model.Folder
	foldersWithFeeds []model.FolderWithFeeds
	folderByID       *model.Folder
	folderByIDErr    error
	createFolderID   int64
	createFolderErr  error

	// Feed data
	allFeeds           []model.Feed
	unfiledFeeds       []model.Feed
	feedsByFolderID    []model.Feed
	feedByID           *model.Feed
	feedByIDErr        error
	getOrCreateFeedID  int64
	getOrCreateFeedNew bool
	getOrCreateFeedErr error

	// Item data
	allItems        []model.Item
	itemsByFeedID   []model.Item
	itemsByFolderID []model.Item
	cleanupDeleted  int64
	cleanupErr      error

	// Settings
	settings   map[string]string
	pollingInt int
	pollingErr error

	// Kalshi
	kalshiReportHTML        string
	kalshiReportGeneratedAt time.Time
	kalshiReportErr         error
	kalshiScanLog           []database.ScanLogEntry

	// Call recorders
	markedReadIDs   []int64
	deletedReadIDs  []int64
	deletedFeedID   int64
	deletedFolderID int64
	movedFeedID     int64
	movedFolderID   *int64
	setSettings     map[string]string // key -> last value set
}

func newMockStore() *mockStore {
	return &mockStore{
		settings:    make(map[string]string),
		setSettings: make(map[string]string),
	}
}

// --- database.Store implementation ---

func (m *mockStore) Close() error { return nil }

func (m *mockStore) DatabaseType() string { return "SQLite" }

func (m *mockStore) SupportsHighConcurrency() bool { return false }

// Folder operations

func (m *mockStore) GetFolders() ([]model.Folder, error) { return m.folders, nil }

func (m *mockStore) CreateFolder(name string, parentID *int64) (int64, error) {
	return m.createFolderID, m.createFolderErr
}

func (m *mockStore) GetOrCreateFolder(name string, parentID *int64) (int64, error) {
	return m.createFolderID, m.createFolderErr
}

func (m *mockStore) GetFolderByID(folderID int64) (*model.Folder, error) {
	return m.folderByID, m.folderByIDErr
}

func (m *mockStore) DeleteFolder(folderID int64) error {
	m.deletedFolderID = folderID
	return nil
}

// Feed operations

func (m *mockStore) GetFeeds(folderID *int64) ([]model.Feed, error) { return m.allFeeds, nil }

func (m *mockStore) GetAllFeeds() ([]model.Feed, error) { return m.allFeeds, nil }

func (m *mockStore) GetFeedsByFolderID(folderID int64) ([]model.Feed, error) {
	return m.feedsByFolderID, nil
}

func (m *mockStore) GetUnfiledFeeds() ([]model.Feed, error) { return m.unfiledFeeds, nil }

func (m *mockStore) GetFoldersWithFeeds() ([]model.FolderWithFeeds, error) {
	return m.foldersWithFeeds, nil
}

func (m *mockStore) CreateFeed(folderID *int64, title, url string) (int64, error) {
	return m.getOrCreateFeedID, m.getOrCreateFeedErr
}

func (m *mockStore) GetOrCreateFeed(folderID *int64, title, url string) (int64, bool, error) {
	return m.getOrCreateFeedID, m.getOrCreateFeedNew, m.getOrCreateFeedErr
}

func (m *mockStore) UpdateFeedLastFetched(feedID int64, t time.Time) error { return nil }

func (m *mockStore) UpdateFeedTitle(feedID int64, title string) error { return nil }

func (m *mockStore) UpdateFeedError(feedID int64, errMsg string) error { return nil }

func (m *mockStore) GetFeedByID(feedID int64) (*model.Feed, error) {
	return m.feedByID, m.feedByIDErr
}

func (m *mockStore) DeleteFeed(feedID int64) error {
	m.deletedFeedID = feedID
	return nil
}

func (m *mockStore) MoveFeedToFolder(feedID int64, folderID *int64) error {
	m.movedFeedID = feedID
	m.movedFolderID = folderID
	return nil
}
func (m *mockStore) AddFeedToFolder(feedID, folderID int64) error      { return nil }
func (m *mockStore) RemoveFeedFromFolder(feedID, folderID int64) error { return nil }
func (m *mockStore) GetFolderIDsForFeed(feedID int64) ([]int64, error) { return nil, nil }
func (m *mockStore) GetUnreadCounts() (map[int64]int, error)           { return nil, nil }

// Item operations

func (m *mockStore) AddItem(item *model.Item) (int64, bool, error) { return 0, false, nil }

func (m *mockStore) GetItems(feedID int64, onlyUnread bool) ([]model.Item, error) {
	return m.itemsByFeedID, nil
}

func (m *mockStore) GetAllItems(onlyUnread bool) ([]model.Item, error) { return m.allItems, nil }

func (m *mockStore) GetItemsByFolderID(folderID int64, onlyUnread bool) ([]model.Item, error) {
	return m.itemsByFolderID, nil
}

func (m *mockStore) MarkItemRead(itemID int64) error { return nil }

func (m *mockStore) MarkItemsRead(itemIDs []int64) error {
	m.markedReadIDs = itemIDs
	return nil
}

func (m *mockStore) DeleteReadItems(itemIDs []int64) error {
	m.deletedReadIDs = itemIDs
	return nil
}

func (m *mockStore) CleanupReadItems() (int64, error) {
	return m.cleanupDeleted, m.cleanupErr
}

// Settings operations

func (m *mockStore) GetSetting(key string) (string, error) {
	return m.settings[key], nil
}

func (m *mockStore) SetSetting(key, value string) error {
	m.setSettings[key] = value
	return nil
}

func (m *mockStore) GetPollingInterval() (int, error) {
	return m.pollingInt, m.pollingErr
}

// Kalshi operations

func (m *mockStore) SaveKalshiReport(html string) error { return nil }

func (m *mockStore) GetLatestKalshiReport() (string, time.Time, error) {
	return m.kalshiReportHTML, m.kalshiReportGeneratedAt, m.kalshiReportErr
}

func (m *mockStore) AddKalshiScanLog(output, errMsg string) error { return nil }

func (m *mockStore) GetKalshiScanLog(limit int) ([]database.ScanLogEntry, error) {
	return m.kalshiScanLog, nil
}

func (m *mockStore) SetKalshiOverride(ticker, side, field string, value float64) error {
	return nil
}

func (m *mockStore) GetKalshiOverrides() (map[string]float64, error) {
	return nil, nil
}

// --- Test server constructor ---

// newTestServer builds a Server without starting any background goroutines.
// It mirrors what New() does but constructs KalshiManager directly (no scheduler)
// and skips the rss.Poller.
func newTestServer(t *testing.T, db database.Store) *Server {
	t.Helper()
	tmpl, err := template.New("").Funcs(template.FuncMap{
		"timeAgo":  timeAgo,
		"safeHTML": func(s string) template.HTML { return template.HTML(s) },
	}).ParseFS(templatesFS, "templates/*.html")
	if err != nil {
		t.Fatalf("parse templates: %v", err)
	}
	s := &Server{
		db:        db,
		fetcher:   rss.NewFetcher(db),
		templates: tmpl,
		kalshi:    &KalshiManager{db: db, stopCh: make(chan struct{})},
	}
	s.setupRoutes()
	return s
}

// get is a convenience helper that issues a GET request and returns the recorder.
func get(t *testing.T, s *Server, path string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	rr := httptest.NewRecorder()
	s.router.ServeHTTP(rr, req)
	return rr
}

// post is a convenience helper that issues a POST request with a JSON body.
func post(t *testing.T, s *Server, path string, body interface{}) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			t.Fatalf("encode request body: %v", err)
		}
	}
	req := httptest.NewRequest(http.MethodPost, path, &buf)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	s.router.ServeHTTP(rr, req)
	return rr
}

// del issues a DELETE request.
func del(t *testing.T, s *Server, path string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodDelete, path, nil)
	rr := httptest.NewRecorder()
	s.router.ServeHTTP(rr, req)
	return rr
}

// decodeJSON decodes the response body into v.
func decodeJSON(t *testing.T, rr *httptest.ResponseRecorder, v interface{}) {
	t.Helper()
	if err := json.NewDecoder(rr.Body).Decode(v); err != nil {
		t.Fatalf("decode JSON response: %v", err)
	}
}

// --- Page handler tests ---

func TestShellPage(t *testing.T) {
	s := newTestServer(t, newMockStore())
	rr := get(t, s, "/")

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}
	body := rr.Body.String()
	if !strings.Contains(body, "Infovore") {
		t.Error("expected body to contain 'Infovore'")
	}
}

func TestSettingsPage(t *testing.T) {
	s := newTestServer(t, newMockStore())
	rr := get(t, s, "/settings")

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}
	body := rr.Body.String()
	if !strings.Contains(body, "Settings") {
		t.Error("expected body to contain 'Settings'")
	}
}

func TestReaderPage(t *testing.T) {
	db := newMockStore()
	db.allItems = []model.Item{
		{ID: 1, FeedID: 1, Title: "Test Article", Link: "https://example.com/1"},
	}
	s := newTestServer(t, db)
	rr := get(t, s, "/reader")

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}
	body := rr.Body.String()
	// layout.html always contains the logo text "Infovore" in the sidebar
	if !strings.Contains(body, "Infovore") {
		t.Error("expected reader page body to contain 'Infovore'")
	}
}

func TestFeedPage(t *testing.T) {
	db := newMockStore()
	feedTitle := "Example RSS Feed"
	db.feedByID = &model.Feed{ID: 1, Title: feedTitle, URL: "https://example.com/feed"}
	db.itemsByFeedID = []model.Item{
		{ID: 1, FeedID: 1, Title: "First Article"},
	}
	s := newTestServer(t, db)
	rr := get(t, s, "/reader/feed/1")

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}
	body := rr.Body.String()
	if !strings.Contains(body, feedTitle) {
		t.Errorf("expected body to contain feed title %q", feedTitle)
	}
}

func TestFolderPage(t *testing.T) {
	db := newMockStore()
	folderName := "Tech News"
	db.folderByID = &model.Folder{ID: 1, Name: folderName}
	db.itemsByFolderID = []model.Item{
		{ID: 2, FeedID: 1, Title: "Folder Article"},
	}
	s := newTestServer(t, db)
	rr := get(t, s, "/reader/folder/1")

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}
	body := rr.Body.String()
	if !strings.Contains(body, folderName) {
		t.Errorf("expected body to contain folder name %q", folderName)
	}
}

// --- API settings tests ---

func TestGetSettings(t *testing.T) {
	db := newMockStore()
	db.pollingInt = 30
	db.settings["kalshi_api_key_id"] = "my-key-id"
	db.settings["kalshi_private_key"] = "some-secret"
	db.settings["kalshi_categories"] = "Politics,Sports"
	db.settings["kalshi_scan_interval_hours"] = "12"
	db.settings["theme"] = "dark"

	s := newTestServer(t, db)
	rr := get(t, s, "/api/settings")

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}

	var resp map[string]interface{}
	decodeJSON(t, rr, &resp)

	if got := resp["polling_interval"].(float64); got != 30 {
		t.Errorf("polling_interval: got %v, want 30", got)
	}
	if got := resp["kalshi_api_key_id"].(string); got != "my-key-id" {
		t.Errorf("kalshi_api_key_id: got %q, want %q", got, "my-key-id")
	}
	// Private key should never be echoed back; only a boolean indicator.
	if configured, ok := resp["kalshi_private_key_configured"].(bool); !ok || !configured {
		t.Error("expected kalshi_private_key_configured to be true")
	}
	if got := resp["kalshi_categories"].(string); got != "Politics,Sports" {
		t.Errorf("kalshi_categories: got %q, want %q", got, "Politics,Sports")
	}
	if got := resp["kalshi_scan_interval_hours"].(float64); got != 12 {
		t.Errorf("kalshi_scan_interval_hours: got %v, want 12", got)
	}
	if got := resp["theme"].(string); got != "dark" {
		t.Errorf("theme: got %q, want %q", got, "dark")
	}
	if ct := rr.Header().Get("Content-Type"); !strings.Contains(ct, "application/json") {
		t.Errorf("expected JSON content type, got %q", ct)
	}
}

func TestGetSettings_Defaults(t *testing.T) {
	// With nothing configured, scan interval defaults to 6.
	db := newMockStore()
	s := newTestServer(t, db)
	rr := get(t, s, "/api/settings")

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}
	var resp map[string]interface{}
	decodeJSON(t, rr, &resp)

	if got := resp["kalshi_scan_interval_hours"].(float64); got != 6 {
		t.Errorf("kalshi_scan_interval_hours default: got %v, want 6", got)
	}
	if configured, _ := resp["kalshi_private_key_configured"].(bool); configured {
		t.Error("expected kalshi_private_key_configured to be false when no key set")
	}
}

func TestSaveSettings(t *testing.T) {
	db := newMockStore()
	s := newTestServer(t, db)

	body := map[string]interface{}{
		"polling_interval":           30,
		"kalshi_api_key_id":          "abc-key",
		"kalshi_private_key":         "secret-pem",
		"kalshi_categories":          "Finance",
		"kalshi_scan_interval_hours": 8,
		"theme":                      "dark",
	}
	rr := post(t, s, "/api/settings", body)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}

	// Verify each setting was persisted.
	checks := map[string]string{
		model.SettingPollingInterval: "30",
		"kalshi_api_key_id":          "abc-key",
		"kalshi_private_key":         "secret-pem",
		"kalshi_categories":          "Finance",
		"kalshi_scan_interval_hours": "8",
		"theme":                      "dark",
	}
	for key, want := range checks {
		if got, ok := db.setSettings[key]; !ok {
			t.Errorf("SetSetting(%q) was never called", key)
		} else if got != want {
			t.Errorf("SetSetting(%q): got %q, want %q", key, got, want)
		}
	}
}

func TestSaveSettings_PollingIntervalMinimumEnforced(t *testing.T) {
	// Any interval below MinPollingIntervalMinutes should be clamped to 15.
	db := newMockStore()
	s := newTestServer(t, db)

	rr := post(t, s, "/api/settings", map[string]interface{}{
		"polling_interval": 5, // below the 15-minute minimum
	})

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}

	got := db.setSettings[model.SettingPollingInterval]
	want := "15"
	if got != want {
		t.Errorf("polling interval below minimum: got %q, want %q", got, want)
	}
}

func TestSaveSettings_KalshiScanIntervalMinimumEnforced(t *testing.T) {
	// Scan interval below 1 should be clamped to 1.
	db := newMockStore()
	s := newTestServer(t, db)

	rr := post(t, s, "/api/settings", map[string]interface{}{
		"kalshi_scan_interval_hours": 0,
	})

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}

	got := db.setSettings["kalshi_scan_interval_hours"]
	want := "1"
	if got != want {
		t.Errorf("scan interval below minimum: got %q, want %q", got, want)
	}
}

func TestSaveSettings_EmptyPrivateKeyNotOverwritten(t *testing.T) {
	// An empty string for kalshi_private_key must not overwrite an existing key.
	db := newMockStore()
	s := newTestServer(t, db)

	rr := post(t, s, "/api/settings", map[string]interface{}{
		"kalshi_private_key": "",
	})

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}

	if _, called := db.setSettings["kalshi_private_key"]; called {
		t.Error("SetSetting(kalshi_private_key) should not be called when value is empty")
	}
}

// --- Mark read ---

func TestMarkRead(t *testing.T) {
	db := newMockStore()
	s := newTestServer(t, db)

	rr := post(t, s, "/api/mark-read", map[string]interface{}{
		"item_ids": []int64{1, 2, 3},
	})

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}
	if len(db.markedReadIDs) != 3 {
		t.Errorf("expected 3 items marked read, got %d", len(db.markedReadIDs))
	}
}

func TestMarkRead_InvalidBody(t *testing.T) {
	s := newTestServer(t, newMockStore())
	req := httptest.NewRequest(http.MethodPost, "/api/mark-read", strings.NewReader("not-json"))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	s.router.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rr.Code)
	}
}

// --- Cleanup ---

func TestCleanup(t *testing.T) {
	db := newMockStore()
	db.cleanupDeleted = 42
	s := newTestServer(t, db)

	rr := post(t, s, "/api/cleanup", nil)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}

	var resp map[string]interface{}
	decodeJSON(t, rr, &resp)

	if got := resp["deleted"].(float64); got != 42 {
		t.Errorf("deleted: got %v, want 42", got)
	}
	if got := resp["status"].(string); got != "ok" {
		t.Errorf("status: got %q, want %q", got, "ok")
	}
}

// --- Delete feed ---

func TestDeleteFeed(t *testing.T) {
	db := newMockStore()
	s := newTestServer(t, db)

	rr := del(t, s, "/api/feed/7")

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}
	if db.deletedFeedID != 7 {
		t.Errorf("expected feed ID 7 to be deleted, got %d", db.deletedFeedID)
	}

	var resp map[string]interface{}
	decodeJSON(t, rr, &resp)
	if resp["status"] != "ok" {
		t.Errorf("expected status ok, got %v", resp["status"])
	}
}

func TestDeleteFeed_InvalidID(t *testing.T) {
	s := newTestServer(t, newMockStore())
	rr := del(t, s, "/api/feed/not-a-number")

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rr.Code)
	}
}

// --- Delete folder ---

func TestDeleteFolder(t *testing.T) {
	db := newMockStore()
	s := newTestServer(t, db)

	rr := del(t, s, "/api/folder/3")

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}
	if db.deletedFolderID != 3 {
		t.Errorf("expected folder ID 3 to be deleted, got %d", db.deletedFolderID)
	}
}

func TestDeleteFolder_InvalidID(t *testing.T) {
	s := newTestServer(t, newMockStore())
	rr := del(t, s, "/api/folder/bad")

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rr.Code)
	}
}

// --- Add feed ---

func TestAddFeed(t *testing.T) {
	db := newMockStore()
	db.getOrCreateFeedID = 99
	db.getOrCreateFeedNew = true
	s := newTestServer(t, db)

	rr := post(t, s, "/api/feed", map[string]interface{}{
		"url": "https://example.com/rss",
	})

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}

	var resp map[string]interface{}
	decodeJSON(t, rr, &resp)
	if got := resp["feed_id"].(float64); got != 99 {
		t.Errorf("feed_id: got %v, want 99", got)
	}
	if isNew, _ := resp["is_new"].(bool); !isNew {
		t.Error("expected is_new to be true")
	}
}

func TestAddFeed_MissingURL(t *testing.T) {
	s := newTestServer(t, newMockStore())

	rr := post(t, s, "/api/feed", map[string]interface{}{
		"url": "",
	})

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status 400 for missing URL, got %d", rr.Code)
	}
}

func TestAddFeed_InvalidBody(t *testing.T) {
	s := newTestServer(t, newMockStore())
	req := httptest.NewRequest(http.MethodPost, "/api/feed", strings.NewReader("not-json"))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	s.router.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status 400 for invalid body, got %d", rr.Code)
	}
}

// --- Add folder ---

func TestAddFolder(t *testing.T) {
	db := newMockStore()
	db.createFolderID = 55
	s := newTestServer(t, db)

	rr := post(t, s, "/api/folder", map[string]interface{}{
		"name": "Science",
	})

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}

	var resp map[string]interface{}
	decodeJSON(t, rr, &resp)
	if got := resp["folder_id"].(float64); got != 55 {
		t.Errorf("folder_id: got %v, want 55", got)
	}
}

func TestAddFolder_MissingName(t *testing.T) {
	s := newTestServer(t, newMockStore())

	rr := post(t, s, "/api/folder", map[string]interface{}{
		"name": "",
	})

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status 400 for missing name, got %d", rr.Code)
	}
}

func TestAddFolder_InvalidBody(t *testing.T) {
	s := newTestServer(t, newMockStore())
	req := httptest.NewRequest(http.MethodPost, "/api/folder", strings.NewReader("not-json"))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	s.router.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status 400 for invalid body, got %d", rr.Code)
	}
}

// --- Move feed ---

func TestMoveFeed(t *testing.T) {
	db := newMockStore()
	s := newTestServer(t, db)

	folderID := int64(10)
	rr := post(t, s, "/api/feed/5/move", map[string]interface{}{
		"folder_id": folderID,
	})

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}
	if db.movedFeedID != 5 {
		t.Errorf("movedFeedID: got %d, want 5", db.movedFeedID)
	}
	if db.movedFolderID == nil || *db.movedFolderID != 10 {
		t.Errorf("movedFolderID: got %v, want pointer to 10", db.movedFolderID)
	}

	var resp map[string]interface{}
	decodeJSON(t, rr, &resp)
	if resp["status"] != "ok" {
		t.Errorf("expected status ok, got %v", resp["status"])
	}
}

func TestMoveFeed_InvalidFeedID(t *testing.T) {
	s := newTestServer(t, newMockStore())
	rr := post(t, s, "/api/feed/bad/move", map[string]interface{}{"folder_id": 1})

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status 400 for invalid feed ID, got %d", rr.Code)
	}
}

func TestMoveFeed_InvalidBody(t *testing.T) {
	s := newTestServer(t, newMockStore())
	req := httptest.NewRequest(http.MethodPost, "/api/feed/1/move", strings.NewReader("not-json"))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	s.router.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status 400 for invalid body, got %d", rr.Code)
	}
}

// --- Test Kalshi credentials ---

func TestTestKalshi_NoCreds(t *testing.T) {
	// Neither key ID nor private key are set: expect 400.
	db := newMockStore()
	s := newTestServer(t, db)

	rr := post(t, s, "/api/settings/test-kalshi", nil)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status 400 when no credentials configured, got %d", rr.Code)
	}

	var resp map[string]interface{}
	decodeJSON(t, rr, &resp)
	if _, ok := resp["error"]; !ok {
		t.Error("expected 'error' field in response body")
	}
}

func TestTestKalshi_MissingPrivateKey(t *testing.T) {
	// Key ID set but no private key: still expect 400.
	db := newMockStore()
	db.settings["kalshi_api_key_id"] = "some-id"
	s := newTestServer(t, db)

	rr := post(t, s, "/api/settings/test-kalshi", nil)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status 400 when private key missing, got %d", rr.Code)
	}
}

// --- Table-driven tests ---

func TestShellPage_ContentType(t *testing.T) {
	s := newTestServer(t, newMockStore())
	rr := get(t, s, "/")

	ct := rr.Header().Get("Content-Type")
	if !strings.Contains(ct, "text/html") {
		t.Errorf("expected text/html content type, got %q", ct)
	}
}

func TestSettingsPage_ContentType(t *testing.T) {
	s := newTestServer(t, newMockStore())
	rr := get(t, s, "/settings")

	ct := rr.Header().Get("Content-Type")
	if !strings.Contains(ct, "text/html") {
		t.Errorf("expected text/html content type, got %q", ct)
	}
}

func TestReaderPage_ContentType(t *testing.T) {
	s := newTestServer(t, newMockStore())
	rr := get(t, s, "/reader")

	ct := rr.Header().Get("Content-Type")
	if !strings.Contains(ct, "text/html") {
		t.Errorf("expected text/html content type, got %q", ct)
	}
}

// TestAPIRoutes_StatusCodes performs a quick smoke-test of all routes that do
// not require complex setup, verifying they return non-5xx status codes.
func TestAPIRoutes_StatusCodes(t *testing.T) {
	tests := []struct {
		method string
		path   string
		body   interface{}
		want   int
	}{
		{http.MethodGet, "/api/settings", nil, http.StatusOK},
		{http.MethodPost, "/api/cleanup", nil, http.StatusOK},
		{http.MethodDelete, "/api/feed/1", nil, http.StatusOK},
		{http.MethodDelete, "/api/folder/1", nil, http.StatusOK},
		{
			http.MethodPost, "/api/feed", map[string]interface{}{"url": "https://test.com/feed"},
			http.StatusOK,
		},
		{
			http.MethodPost, "/api/folder", map[string]interface{}{"name": "MyFolder"},
			http.StatusOK,
		},
		{
			http.MethodPost, "/api/feed/1/move", map[string]interface{}{"folder_id": nil},
			http.StatusOK,
		},
		{http.MethodPost, "/api/settings/test-kalshi", nil, http.StatusBadRequest},
		{http.MethodGet, "/api/rss/status", nil, http.StatusOK},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.method+"_"+tc.path, func(t *testing.T) {
			db := newMockStore()
			db.getOrCreateFeedID = 1
			db.createFolderID = 1
			s := newTestServer(t, db)

			var rr *httptest.ResponseRecorder
			switch tc.method {
			case http.MethodGet:
				rr = get(t, s, tc.path)
			case http.MethodPost:
				rr = post(t, s, tc.path, tc.body)
			case http.MethodDelete:
				rr = del(t, s, tc.path)
			default:
				t.Fatalf("unsupported method %q in test table", tc.method)
			}

			if rr.Code != tc.want {
				t.Errorf("%s %s: got status %d, want %d (body: %s)",
					tc.method, tc.path, rr.Code, tc.want, rr.Body.String())
			}
		})
	}
}

// TestRSSRefreshAndStatus tests the async RSS refresh + status polling flow.
func TestRSSRefreshAndStatus(t *testing.T) {
	db := newMockStore()
	db.allFeeds = []model.Feed{
		{ID: 1, Title: "Feed1", URL: "https://example.com/feed1.xml"},
	}
	s := newTestServer(t, db)

	// Status should initially show not refreshing
	rr := get(t, s, "/api/rss/status")
	if rr.Code != http.StatusOK {
		t.Fatalf("GET /api/rss/status: got %d, want %d", rr.Code, http.StatusOK)
	}
	var status struct {
		Refreshing bool `json:"refreshing"`
		FeedsDone  int  `json:"feeds_done"`
		FeedsTotal int  `json:"feeds_total"`
	}
	json.Unmarshal(rr.Body.Bytes(), &status)
	if status.Refreshing {
		t.Error("expected refreshing=false initially")
	}

	// Start async refresh
	rr = post(t, s, "/api/rss/refresh", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("POST /api/rss/refresh: got %d, want %d", rr.Code, http.StatusOK)
	}
	var resp map[string]string
	json.Unmarshal(rr.Body.Bytes(), &resp)
	if resp["status"] != "started" {
		t.Errorf("expected status=started, got %q", resp["status"])
	}

	// Wait for the async refresh to complete (feeds are mocked, should be fast)
	time.Sleep(500 * time.Millisecond)

	// Status should now show not refreshing with feed counts populated
	rr = get(t, s, "/api/rss/status")
	json.Unmarshal(rr.Body.Bytes(), &status)
	if status.Refreshing {
		t.Error("expected refreshing=false after completion")
	}
	if status.FeedsTotal != 1 {
		t.Errorf("expected feeds_total=1, got %d", status.FeedsTotal)
	}
}
