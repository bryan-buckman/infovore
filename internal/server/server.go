// Package server provides the HTTP server and handlers.
package server

import (
	"bufio"
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"html/template"
	"io/fs"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/bryan-buckman/infovore/internal/database"
	kalshiconfig "github.com/bryan-buckman/infovore/internal/kalshi/config"
	"github.com/bryan-buckman/infovore/internal/model"
	"github.com/bryan-buckman/infovore/internal/opml"
	"github.com/bryan-buckman/infovore/internal/rss"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

//go:embed templates/*.html
var templatesFS embed.FS

//go:embed static/*
var staticFS embed.FS

// Server is the main HTTP server.
type Server struct {
	db         database.Store
	fetcher    *rss.Fetcher
	poller     *rss.Poller
	router     chi.Router
	httpServer *http.Server
	templates  *template.Template
	kalshi     *KalshiManager
}

// New creates a new server.
func New(db database.Store) (*Server, error) {
	tmpl, err := template.New("").Funcs(template.FuncMap{
		"timeAgo":  timeAgo,
		"safeHTML": func(s string) template.HTML { return template.HTML(s) },
	}).ParseFS(templatesFS, "templates/*.html")
	if err != nil {
		return nil, fmt.Errorf("parse templates: %w", err)
	}

	s := &Server{
		db:        db,
		fetcher:   rss.NewFetcher(db),
		poller:    rss.NewPoller(db),
		templates: tmpl,
		kalshi:    NewKalshiManager(db),
	}
	s.setupRoutes()
	return s, nil
}

func (s *Server) setupRoutes() {
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Compress(5))

	// Serve static files.
	staticSub, _ := fs.Sub(staticFS, "static")
	r.Handle("/static/*", http.StripPrefix("/static/", http.FileServer(http.FS(staticSub))))

	// Shell page (tabbed container).
	r.Get("/", s.handleShell)

	// Reader pages (inside iframe).
	r.Get("/reader", s.handleHome)
	r.Get("/reader/feed/{feedID}", s.handleFeed)
	r.Get("/reader/folder/{folderID}", s.handleFolder)

	// Settings page.
	r.Get("/settings", s.handleSettings)

	// API.
	r.Route("/api", func(r chi.Router) {
		r.Post("/mark-read", s.handleMarkRead)
		r.Post("/delete-read", s.handleDeleteRead)
		r.Post("/settings", s.handleSaveSettings)
		r.Get("/settings", s.handleGetSettings)
		r.Post("/settings/test-kalshi", s.handleTestKalshi)
		r.Post("/import-opml", s.handleImportOPML)
		r.Get("/export-opml", s.handleExportOPML)
		r.Post("/refresh", s.handleRefresh)
		r.Post("/refresh-feed/{feedID}", s.handleRefreshFeed)
		r.Post("/refresh-folder/{folderID}", s.handleRefreshFolder)
		r.Post("/cleanup", s.handleCleanup)
		r.Get("/sidebar", s.handleSidebar)
		r.Delete("/feed/{feedID}", s.handleDeleteFeed)
		r.Delete("/folder/{folderID}", s.handleDeleteFolder)
		r.Post("/feed/{feedID}/move", s.handleMoveFeed)
		r.Post("/feed", s.handleAddFeed)
		r.Post("/folder", s.handleAddFolder)
		r.Get("/database-settings", s.handleGetDatabaseSettings)
		r.Post("/database-settings", s.handleSaveDatabaseSettings)

		// Kalshi API
		r.Get("/kalshi/status", s.kalshi.HandleKalshiStatus)
		r.Post("/kalshi/refresh", s.kalshi.HandleKalshiRefresh)
		r.Get("/kalshi/log", s.kalshi.HandleKalshiLog)
	})

	// Kalshi markets page (inside iframe).
	r.Get("/markets", s.kalshi.HandleMarketsPage)

	s.router = r
}

// Start starts the server (poller is NOT started automatically - use manual refresh).
func (s *Server) Start(addr string) error {
	s.httpServer = &http.Server{
		Addr:    addr,
		Handler: s.router,
	}
	// Note: Poller is NOT started automatically to avoid 403 errors from aggressive polling.
	// Users should use the manual Refresh button instead.
	log.Printf("Server starting on %s", addr)
	return s.httpServer.ListenAndServe()
}

