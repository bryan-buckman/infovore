# Plan: Merge Kalshi Scanner into Infovore as a Tabbed Webapp

## Overview

Merge the Kalshi prediction markets scanner (from `/mnt/f/Projects/kalshi_API`) into the Infovore RSS reader (this project) to create a single Dockerized webapp with a tabbed interface switching between "Reader" and "Markets" views, a unified design system based on the Kalshi app's look and feel, a light/dark mode toggle, a unified Settings page, and PostgreSQL as the sole storage backend.

Both projects are Go 1.22, both serve on port 8080, both use embedded static assets, and both are Dockerized with Alpine/Debian slim runtimes. The merge is structurally clean.

---

## Current Architecture Summary

### Infovore (this project)
- **Go 1.22** with chi router, SQLite/Postgres, gofeed
- **Server-rendered** via Go `html/template` with a single `layout.html`
- **Embedded assets** (`//go:embed`) for templates, CSS, JS
- **Single binary** `infovore` serves everything on `:8080`
- **Dockerfile**: `golang:1.22-bookworm` build, `debian:bookworm-slim` runtime
- **Data**: SQLite DB at `/data/infovore.db`, optional `.env` for DB_URL
- **Visual style**: Dark theme, GitHub-inspired — `#0d1117` bg, `#58a6ff` accent, Inter font

### Kalshi Scanner (`/mnt/f/Projects/kalshi_API`)
- **Go 1.22** with stdlib `net/http`, Kalshi API client, Kelly criterion math
- **Two binaries**: `kalshi` (scanner CLI) and `kalshi-serve` (HTTP server)
- `kalshi` generates a **self-contained HTML report file** (`kalshi_report.html`) with all CSS/JS inline
- `kalshi-serve` serves the report file and provides API endpoints (`/api/refresh`, `/api/status`, `/api/config`, `/api/log`)
- **Dockerfile**: `golang:1.22-alpine` build, `alpine:3.19` runtime
- **Data**: `/data/config.json` (flat file), `/data/reports/kalshi_report.html` (flat file), `/data/scanner.log` (flat file)
- **Credentials**: `KALSHI_API_KEY_ID`, `KALSHI_PRIVATE_KEY_PATH`, `KALSHI_ENV` (environment variables)
- **Cron**: 4-hour scan cycle via Alpine crond
- **Visual style**: Light theme, financial/corporate — `#f8fafc` bg, `#2563eb` accent, Inter font, SF Mono for tickers

---

## Target Architecture

```
infovore (single binary)
  ├── GET /                    → tabbed shell page (tab selector + content area)
  ├── GET /reader              → Infovore RSS reader
  ├── GET /reader/feed/{id}    → RSS feed view
  ├── GET /markets             → Kalshi report (from DB or placeholder)
  │
  ├── /api/...                 → existing Infovore API (unchanged)
  ├── /api/kalshi/refresh      → trigger Kalshi scan
  ├── /api/kalshi/status       → scan status (scanning, last_error, last_output, report_ready)
  ├── /api/kalshi/log          → scanner log tail (from DB)
  ├── /api/settings            → GET/POST unified settings (all config)
  │
  ├── /static/...              → unified static assets (CSS, JS)
  │
  ├── PostgreSQL               → sole storage backend
  │   ├── folders, feeds, items, settings  (existing Infovore tables)
  │   ├── kalshi_reports       → generated HTML reports
  │   ├── kalshi_scan_log      → scan log entries
  │   └── settings             → all config (DB URL, RSS polling, Kalshi creds, categories, theme)
  │
  └── Background:
      ├── RSS Poller (existing, disabled by default)
      └── Kalshi Scanner (goroutine, interval from settings)
```

### Single binary, single container, single port. PostgreSQL only — no SQLite, no flat files.

---

## Storage: PostgreSQL Only

### Rationale

- SQLite is too slow for concurrent read/write workloads (Infovore already limits to 1 sequential worker on SQLite)
- Flat files (config.json, report HTML, scanner.log) create state management complexity and Docker volume issues
- A single PostgreSQL instance is simpler to operate, back up, and monitor
- The existing Infovore `settings` table (key-value) already provides the mechanism for all configuration

### Remove SQLite

- Delete `internal/database/sqlite.go`
- Remove `modernc.org/sqlite` from `go.mod`
- Remove the SQLite branch from `main.go` (the `if dbURL == ""` fallback)
- The app requires a `DB_URL` to start — either via env var or `.env` file
- On first launch with no DB_URL, the app shows a setup page (see First-Run Setup below)

### New Database Tables

Add to the PostgreSQL migration in `internal/database/postgres.go`:

