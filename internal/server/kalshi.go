package server

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/bryan-buckman/infovore/internal/database"
	kalshiconfig "github.com/bryan-buckman/infovore/internal/kalshi/config"
	"github.com/bryan-buckman/infovore/internal/kalshi/scanner"
)

// KalshiManager coordinates Kalshi scanning, report storage, and scheduling.
type KalshiManager struct {
	db database.Store

	mu        sync.RWMutex
	scanning  bool
	lastScan  time.Time
	lastError string
	stopCh    chan struct{}
}

// NewKalshiManager creates a new manager and starts the background scheduler.
func NewKalshiManager(db database.Store) *KalshiManager {
	m := &KalshiManager{
		db:     db,
		stopCh: make(chan struct{}),
	}
	go m.scheduler()
	return m
}

// Stop stops the background scheduler.
func (m *KalshiManager) Stop() {
	close(m.stopCh)
}

// scheduler runs scans on a configured interval.
func (m *KalshiManager) scheduler() {
	// Initial delay to let the server start up
	timer := time.NewTimer(30 * time.Second)
	defer timer.Stop()

	for {
		select {
		case <-m.stopCh:
			return
		case <-timer.C:
			// Check if Kalshi is configured
			apiKey, _ := m.db.GetSetting("kalshi_api_key_id")
			if apiKey == "" {
				// Not configured, check again in 5 minutes
				timer.Reset(5 * time.Minute)
				continue
			}

			// Run scan if enough time has passed
			m.mu.RLock()
			lastScan := m.lastScan
			m.mu.RUnlock()

			intervalStr, _ := m.db.GetSetting("kalshi_scan_interval_hours")
			intervalHours, err := strconv.Atoi(intervalStr)
			if err != nil || intervalHours < 1 {
				intervalHours = 6
			}

			if time.Since(lastScan) >= time.Duration(intervalHours)*time.Hour {
				m.runScan()
			}

			timer.Reset(5 * time.Minute) // Check every 5 minutes
		}
	}
}

// runScan executes a market scan and stores the result.
func (m *KalshiManager) runScan() {
	m.mu.Lock()
	if m.scanning {
		m.mu.Unlock()
		return
	}
	m.scanning = true
	m.mu.Unlock()

	defer func() {
		m.mu.Lock()
		m.scanning = false
		m.lastScan = time.Now()
		m.mu.Unlock()
	}()

	log.Println("[kalshi] Starting market scan...")

	// Load config from database
	cfg, err := kalshiconfig.LoadFromSettings(m.db)
	if err != nil {
		errMsg := fmt.Sprintf("config error: %v", err)
		log.Printf("[kalshi] %s", errMsg)
		m.mu.Lock()
		m.lastError = errMsg
		m.mu.Unlock()
		_ = m.db.AddKalshiScanLog("", errMsg)
		return
	}

	// Load categories from settings
	catStr, _ := m.db.GetSetting("kalshi_categories")
	categories := strings.Split(catStr, ",")
	for i := range categories {
		categories[i] = strings.TrimSpace(categories[i])
	}
	if len(categories) == 0 || (len(categories) == 1 && categories[0] == "") {
		categories = []string{"Politics"}
	}

	scanCfg := scanner.ScanConfig{
		Categories: categories,
	}

	result, err := scanner.RunScan(cfg, scanCfg)
	if err != nil {
		errMsg := fmt.Sprintf("scan error: %v", err)
		log.Printf("[kalshi] %s", errMsg)
		m.mu.Lock()
		m.lastError = errMsg
		m.mu.Unlock()
		_ = m.db.AddKalshiScanLog("", errMsg)
		return
	}

	// Save report
	if err := m.db.SaveKalshiReport(result.HTML); err != nil {
		log.Printf("[kalshi] Failed to save report: %v", err)
	}

	// Save log
	_ = m.db.AddKalshiScanLog(result.Log, "")

	m.mu.Lock()
	m.lastError = ""
	m.mu.Unlock()

	log.Printf("[kalshi] Scan complete: %d markets found", result.NumMarkets)
}

// --- HTTP Handlers ---

// HandleKalshiStatus returns the current scan status as JSON.
func (m *KalshiManager) HandleKalshiStatus(w http.ResponseWriter, r *http.Request) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	status := struct {
		Scanning  bool      `json:"scanning"`
		LastScan  time.Time `json:"last_scan"`
		LastError string    `json:"last_error,omitempty"`
	}{
		Scanning:  m.scanning,
		LastScan:  m.lastScan,
		LastError: m.lastError,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
}

// HandleKalshiRefresh triggers a manual scan.
func (m *KalshiManager) HandleKalshiRefresh(w http.ResponseWriter, r *http.Request) {
	m.mu.RLock()
	busy := m.scanning
	m.mu.RUnlock()

	if busy {
		http.Error(w, `{"error":"scan already in progress"}`, http.StatusConflict)
		return
	}

	go m.runScan()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "scan started"})
}

// HandleKalshiLog returns recent scan log entries.
func (m *KalshiManager) HandleKalshiLog(w http.ResponseWriter, r *http.Request) {
	entries, err := m.db.GetKalshiScanLog(10)
	if err != nil {
		http.Error(w, "Failed to read scan log", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(entries)
}

// HandleMarketsPage serves the latest Kalshi report HTML at /markets.
func (m *KalshiManager) HandleMarketsPage(w http.ResponseWriter, r *http.Request) {
	html, generatedAt, err := m.db.GetLatestKalshiReport()
	if err != nil {
		http.Error(w, "Failed to load report", http.StatusInternalServerError)
		return
	}

	if html == "" {
		// No report yet — show a placeholder
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, `<!DOCTYPE html>
<html><head><title>Kalshi Markets</title>
<style>body{font-family:Inter,sans-serif;display:flex;justify-content:center;align-items:center;height:100vh;background:#f8fafc;color:#334155;}
.card{text-align:center;padding:2rem;background:white;border-radius:12px;box-shadow:0 1px 3px rgba(0,0,0,0.1);}
h1{font-size:1.5rem;margin-bottom:0.5rem;} p{color:#94a3b8;}</style></head>
<body><div class="card"><h1>📊 No Report Yet</h1>
<p>Configure your Kalshi API key in Settings, then click Refresh to run a scan.</p></div></body></html>`)
		return
	}

	_ = generatedAt // Available for future use
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(html))
}