// Stop gracefully shuts down the server and poller.
func (s *Server) Stop() {
	log.Println("Stopping Kalshi scheduler...")
	if s.kalshi != nil {
		s.kalshi.Stop()
	}

	log.Println("Stopping poller...")
	s.poller.Stop()

	if s.httpServer != nil {
		log.Println("Shutting down HTTP server...")
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := s.httpServer.Shutdown(ctx); err != nil {
			log.Printf("HTTP server shutdown error: %v", err)
		}
	}
	log.Println("Shutdown complete")
}

// --- Page Handlers ---

func (s *Server) handleShell(w http.ResponseWriter, r *http.Request) {
	s.render(w, "shell.html", nil)
}

func (s *Server) handleSettings(w http.ResponseWriter, r *http.Request) {
	s.render(w, "settings.html", nil)
}

func (s *Server) handleHome(w http.ResponseWriter, r *http.Request) {
	foldersWithFeeds, _ := s.db.GetFoldersWithFeeds()
	unfiledFeeds, _ := s.db.GetUnfiledFeeds()
	items, _ := s.db.GetAllItems(false)

	data := map[string]interface{}{
		"FoldersWithFeeds": foldersWithFeeds,
		"UnfiledFeeds":     unfiledFeeds,
		"Items":            items,
		"PageTitle":        "All Items",
	}
	s.render(w, "layout.html", data)
}

func (s *Server) handleFeed(w http.ResponseWriter, r *http.Request) {
	feedIDStr := chi.URLParam(r, "feedID")
	feedID, _ := strconv.ParseInt(feedIDStr, 10, 64)

	foldersWithFeeds, _ := s.db.GetFoldersWithFeeds()
	unfiledFeeds, _ := s.db.GetUnfiledFeeds()
	items, _ := s.db.GetItems(feedID, false)

	// Get feed name and error for title.
	pageTitle := "Feed"
	feedError := ""
	if feed, err := s.db.GetFeedByID(feedID); err == nil {
		pageTitle = feed.Title
		feedError = feed.LastError
	}

	data := map[string]interface{}{
		"FoldersWithFeeds": foldersWithFeeds,
		"UnfiledFeeds":     unfiledFeeds,
		"Items":            items,
		"CurrentFeedID":    feedID,
		"PageTitle":        pageTitle,
		"FeedError":        feedError,
	}
	s.render(w, "layout.html", data)
}

func (s *Server) handleFolder(w http.ResponseWriter, r *http.Request) {
	folderIDStr := chi.URLParam(r, "folderID")
	folderID, _ := strconv.ParseInt(folderIDStr, 10, 64)

	foldersWithFeeds, _ := s.db.GetFoldersWithFeeds()
	unfiledFeeds, _ := s.db.GetUnfiledFeeds()
	items, _ := s.db.GetItemsByFolderID(folderID, false)

	// Get folder name for title.
	pageTitle := "Folder"
	if folder, err := s.db.GetFolderByID(folderID); err == nil {
		pageTitle = folder.Name
	}

	data := map[string]interface{}{
		"FoldersWithFeeds": foldersWithFeeds,
		"UnfiledFeeds":     unfiledFeeds,
		"Items":            items,
		"CurrentFolderID":  folderID,
		"PageTitle":        pageTitle,
	}
	s.render(w, "layout.html", data)
}

// --- API Handlers ---