```sql
-- Kalshi generated reports (replaces flat HTML files)
CREATE TABLE IF NOT EXISTS kalshi_reports (
    id SERIAL PRIMARY KEY,
    html TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
-- Keep only the latest report; old ones are deleted after insert
CREATE INDEX IF NOT EXISTS idx_kalshi_reports_created ON kalshi_reports(created_at DESC);

-- Kalshi scan log entries (replaces flat scanner.log file)
CREATE TABLE IF NOT EXISTS kalshi_scan_log (
    id SERIAL PRIMARY KEY,
    output TEXT NOT NULL,
    error TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
-- Auto-purge: keep last 50 entries (enforced by app after insert)
CREATE INDEX IF NOT EXISTS idx_kalshi_scan_log_created ON kalshi_scan_log(created_at DESC);
```

### Settings Keys (all in existing `settings` table)

| Key | Default | Description |
|-----|---------|-------------|
| `polling_interval_minutes` | `15` | RSS feed polling interval (existing) |
| `kalshi_api_key_id` | `""` | Kalshi API key ID |
| `kalshi_private_key` | `""` | Kalshi private key (PEM content, stored directly) |
| `kalshi_environment` | `prod` | `prod` or `demo` |
| `kalshi_categories` | `["Politics"]` | JSON array of enabled scan categories |
| `kalshi_scan_interval_hours` | `4` | Hours between automatic scans |
| `theme` | `light` | UI theme preference (`light` or `dark`) |

**Private key storage**: Instead of mounting a PEM file into the container and passing its path, the private key PEM content is pasted into the Settings page and stored in the `settings` table. The Kalshi config package is updated to read from DB instead of `KALSHI_PRIVATE_KEY_PATH` env var. This eliminates the need for volume mounts for credentials entirely.

**Security consideration**: The private key is stored in PostgreSQL. This is acceptable because:
- The database should already be on a private network / localhost
- It's equivalent to storing it as a file on disk (current approach)
- It avoids the operational complexity of file mounts and env vars
- If stricter security is needed later, the value can be encrypted at rest using a DB-level or app-level encryption key

### Store Interface Additions

Add to `internal/database/store.go`:

```go
// Kalshi report storage
SaveKalshiReport(html string) error          // Insert report, delete all previous
GetLatestKalshiReport() (string, time.Time, error)  // Get latest report HTML + timestamp

// Kalshi scan log
AddKalshiScanLog(output string, scanErr string) error  // Insert log entry, purge old
GetKalshiScanLog(limit int) ([]ScanLogEntry, error)    // Get recent log entries
```

---

## Unified Settings Page

### Design

Replace the current scattered settings (Infovore gear icon modal, Kalshi hamburger menu settings dialog, env vars, .env file) with a single, full-page Settings view accessible from the hamburger menu on every tab.

The Settings page is a third tab-level route (`GET /settings`) but accessed via the hamburger menu rather than a persistent tab. It uses the same design tokens and layout patterns.

### Settings Page Layout

```
┌─────────────────────────────────────────────────────┐
│  ← Back                        Settings             │
├─────────────────────────────────────────────────────┤
│                                                     │
│  DATABASE CONNECTION                                │
│  ┌─────────────────────────────────────────────┐    │
│  │ PostgreSQL URL                              │    │
│  │ [postgres://user:***@host:5432/infovore   ] │    │
│  │ Status: ● Connected                         │    │
│  └─────────────────────────────────────────────┘    │
│                                                     │
│  RSS READER                                         │
│  ┌─────────────────────────────────────────────┐    │
│  │ Polling interval: [15] minutes              │    │
│  │ [Import OPML]  [Export OPML]                 │    │
│  └─────────────────────────────────────────────┘    │
│                                                     │
│  KALSHI MARKETS                                     │
│  ┌─────────────────────────────────────────────┐    │
│  │ API Key ID:    [_________________________ ] │    │
│  │ Environment:   (●) Production  ( ) Demo     │    │
│  │ Private Key:                                │    │
│  │ ┌─────────────────────────────────────────┐ │    │
│  │ │ -----BEGIN EC PRIVATE KEY-----          │ │    │
│  │ │ (paste PEM content here)                │ │    │
│  │ │ -----END EC PRIVATE KEY-----            │ │    │
│  │ └─────────────────────────────────────────┘ │    │
│  │ Status: ● Configured  [Test Connection]     │    │
│  │                                             │    │
│  │ Scan Categories:                            │    │
│  │ ☑ Politics    ☐ Sports    ☐ Crypto          │    │
│  │ ☐ Economics   ☐ Elections ☐ Entertainment   │    │
│  │ ☐ Financials  ☐ Health   ☐ Companies        │    │
│  │ ☐ Science...  ☐ Social   ☐ World            │    │
│  │ ☐ Climate...  ☐ Mentions                    │    │
│  │                                             │    │
│  │ Scan interval: [4] hours                    │    │
│  └─────────────────────────────────────────────┘    │
│                                                     │
│  [Save Settings]                                    │
│                                                     │
└─────────────────────────────────────────────────────┘
```

