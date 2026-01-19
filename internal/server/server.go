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
	db        *database.DB
	fetcher   *rss.Fetcher
	poller    *rss.Poller
	router    chi.Router
	templates *template.Template
}

// New creates a new server.
func New(db *database.DB) (*Server, error) {
	tmpl, err := template.New("").Funcs(template.FuncMap{
		"timeAgo": timeAgo,
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

	// API.
	r.Route("/api", func(r chi.Router) {
		r.Post("/mark-read", s.handleMarkRead)
		r.Post("/settings", s.handleSaveSettings)
		r.Get("/settings", s.handleGetSettings)
		r.Post("/import-opml", s.handleImportOPML)
		r.Get("/export-opml", s.handleExportOPML)
		r.Post("/refresh", s.handleRefresh)
		r.Post("/cleanup", s.handleCleanup)
		r.Get("/sidebar", s.handleSidebar)
	})

	s.router = r
}

// Start starts the server and poller.
func (s *Server) Start(addr string) error {
	s.poller.Start()
	log.Printf("Server starting on %s", addr)
	return http.ListenAndServe(addr, s.router)
}

// Stop stops the poller.
func (s *Server) Stop() {
	s.poller.Stop()
}

// --- Page Handlers ---

func (s *Server) handleHome(w http.ResponseWriter, r *http.Request) {
	folders, _ := s.db.GetFolders()
	feeds, _ := s.db.GetAllFeeds()
	items, _ := s.db.GetAllItems(false)
	interval, _ := s.db.GetPollingInterval()

	data := map[string]interface{}{
		"Folders":         folders,
		"Feeds":           feeds,
		"Items":           items,
		"PollingInterval": interval,
	}
	s.render(w, "layout.html", data)
}

func (s *Server) handleFeed(w http.ResponseWriter, r *http.Request) {
	feedIDStr := chi.URLParam(r, "feedID")
	feedID, _ := strconv.ParseInt(feedIDStr, 10, 64)

	folders, _ := s.db.GetFolders()
	feeds, _ := s.db.GetAllFeeds()
	items, _ := s.db.GetItems(feedID, false)
	interval, _ := s.db.GetPollingInterval()

	data := map[string]interface{}{
		"Folders":         folders,
		"Feeds":           feeds,
		"Items":           items,
		"CurrentFeedID":   feedID,
		"PollingInterval": interval,
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