func (s *Server) handleMarkRead(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ItemIDs []int64 `json:"item_ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}
	if err := s.db.MarkItemsRead(req.ItemIDs); err != nil {
		http.Error(w, "Failed to mark read", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (s *Server) handleSaveSettings(w http.ResponseWriter, r *http.Request) {
	var req map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	// Polling interval
	if v, ok := req["polling_interval"]; ok {
		if fv, ok := v.(float64); ok {
			interval := int(fv)
			if interval < rss.MinPollingIntervalMinutes {
				interval = rss.MinPollingIntervalMinutes
			}
			s.db.SetSetting(model.SettingPollingInterval, strconv.Itoa(interval))
		}
	}

	// Kalshi API Key ID
	if v, ok := req["kalshi_api_key_id"]; ok {
		if sv, ok := v.(string); ok {
			s.db.SetSetting("kalshi_api_key_id", sv)
		}
	}

	// Kalshi Private Key (write-only)
	if v, ok := req["kalshi_private_key"]; ok {
		if sv, ok := v.(string); ok && sv != "" {
			s.db.SetSetting("kalshi_private_key", sv)
		}
	}

	// Kalshi Categories
	if v, ok := req["kalshi_categories"]; ok {
		if sv, ok := v.(string); ok {
			s.db.SetSetting("kalshi_categories", sv)
		}
	}

	// Kalshi Scan Interval
	if v, ok := req["kalshi_scan_interval_hours"]; ok {
		if fv, ok := v.(float64); ok {
			hours := int(fv)
			if hours < 1 {
				hours = 1
			}
			s.db.SetSetting("kalshi_scan_interval_hours", strconv.Itoa(hours))
		}
	}

	// Theme
	if v, ok := req["theme"]; ok {
		if sv, ok := v.(string); ok {
			s.db.SetSetting("theme", sv)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (s *Server) handleGetSettings(w http.ResponseWriter, r *http.Request) {
	interval, _ := s.db.GetPollingInterval()

	// Kalshi settings
	kalshiKeyID, _ := s.db.GetSetting("kalshi_api_key_id")
	kalshiKey, _ := s.db.GetSetting("kalshi_private_key")
	kalshiCategories, _ := s.db.GetSetting("kalshi_categories")
	kalshiScanInterval, _ := s.db.GetSetting("kalshi_scan_interval_hours")
	theme, _ := s.db.GetSetting("theme")

	scanHours, err := strconv.Atoi(kalshiScanInterval)
	if err != nil || scanHours < 1 {
		scanHours = 6
	}

	// Database info
	envFilePath := os.Getenv("INFOVORE_ENV_FILE")
	if envFilePath == "" {
		envFilePath = ".env"
	}
	dbURL := ""
	if file, err := os.Open(envFilePath); err == nil {
		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if strings.HasPrefix(line, "DB_URL=") {
				dbURL = strings.TrimPrefix(line, "DB_URL=")
				if len(dbURL) >= 2 && ((dbURL[0] == '"' && dbURL[len(dbURL)-1] == '"') ||
					(dbURL[0] == '\'' && dbURL[len(dbURL)-1] == '\'')) {
					dbURL = dbURL[1 : len(dbURL)-1]
				}
				break
			}
		}
		file.Close()
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"polling_interval":              interval,
		"kalshi_api_key_id":             kalshiKeyID,
		"kalshi_private_key_configured": kalshiKey != "",
		"kalshi_categories":             kalshiCategories,
		"kalshi_scan_interval_hours":    scanHours,
		"theme":                         theme,
		"db_url":                        dbURL,
		"database_type":                 s.db.DatabaseType(),
	})
}

func (s *Server) handleTestKalshi(w http.ResponseWriter, r *http.Request) {
	keyID, _ := s.db.GetSetting("kalshi_api_key_id")
	key, _ := s.db.GetSetting("kalshi_private_key")

	if keyID == "" || key == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Kalshi API key and private key must be configured first"})
		return
	}

	// Try to load config from settings — this validates the key format
	_, err := kalshiconfig.LoadFromSettings(s.db)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (s *Server) handleImportOPML(w http.ResponseWriter, r *http.Request) {
	file, _, err := r.FormFile("opml")
	if err != nil {
		http.Error(w, "No file provided", http.StatusBadRequest)
		return
	}
	defer file.Close()

	entries, err := opml.Parse(file)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to parse OPML: %v", err), http.StatusBadRequest)
		return
	}

	imported := 0
	for _, entry := range entries {
		// Create folder hierarchy.
		var folderID *int64
		for _, folderName := range entry.FolderPath {
			id, err := s.db.GetOrCreateFolder(folderName, folderID)
			if err != nil {
				log.Printf("Error creating folder %s: %v", folderName, err)
				continue
			}
			folderID = &id
		}

		// Create feed.
		_, isNew, err := s.db.GetOrCreateFeed(folderID, entry.Title, entry.URL)
		if err != nil {
			log.Printf("Error creating feed %s: %v", entry.URL, err)
			continue
		}
		if isNew {
			imported++
		}
	}

	// Note: We no longer auto-fetch after import to avoid 403 errors.
	// Users should click the Refresh button manually.

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":   "ok",
		"imported": imported,
		"total":    len(entries),
	})
}

func (s *Server) handleExportOPML(w http.ResponseWriter, r *http.Request) {
	feeds, err := s.db.GetAllFeeds()
	if err != nil {
		http.Error(w, "Failed to get feeds", http.StatusInternalServerError)
		return
	}

	folders, _ := s.db.GetFolders()
	folderMap := make(map[int64]string)
	for _, f := range folders {
		folderMap[f.ID] = f.Name
	}

	// Group feeds.
	grouped := make(map[string][]opml.FeedEntry)
	for _, feed := range feeds {
		entry := opml.FeedEntry{
			Title: feed.Title,
			URL:   feed.URL,
		}
		if feed.FolderID != nil {
			if name, ok := folderMap[*feed.FolderID]; ok {
				entry.FolderPath = []string{name}
			}
		}
		key := strings.Join(entry.FolderPath, "/")
		grouped[key] = append(grouped[key], entry)
	}

	data, err := opml.Export("Infovore Feeds", grouped)
	if err != nil {
		http.Error(w, "Failed to export", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/xml")
	w.Header().Set("Content-Disposition", "attachment; filename=infovore-feeds.opml")
	w.Write(data)
}

func (s *Server) handleRefresh(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Minute)
	defer cancel()

	results, err := s.fetcher.FetchAll(ctx)
	if err != nil {
		http.Error(w, fmt.Sprintf("Fetch error: %v", err), http.StatusInternalServerError)
		return
	}

	total := 0
	for _, c := range results {
		total += c
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":    "ok",
		"new_items": total,
		"feeds":     len(results),
	})
}

func (s *Server) handleCleanup(w http.ResponseWriter, r *http.Request) {
	deleted, err := s.db.CleanupReadItems()
	if err != nil {
		http.Error(w, "Cleanup failed", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "ok",
		"deleted": deleted,
	})
}

func (s *Server) handleSidebar(w http.ResponseWriter, r *http.Request) {
	folders, _ := s.db.GetFolders()
	feeds, _ := s.db.GetAllFeeds()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"folders": folders,
		"feeds":   feeds,
	})
}

func (s *Server) handleDeleteFeed(w http.ResponseWriter, r *http.Request) {
	feedIDStr := chi.URLParam(r, "feedID")
	feedID, err := strconv.ParseInt(feedIDStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid feed ID", http.StatusBadRequest)
		return
	}

	if err := s.db.DeleteFeed(feedID); err != nil {
		http.Error(w, "Failed to delete feed", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "ok",
	})
}

func (s *Server) handleDeleteFolder(w http.ResponseWriter, r *http.Request) {
	folderIDStr := chi.URLParam(r, "folderID")
	folderID, err := strconv.ParseInt(folderIDStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid folder ID", http.StatusBadRequest)
		return
	}

	if err := s.db.DeleteFolder(folderID); err != nil {
		http.Error(w, "Failed to delete folder", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "ok",
	})
}

func (s *Server) handleMoveFeed(w http.ResponseWriter, r *http.Request) {
	feedIDStr := chi.URLParam(r, "feedID")
	feedID, err := strconv.ParseInt(feedIDStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid feed ID", http.StatusBadRequest)
		return
	}

	var req struct {
		FolderID *int64 `json:"folder_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	if err := s.db.MoveFeedToFolder(feedID, req.FolderID); err != nil {
		http.Error(w, "Failed to move feed", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "ok",
	})
}