### Settings API

**`GET /api/settings`** — returns all settings as JSON:
```json
{
  "polling_interval_minutes": 15,
  "kalshi_api_key_id": "abc123",
  "kalshi_private_key_configured": true,
  "kalshi_environment": "prod",
  "kalshi_categories": ["Politics"],
  "kalshi_scan_interval_hours": 4,
  "theme": "light",
  "db_url_masked": "postgres://user:***@host:5432/infovore",
  "db_connected": true
}
```

Note: `kalshi_private_key` is never returned in GET responses — only a boolean `kalshi_private_key_configured` indicating whether one is stored. The actual key is write-only from the API perspective.

**`POST /api/settings`** — accepts partial updates (only include keys to change):
```json
{
  "kalshi_api_key_id": "new-key",
  "kalshi_private_key": "-----BEGIN EC PRIVATE KEY-----\n...",
  "kalshi_categories": ["Politics", "Sports"]
}
```

**`POST /api/settings/test-kalshi`** — tests Kalshi credentials by attempting an API call. Returns `{"ok": true}` or `{"ok": false, "error": "..."}`.

### First-Run Setup

On first launch with no `DB_URL` configured:

1. The app starts and serves a setup page at all routes
2. The setup page has a single field: PostgreSQL connection URL
3. On submit, the app:
   - Tests the connection
   - Runs migrations
   - Writes `DB_URL` to `/data/.env`
   - Restarts itself (or prompts the user to restart the container)

This replaces the current pattern of passing `DB_URL` as an env var or editing `.env` manually.

### Removing the Hamburger Menu Settings Dialog

The Kalshi report's current hamburger menu settings dialog (category checkboxes + save/refresh) is replaced by the unified Settings page. The hamburger menu in the report retains only:
- **Settings** → navigates to `/settings` (the full settings page)
- **Refresh** → triggers a scan (POST `/api/kalshi/refresh`)

### Refactoring the Kalshi Config Package

**File: `internal/kalshi/config/config.go`** — rewrite to read from the database Store instead of environment variables:

```go
func LoadFromStore(store database.Store) (*Config, error) {
    keyID, _ := store.GetSetting("kalshi_api_key_id")
    if keyID == "" {
        return nil, fmt.Errorf("Kalshi API key not configured (set in Settings)")
    }

    privateKey, _ := store.GetSetting("kalshi_private_key")
    if privateKey == "" {
        return nil, fmt.Errorf("Kalshi private key not configured (set in Settings)")
    }

    env, _ := store.GetSetting("kalshi_environment")
    if env == "" {
        env = "prod"
    }

    // ... build Config with BaseURL based on env
    // Private key is now PEM content string, not a file path
    // Auth signer needs updating to accept key bytes instead of file path
}
```

**File: `internal/kalshi/auth/`** — update the signer to accept PEM bytes directly (currently it reads from a file path). Add a `NewSignerFromBytes(pemData []byte)` constructor alongside the existing file-based one (keep file-based for `--local` mode).

---

## Unified Design System

### Design Philosophy

The Kalshi app's light theme is the primary visual identity. It has a clean, professional, financial-data aesthetic that works well for both data tables (markets) and content reading (RSS). The Infovore dark theme becomes the dark mode variant, harmonized to use the same accent colors and spacing as the Kalshi light theme.

Both apps already use **Inter** as their primary font and share similar structural patterns (cards, tables, modals), making unification straightforward.

### CSS Custom Properties (Design Tokens)

All styling uses CSS custom properties on `:root`, with dark mode overrides via `[data-theme="dark"]`. Both the shell page, the reader, and the Kalshi report share these tokens.

**File: `internal/server/static/css/theme.css`** (new, loaded by all pages)

