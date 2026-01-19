// Package server provides the HTTP server and handlers.
package server

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"html/template"
	"io/fs"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/bryan-buckman/infovore/internal/database"
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
	db         *database.DB
	fetcher    *rss.Fetcher
	poller     *rss.Poller
	router     chi.Router
	httpServer *http.Server
	templates  *template.Template
}

// New creates a new server.
func New(db *database.DB) (*Server, error) {
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

	// Pages.
	r.Get("/", s.handleHome)
	r.Get("/feed/{feedID}", s.handleFeed)
	r.Get("/folder/{folderID}", s.handleFolder)

	// API.
	r.Route("/api", func(r chi.Router) {
		r.Post("/mark-read", s.handleMarkRead)
		r.Post("/delete-read", s.handleDeleteRead)
		r.Post("/settings", s.handleSaveSettings)
		r.Get("/settings", s.handleGetSettings)
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
	})

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

func (s *Server) handleHome(w http.ResponseWriter, r *http.Request) {
	foldersWithFeeds, _ := s.db.GetFoldersWithFeeds()
	unfiledFeeds, _ := s.db.GetUnfiledFeeds()
	items, _ := s.db.GetAllItems(false)
	interval, _ := s.db.GetPollingInterval()

	data := map[string]interface{}{
		"FoldersWithFeeds": foldersWithFeeds,
		"UnfiledFeeds":     unfiledFeeds,
		"Items":            items,
		"PollingInterval":  interval,
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
	interval, _ := s.db.GetPollingInterval()

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
		"PollingInterval":  interval,
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
	interval, _ := s.db.GetPollingInterval()

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
		"PollingInterval":  interval,
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
	var req struct {
		PollingInterval int `json:"polling_interval"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}
	// Enforce minimum.
	if req.PollingInterval < rss.MinPollingIntervalMinutes {
		req.PollingInterval = rss.MinPollingIntervalMinutes
	}
	if err := s.db.SetSetting(model.SettingPollingInterval, strconv.Itoa(req.PollingInterval)); err != nil {
		http.Error(w, "Failed to save", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{"status": "ok", "polling_interval": req.PollingInterval})
}

func (s *Server) handleGetSettings(w http.ResponseWriter, r *http.Request) {
	interval, _ := s.db.GetPollingInterval()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"polling_interval": interval,
	})
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