func (s *Server) handleRefreshFeed(w http.ResponseWriter, r *http.Request) {
	feedIDStr := chi.URLParam(r, "feedID")
	feedID, err := strconv.ParseInt(feedIDStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid feed ID", http.StatusBadRequest)
		return
	}

	feed, err := s.db.GetFeedByID(feedID)
	if err != nil {
		http.Error(w, "Feed not found", http.StatusNotFound)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Minute)
	defer cancel()

	count, err := s.fetcher.FetchFeed(ctx, *feed)
	if err != nil {
		http.Error(w, fmt.Sprintf("Fetch error: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":    "ok",
		"new_items": count,
	})
}

func (s *Server) handleRefreshFolder(w http.ResponseWriter, r *http.Request) {
	folderIDStr := chi.URLParam(r, "folderID")
	folderID, err := strconv.ParseInt(folderIDStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid folder ID", http.StatusBadRequest)
		return
	}

	feeds, err := s.db.GetFeedsByFolderID(folderID)
	if err != nil {
		http.Error(w, "Failed to get feeds", http.StatusInternalServerError)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Minute)
	defer cancel()

	total := 0
	for _, feed := range feeds {
		select {
		case <-ctx.Done():
			break
		default:
		}
		count, err := s.fetcher.FetchFeed(ctx, feed)
		if err != nil {
			log.Printf("Failed to fetch %s: %v", feed.URL, err)
			continue
		}
		total += count
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":    "ok",
		"new_items": total,
		"feeds":     len(feeds),
	})
}