```css
/* ===== LIGHT MODE (default, based on Kalshi) ===== */
:root {
  /* Primary palette */
  --primary:        #2563eb;
  --primary-dark:   #1d4ed8;
  --primary-light:  #dbeafe;

  /* Semantic colors */
  --success:        #16a34a;
  --success-light:  #dcfce7;
  --danger:         #dc2626;
  --danger-light:   #fee2e2;
  --warning:        #d97706;

  /* Surfaces */
  --bg:             #f8fafc;
  --bg-card:        #ffffff;
  --bg-hover:       #f1f5f9;
  --bg-input:       #f8fafc;
  --bg-overlay:     rgba(0, 0, 0, 0.4);

  /* Text */
  --text:           #1e293b;
  --text-muted:     #64748b;
  --text-on-primary: #ffffff;

  /* Borders & shadows */
  --border:         #e2e8f0;
  --shadow-sm:      0 1px 3px rgba(0, 0, 0, 0.1);
  --shadow-md:      0 4px 12px rgba(0, 0, 0, 0.1);
  --shadow-lg:      0 8px 24px rgba(0, 0, 0, 0.15);

  /* Layout */
  --radius-sm:      4px;
  --radius:         8px;
  --radius-lg:      12px;

  /* Fonts */
  --font-sans:      'Inter', -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
  --font-mono:      'SF Mono', Monaco, 'Courier New', monospace;

  /* Dark panel (Kelly criterion section, always dark regardless of theme) */
  --panel-dark-bg:  linear-gradient(135deg, #1e293b, #334155);
  --panel-dark-text: #93c5fd;
}

/* ===== DARK MODE ===== */
[data-theme="dark"] {
  /* Primary palette — brighter blue for dark backgrounds */
  --primary:        #58a6ff;
  --primary-dark:   #79c0ff;
  --primary-light:  rgba(88, 166, 255, 0.15);

  /* Semantic colors — brighter variants for dark bg contrast */
  --success:        #3fb950;
  --success-light:  rgba(63, 185, 80, 0.15);
  --danger:         #f85149;
  --danger-light:   rgba(248, 81, 73, 0.15);
  --warning:        #e3b341;

  /* Surfaces — GitHub-dark inspired, from Infovore */
  --bg:             #0d1117;
  --bg-card:        #161b22;
  --bg-hover:       #21262d;
  --bg-input:       #21262d;
  --bg-overlay:     rgba(0, 0, 0, 0.7);

  /* Text */
  --text:           #f0f6fc;
  --text-muted:     #8b949e;
  --text-on-primary: #000000;

  /* Borders & shadows — deeper for dark mode */
  --border:         #30363d;
  --shadow-sm:      0 1px 3px rgba(0, 0, 0, 0.3);
  --shadow-md:      0 4px 12px rgba(0, 0, 0, 0.3);
  --shadow-lg:      0 8px 24px rgba(0, 0, 0, 0.4);
}
```

### Color Palette Rationale

| Token | Light Value | Dark Value | Rationale |
|-------|-------------|------------|-----------|
| `--primary` | `#2563eb` (Kalshi blue) | `#58a6ff` (Infovore blue) | Kalshi's deeper blue works on white; Infovore's brighter blue needed for dark bg readability |
| `--bg` | `#f8fafc` (Kalshi) | `#0d1117` (Infovore) | Each app's existing page background, proven readable |
| `--bg-card` | `#ffffff` (Kalshi) | `#161b22` (Infovore) | Card surfaces from each app's existing design |
| `--text` | `#1e293b` (Kalshi slate) | `#f0f6fc` (Infovore near-white) | High-contrast primaries from each existing design |
| `--text-muted` | `#64748b` (Kalshi) | `#8b949e` (Infovore) | Both are mid-gray; each tuned for its bg contrast |
| `--border` | `#e2e8f0` (Kalshi) | `#30363d` (Infovore) | Subtle but visible dividers from each existing palette |
| `--success` | `#16a34a` (Kalshi) | `#3fb950` (Infovore) | Darker green on white, brighter green on dark |
| `--danger` | `#dc2626` (Kalshi) | `#f85149` (Infovore) | Same principle — brighter on dark |

### Dark Mode Toggle

A single crescent moon button in the upper-right corner of the shell tab bar. Applies to all tabs (reader + markets).

**Behavior:**
- Clicking toggles `data-theme="dark"` on `<html>` element
- Icon switches between moon (light mode active) and sun (dark mode active)
- Preference saved to both `localStorage` (for instant load) and DB `settings` table (for persistence across devices)
- On page load, reads `localStorage` before first paint (inline `<script>` in `<head>` to prevent flash)
- If iframe approach is used: parent posts `theme-changed` message to iframe; iframe sets its own `data-theme`

### Applying the Design System to Each App

#### Infovore Reader (currently dark-only)
- Replace all hardcoded color values in `style.css` with `var(--token)` references
- Remove the existing CSS custom properties block and import `theme.css` instead
- The reader's existing dark palette maps almost 1:1 to the dark mode tokens
- Light mode "just works" because all colors now come from tokens