func (s *Server) handleDeleteRead(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ItemIDs []int64 `json:"item_ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}
	if err := s.db.DeleteReadItems(req.ItemIDs); err != nil {
		http.Error(w, "Failed to delete items", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "ok",
		"deleted": len(req.ItemIDs),
	})
}

func (s *Server) handleAddFeed(w http.ResponseWriter, r *http.Request) {
	var req struct {
		URL      string `json:"url"`
		FolderID *int64 `json:"folder_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}
	if req.URL == "" {
		http.Error(w, "URL is required", http.StatusBadRequest)
		return
	}

	// Use URL as default title until we fetch the feed
	feedID, isNew, err := s.db.GetOrCreateFeed(req.FolderID, req.URL, req.URL)
	if err != nil {
		http.Error(w, "Failed to add feed", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "ok",
		"feed_id": feedID,
		"is_new":  isNew,
	})
}

func (s *Server) handleAddFolder(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name     string `json:"name"`
		ParentID *int64 `json:"parent_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}
	if req.Name == "" {
		http.Error(w, "Name is required", http.StatusBadRequest)
		return
	}

	folderID, err := s.db.CreateFolder(req.Name, req.ParentID)
	if err != nil {
		http.Error(w, "Failed to create folder", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":    "ok",
		"folder_id": folderID,
	})
}

func (s *Server) handleGetDatabaseSettings(w http.ResponseWriter, r *http.Request) {
	envFilePath := os.Getenv("INFOVORE_ENV_FILE")
	if envFilePath == "" {
		envFilePath = ".env"
	}

	// Read current DB_URL from .env file (masked for display)
	dbURL := ""
	if file, err := os.Open(envFilePath); err == nil {
		defer file.Close()
		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if strings.HasPrefix(line, "DB_URL=") {
				dbURL = strings.TrimPrefix(line, "DB_URL=")
				// Remove quotes if present
				if len(dbURL) >= 2 && ((dbURL[0] == '"' && dbURL[len(dbURL)-1] == '"') ||
					(dbURL[0] == '\'' && dbURL[len(dbURL)-1] == '\'')) {
					dbURL = dbURL[1 : len(dbURL)-1]
				}
				break
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"db_url":        dbURL,
		"database_type": s.db.DatabaseType(),
		"env_file":      envFilePath,
	})
}

func (s *Server) handleSaveDatabaseSettings(w http.ResponseWriter, r *http.Request) {
	var req struct {
		DBURL string `json:"db_url"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	envFilePath := os.Getenv("INFOVORE_ENV_FILE")
	if envFilePath == "" {
		envFilePath = ".env"
	}

	// Read existing .env file content (preserving other variables)
	existingLines := []string{}
	dbURLFound := false

	if file, err := os.Open(envFilePath); err == nil {
		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			line := scanner.Text()
			if strings.HasPrefix(strings.TrimSpace(line), "DB_URL=") {
				dbURLFound = true
				if req.DBURL != "" {
					existingLines = append(existingLines, fmt.Sprintf("DB_URL=%s", req.DBURL))
				}
				// If empty, skip the line (remove the setting)
			} else {
				existingLines = append(existingLines, line)
			}
		}
		file.Close()
	}

	// Add DB_URL if it wasn't found and we have a value
	if !dbURLFound && req.DBURL != "" {
		existingLines = append(existingLines, fmt.Sprintf("DB_URL=%s", req.DBURL))
	}

	// Write the .env file
	file, err := os.Create(envFilePath)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to write .env file: %v", err), http.StatusInternalServerError)
		return
	}
	defer file.Close()

	for _, line := range existingLines {
		file.WriteString(line + "\n")
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":       "ok",
		"env_file":     envFilePath,
		"restart_hint": "Restart the application to apply database changes",
	})
}

// --- Helpers ---

func (s *Server) render(w http.ResponseWriter, name string, data interface{}) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.templates.ExecuteTemplate(w, name, data); err != nil {
		log.Printf("Template error: %v", err)
		http.Error(w, "Render error", http.StatusInternalServerError)
	}
}

func timeAgo(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd ago", int(d.Hours()/24))
	}
}