#### Kalshi Report (currently light-only)
- Replace all hardcoded color values in `report.go`'s CSS with `var(--token)` references
- Remove the existing CSS custom properties block from the template
- Add `<link rel="stylesheet" href="/static/css/theme.css">` to the report template
- The Kelly criterion panel keeps its dark gradient in both modes
- For `--local` mode (report opened as a file), embed the theme CSS inline as a fallback

#### Shell Page + Settings Page
- The tab bar, theme toggle, and settings page all use the same tokens
- Settings page sections use card styling (`var(--bg-card)`, `var(--border)`, `var(--radius-lg)`)
- Form inputs use `var(--bg-input)`, `var(--border)`, `var(--text)`

---

## Implementation Steps

### Step 1: Copy Kalshi packages into Infovore

Copy these packages from `kalshi_API` into `infovore`:

```
kalshi_API/internal/client/     → infovore/internal/kalshi/client/
kalshi_API/internal/auth/       → infovore/internal/kalshi/auth/
kalshi_API/internal/config/     → infovore/internal/kalshi/config/
kalshi_API/internal/scanner/    → infovore/internal/kalshi/scanner/
kalshi_API/pkg/models/          → infovore/internal/kalshi/models/
```

Update all import paths from `github.com/bryanedds/kalshi-api/...` to `github.com/bryanedds/infovore/internal/kalshi/...`.

Add Kalshi's Go dependencies to `infovore/go.mod` (the Kalshi project uses only stdlib + crypto, so minimal additions).

**Verification**: `go build ./...` should pass after import path updates.

### Step 2: Remove SQLite, make PostgreSQL mandatory

- Delete `internal/database/sqlite.go`
- Remove `modernc.org/sqlite` from `go.mod`
- Remove the SQLite fallback in `main.go`
- Update `main.go` to require `DB_URL` (from env var or `.env`)
- If no `DB_URL` is set, serve the first-run setup page on all routes
- Remove `-db` CLI flag (SQLite path), keep `-db-url` as an alternative to env var

### Step 3: Add Kalshi database tables and store methods

**Update: `internal/database/postgres.go`**
- Add `kalshi_reports` and `kalshi_scan_log` tables to the migration
- Add new settings key defaults for Kalshi config
- Implement `SaveKalshiReport`, `GetLatestKalshiReport`, `AddKalshiScanLog`, `GetKalshiScanLog`

**Update: `internal/database/store.go`**
- Add new methods to the Store interface

### Step 4: Create the unified design system

**File: `internal/server/static/css/theme.css`** (new)

Contains all CSS custom properties for both light and dark modes as defined in the design system section above.

**Update: `internal/server/static/css/style.css`** (Infovore reader)
- Remove existing `:root { }` custom properties block
- Replace all hardcoded colors with `var(--token)` references

**Update: `internal/kalshi/scanner/report.go`** (Kalshi report template CSS)
- Remove inline `:root { }` custom properties block
- Add `<link rel="stylesheet" href="/static/css/theme.css">` in `<head>`
- Replace all hardcoded colors with `var(--token)` references
- For `--local` mode: embed theme CSS inline

### Step 5: Refactor Kalshi config to read from database

**Update: `internal/kalshi/config/config.go`**
- Add `LoadFromStore(store database.Store)` that reads credentials from the `settings` table
- Keep `Load()` (env var based) for `--local` mode only

**Update: `internal/kalshi/auth/`**
- Add `NewSignerFromBytes(pemData []byte)` alongside the existing file-path-based constructor
- The DB-stored private key is PEM content (string), not a file path

### Step 6: Absorb kalshi-serve into the Infovore server

**File: `internal/server/kalshi.go`** (new)

Create `KalshiManager` struct that:
- Reads config from DB via `LoadFromStore()`
- Runs scans as goroutines with streaming output
- Writes reports to `kalshi_reports` table instead of flat files
- Writes scan logs to `kalshi_scan_log` table
- Tracks scan state in memory (mutex-protected `scanning`, `lastScanOutput`, etc.)

```go
type KalshiManager struct {
    mu             sync.Mutex
    scanning       bool
    lastScanTime   time.Time
    lastScanError  string
    lastScanOutput string
    store          database.Store
}
```

**Route registration** in `internal/server/server.go`:
```go
km := NewKalshiManager(store)
r.Route("/api/kalshi", func(r chi.Router) {
    r.Post("/refresh", km.HandleRefresh)
    r.Get("/status", km.HandleStatus)
    r.Get("/log", km.HandleLog)
})
r.Get("/markets", km.HandleMarketsPage)
```

### Step 7: Refactor scanner CLI into a library function

**File: `internal/kalshi/scanner/run.go`** (new)

```go
type ScanConfig struct {
    Categories []string
    LogWriter  io.Writer
    Store      database.Store  // for saving report to DB
}

func RunScan(cfg ScanConfig) error {
    // Load credentials from DB via config.LoadFromStore()
    // Create API client
    // Scan categories, filter markets, calculate Kelly
    // Generate report HTML
    // Save to kalshi_reports table via cfg.Store.SaveKalshiReport()
    // Stream progress to cfg.LogWriter
}
```

### Step 8: Build the unified Settings page

**File: `internal/server/templates/settings.html`** (new)

Full-page settings view with sections for Database, RSS Reader, and Kalshi Markets as shown in the layout above.

**File: `internal/server/static/js/settings.js`** (new)

Handles:
- Loading current settings via `GET /api/settings`
- Saving settings via `POST /api/settings`
- Testing Kalshi credentials via `POST /api/settings/test-kalshi`
- OPML import/export buttons
- Private key textarea (write-only — shows placeholder if configured, never shows actual key)

**Update: `internal/server/server.go`**
- Rewrite `handleGetSettings` and `handleSaveSettings` to handle all settings keys
- Add `handleTestKalshi` endpoint
- Add `GET /settings` route to serve the settings page
- Remove `handleGetDatabaseSettings` / `handleSaveDatabaseSettings` (absorbed into unified settings)

**Update: Kalshi report hamburger menu** in `report.go`
- Remove the inline category settings dialog
- "Settings" menu item now navigates to `/settings` (the full settings page)
- Keep "Refresh" as a direct action (POST `/api/kalshi/refresh`)

**Update: Infovore reader** in `layout.html` / `app.js`
- Remove the gear icon settings modal
- Add hamburger menu (matching Kalshi's) with "Settings" linking to `/settings`

### Step 9: Add the tabbed shell page

**File: `internal/server/templates/shell.html`** (new)

Tab bar with Reader and Markets tabs, plus the dark mode toggle in the upper right. Uses iframe for content isolation.

**Theme propagation:**
- Parent posts `{type: 'theme', value: 'dark'}` to iframe on toggle
- Each iframe page listens and sets `data-theme`
- `localStorage` provides instant load; DB `settings.theme` provides cross-device persistence

**Route: `GET /`** → serves `shell.html`, defaults to Reader tab.

### Step 10: Background scanner goroutine

**In `internal/server/kalshi.go`:**
```go
func (km *KalshiManager) StartScheduler() {
    go func() {
        for {
            hours, _ := km.store.GetSetting("kalshi_scan_interval_hours")
            interval := parseDuration(hours, 4) // default 4h
            time.Sleep(interval)
            km.RunScan()
        }
    }()
}
```

Reads interval from DB on each cycle so changes in Settings take effect without restart.

### Step 11: Unified Dockerfile

```dockerfile
FROM golang:1.22-bookworm AS builder
WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o infovore .

FROM debian:bookworm-slim
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates && rm -rf /var/lib/apt/lists/*
RUN useradd -r -u 1001 infovore
RUN mkdir -p /data && chown -R infovore:infovore /data
COPY --from=builder /build/infovore /usr/local/bin/

USER infovore
EXPOSE 8080
VOLUME ["/data"]
CMD ["infovore", "-addr", ":8080", "-data-dir", "/data"]
```

No cron, no entrypoint script, no second binary, no SQLite file, no key file mount. Single process, single volume (just for `.env`).

### Step 12: Update CLI flags and docker run

```go
// main.go flags
addr     := flag.String("addr", ":8080", "HTTP listen address")
dbURL    := flag.String("db-url", "", "PostgreSQL URL (overrides DB_URL env var)")
dataDir  := flag.String("data-dir", "/data", "Data directory (for .env file)")
```

```bash
docker run -d -p 8080:8080 \
  -v infovore-data:/data \
  -e DB_URL=postgres://user:pass@host:5432/infovore \
  infovore
```

All Kalshi credentials are configured via the Settings page in the browser after first launch. No env vars needed beyond `DB_URL`.

### Step 13: Write PostgreSQL setup documentation

**File: `docs/db.md`** (new)

Write documentation covering:

1. **Proxmox VM sizing** for the combined Infovore + Kalshi workload:
   - CPU: 1-2 vCPUs (the app is I/O bound, not CPU bound)
   - RAM: 1 GB minimum, 2 GB recommended (PostgreSQL shared_buffers + Go runtime)
   - Disk: 10 GB minimum (PostgreSQL data + WAL + report HTML storage)
   - Network: bridged, static IP recommended for stable DB_URL

2. **PostgreSQL installation** on Debian/Ubuntu (the likely Proxmox guest OS):
   - `apt install postgresql postgresql-contrib`
   - Version: 15+ recommended

3. **Database and user setup**:
   ```sql
   CREATE USER infovore WITH PASSWORD 'secure-password-here';
   CREATE DATABASE infovore OWNER infovore;
   GRANT ALL PRIVILEGES ON DATABASE infovore TO infovore;
   ```

4. **PostgreSQL configuration** (`postgresql.conf`):
   - `listen_addresses = '*'` (or specific IP for the Docker host)
   - `max_connections = 20` (more than enough for a single-user app)
   - `shared_buffers = 256MB`
   - `work_mem = 4MB`
   - `effective_cache_size = 512MB`
   - `wal_level = minimal` (no replication needed for single instance)

5. **Network access** (`pg_hba.conf`):
   - Allow connections from Docker network / container IP range
   - `host infovore infovore 172.17.0.0/16 scram-sha-256` (Docker default bridge)
   - Or use the Proxmox VM's LAN IP if Docker is on a different host

6. **Connection string format**:
   - `postgres://infovore:password@vm-ip:5432/infovore?sslmode=disable`
   - Use `sslmode=require` if connecting over untrusted networks

7. **Backup strategy**:
   - `pg_dump infovore > backup.sql` (daily cron on the VM)
   - Or use `pg_basebackup` for physical backups

8. **Expected traffic profile**:
   - RSS: ~50-200 INSERT/UPDATE per feed refresh (every 15min if polling enabled, or manual)
   - Kalshi: ~5000 SELECT-equivalent API calls per scan (every 4h), 1 large INSERT (report HTML), 1 INSERT (scan log)
   - Read traffic: minimal (single user, page loads)
   - Total: well within a 1-vCPU VM's capacity

---

## Migration Checklist

### Backend
- [ ] Copy Kalshi packages into `internal/kalshi/`
- [ ] Update all import paths
- [ ] Add dependencies to `go.mod` (run `go mod tidy`)
- [ ] Delete `internal/database/sqlite.go` and remove SQLite deps
- [ ] Make PostgreSQL mandatory in `main.go` (with first-run setup page)
- [ ] Add `kalshi_reports` and `kalshi_scan_log` tables to Postgres migration
- [ ] Add Kalshi settings key defaults to migration
- [ ] Implement new Store interface methods for Kalshi data
- [ ] Refactor `internal/kalshi/config/config.go` to read from DB Store
- [ ] Add `NewSignerFromBytes()` to `internal/kalshi/auth/`
- [ ] Create `internal/kalshi/scanner/run.go` — extract scan orchestration as library function
- [ ] Update `run.go` to save reports to DB instead of flat files
- [ ] Create `internal/server/kalshi.go` — KalshiManager with routes, scan state, scheduler
- [ ] Build unified Settings API (`GET/POST /api/settings`, `POST /api/settings/test-kalshi`)
- [ ] Remove old settings/database-settings endpoints
- [ ] Register all new routes in `server.go`
- [ ] Start Kalshi scheduler goroutine on server init

### Frontend
- [ ] Create `internal/server/static/css/theme.css` with light/dark tokens
- [ ] Refactor `style.css` to use theme tokens (remove hardcoded colors)
- [ ] Refactor `report.go` CSS to use theme tokens (remove hardcoded colors)
- [ ] Add theme.css `<link>` to Kalshi report template `<head>`
- [ ] Add inline theme.css fallback for `--local` mode reports
- [ ] Create `shell.html` template with tab bar, theme toggle, iframe
- [ ] Create `settings.html` template (full settings page)
- [ ] Create `settings.js` (settings page logic)
- [ ] Add FOUC-prevention inline script in shell.html `<head>`
- [ ] Add theme toggle JS with localStorage + DB persistence
- [ ] Add iframe theme propagation (postMessage)
- [ ] Remove Kalshi hamburger menu settings dialog (replace with link to /settings)
- [ ] Remove Infovore gear icon settings modal (replace with hamburger menu link)
- [ ] Update Kalshi report API paths to `/api/kalshi/*`
- [ ] Update Kalshi placeholder page API paths

### Infrastructure
- [ ] Update Dockerfile (single binary, no cron, no entrypoint, no SQLite)
- [ ] Remove `docker-entrypoint.sh`
- [ ] Update `.dockerignore`
- [ ] Create `docs/db.md` with Proxmox VM + PostgreSQL setup guide

### Testing
- [ ] `go build ./... && go vet ./... && go test ./...`
- [ ] RSS reader works at `/reader` in light mode
- [ ] RSS reader works at `/reader` in dark mode
- [ ] Kalshi report works at `/markets` in light mode
- [ ] Kalshi report works at `/markets` in dark mode
- [ ] Settings page loads and saves all fields
- [ ] Kalshi credentials test button works
- [ ] Theme toggle persists across tab switches and page reloads
- [ ] Tab switching works
- [ ] First-run setup page works (no DB_URL → setup → connect)
- [ ] Docker build and run with PostgreSQL
- [ ] `--local` flag still works for Kalshi (with inline theme CSS, env var credentials)
- [ ] Background scan scheduler runs at configured interval
- [ ] Scan reports saved to and served from PostgreSQL
- [ ] Scan logs saved to and queryable from PostgreSQL

---

## Files Changed (Infovore)

| File | Change |
|------|--------|
| `main.go` | Remove SQLite fallback, require DB_URL, init KalshiManager |
| `go.mod` / `go.sum` | Remove SQLite deps, add Kalshi deps |
| `internal/database/sqlite.go` | DELETED |
| `internal/database/store.go` | Add Kalshi store methods to interface |
| `internal/database/postgres.go` | Add Kalshi tables, settings defaults, new methods |
| `internal/server/server.go` | Move `/` to `/reader`, add shell/settings/Kalshi routes, rewrite settings handlers |
| `internal/server/kalshi.go` | NEW — KalshiManager, route handlers, scan state, scheduler |
| `internal/server/templates/shell.html` | NEW — tabbed shell with theme toggle |
| `internal/server/templates/settings.html` | NEW — unified settings page |
| `internal/server/templates/layout.html` | Add theme.css link, add hamburger menu, remove gear modal |
| `internal/server/static/css/theme.css` | NEW — unified design tokens (light + dark) |
| `internal/server/static/css/style.css` | Replace all hardcoded colors with `var(--token)` references |
| `internal/server/static/js/app.js` | Remove settings modal, add hamburger menu |
| `internal/server/static/js/settings.js` | NEW — settings page logic |
| `internal/kalshi/` | NEW — entire directory copied from kalshi_API |
| `internal/kalshi/config/config.go` | Add `LoadFromStore()`, keep `Load()` for local mode |
| `internal/kalshi/auth/` | Add `NewSignerFromBytes()` |
| `internal/kalshi/scanner/run.go` | NEW — scan orchestration, writes to DB |
| `internal/kalshi/scanner/report.go` | Replace hardcoded colors with tokens, update API paths, remove inline settings dialog |
| `Dockerfile` | Simplify: single binary, no cron/entrypoint/SQLite |
| `docs/db.md` | NEW — Proxmox VM sizing + PostgreSQL setup guide |

## Files to NOT Copy

- `kalshi_API/cmd/kalshi/main.go` — logic extracted into `scanner/run.go`
- `kalshi_API/cmd/serve/main.go` — logic absorbed into `server/kalshi.go`
- `kalshi_API/docker-entrypoint.sh` — no longer needed
- `kalshi_API/Dockerfile` — replaced by unified Dockerfile
- `kalshi_API/.devcontainer/` — use Infovore's devcontainer

---

## Risk Assessment

| Risk | Mitigation |
|------|------------|
| CSS/JS conflicts between reader and report | Iframe isolation + shared theme tokens |
| Theme flash on page load (FOUC) | Inline `<script>` in `<head>` reads localStorage before paint |
| Theme not propagating to iframe | postMessage listener + localStorage fallback on iframe load |
| Kalshi `--local` mode breaks (no server for theme.css) | Embed theme.css inline when generating standalone report |
| Kalshi `--local` mode breaks (no DB for config) | Keep env-var-based `Load()` for local mode |
| Private key stored in DB | Equivalent security to file-on-disk; DB should be on private network |
| PostgreSQL required but not running | First-run setup page guides user; clear error messages |
| No SQLite fallback for quick testing | Use Docker Compose with a Postgres container for dev; or connect to existing Postgres |
| Report HTML large in DB | Single report stored; `TEXT` column handles it fine; old reports deleted |
| Scanner blocking the HTTP server | Runs in goroutine with mutex, already proven |
| Import path update mistakes | `go build ./...` catches all errors immediately |
| Color token swap misses a hardcoded value | Visual regression — easy to spot and fix iteratively |

---

## Estimated Scope

- ~22 files touched/created
- ~500 lines of new code (KalshiManager, settings page, theme system, DB methods, shell)
- ~150 lines of find-and-replace (hardcoded colors → token references)
- ~100 lines of deletions (SQLite code, old settings modals)
- ~50 lines of API path updates
- ~200 lines of documentation (docs/db.md)
- The Kalshi scanner logic and the Infovore reader logic are completely independent — they share only the HTTP server, the PostgreSQL database, and the design tokens
