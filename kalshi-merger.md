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
SaveKalshiReport(html string) error                    // Insert report, delete all previous
GetLatestKalshiReport() (string, time.Time, error)     // Get latest report HTML + timestamp

// Kalshi scan log
AddKalshiScanLog(output string, scanErr string) error  // Insert log entry, purge old
GetKalshiScanLog(limit int) ([]ScanLogEntry, error)    // Get recent log entries
```

Add the `ScanLogEntry` type to `store.go`:
```go
type ScanLogEntry struct {
    ID        int64
    Output    string
    Error     string
    CreatedAt time.Time
}
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
│  │ Private Key (RSA PEM):                      │    │
│  │ ┌─────────────────────────────────────────┐ │    │
│  │ │ -----BEGIN RSA PRIVATE KEY-----         │ │    │
│  │ │ (paste PEM content here)                │ │    │
│  │ │ -----END RSA PRIVATE KEY-----           │ │    │
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
  "kalshi_private_key": "-----BEGIN RSA PRIVATE KEY-----\n...",
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

See **Step 5** in the Implementation Steps section for full details on `config.LoadFromStore()`, `auth.NewSignerFromBytes()`, and the updated `client.New()` constructor chain.

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
  --text-on-primary: #ffffff;

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
  client.go           — API client with rate limiting (Get, Post, GetSeriesList, GetMarkets,
                        GetBalance, GetPositions, GetMarket, GetSettlements, GetFills, GetCategories)
kalshi_API/internal/auth/       → infovore/internal/kalshi/auth/
  signer.go           — RSA-PSS request signing (NewSigner, SignRequest)
  signer_test.go      — signer unit tests
kalshi_API/internal/config/     → infovore/internal/kalshi/config/
  config.go           — credential loading (currently env-var-based Load())
kalshi_API/internal/scanner/    → infovore/internal/kalshi/scanner/
  scanner.go          — market filtering (FilterByPriceRange*, SortByAskPrice)
  kelly.go            — Kelly criterion math (CalculateKelly, LongshotBiasEdge)
  kelly_test.go       — Kelly criterion tests
  report.go           — HTML report generation (~1750 lines, contains the full HTML/CSS/JS template
                        as a Go template string, plus types: ReportMarket, EnrichedPosition,
                        CategoryView, ReportData, and func GenerateHTMLReport with 11 parameters)
kalshi_API/pkg/models/          → infovore/internal/kalshi/models/
  models.go           — API response types (Market, Series, Settlement, Fill, MarketPosition, etc.)
```

Update all import paths from `github.com/bryanedds/kalshi-api/...` to `github.com/bryan-buckman/infovore/internal/kalshi/...`. (The Kalshi module is `github.com/bryanedds/kalshi-api`; the Infovore module is `github.com/bryan-buckman/infovore`.)

Add Kalshi's Go dependencies to `infovore/go.mod`. The Kalshi packages use only stdlib + `crypto` — no third-party deps. (`github.com/joho/godotenv` is only used in `cmd/kalshi/main.go` which is not being copied.)

**Import path updates** — files that contain `github.com/bryanedds/kalshi-api/` imports:
- `internal/kalshi/client/client.go` — imports `auth`, `config`, `models` (3 imports to update)
- `internal/kalshi/scanner/scanner.go` — imports `models` (1 import)
- `internal/kalshi/scanner/report.go` — imports `models` (1 import)
- `internal/kalshi/scanner/kelly_test.go` — no cross-package imports (just `testing`)
- `internal/kalshi/auth/signer_test.go` — no cross-package imports (just `testing`, `crypto`)
- `internal/kalshi/config/config.go` — no cross-package imports (just `os`, `fmt`, `strings`)

**Verification**: `go build ./...` should pass after import path updates.

### Step 2: Remove SQLite, make PostgreSQL mandatory

- Delete `internal/database/sqlite.go`
- Remove `modernc.org/sqlite` from `go.mod` and run `go mod tidy`
- In `main.go`:
  - Remove `dbPath := flag.String("db", ...)` flag
  - Remove the `strings.HasPrefix(*dbURL, "sqlite://")` branch (lines ~106-109)
  - Remove the `else` fallback `database.NewSQLite(*dbPath)` (lines ~113-116)
  - Remove the `database.NewSQLite` import usage
  - If `*dbURL` is empty after checking env: serve first-run setup page on all routes instead of `log.Fatalf`
- Keep `dbURL` flag (`-db-url`) and `DB_URL` env var as alternative input methods
- If no `DB_URL` is set, serve the first-run setup page on all routes (see First-Run Setup section above)

### Step 3: Add Kalshi database tables and store methods

**Update: `internal/database/postgres.go`**
- Add `kalshi_reports` and `kalshi_scan_log` tables to the migration
- Add new settings key defaults for Kalshi config
- Implement `SaveKalshiReport`, `GetLatestKalshiReport`, `AddKalshiScanLog`, `GetKalshiScanLog`

`SaveKalshiReport` should delete all previous reports and insert the new one in a single transaction:
```sql
DELETE FROM kalshi_reports;
INSERT INTO kalshi_reports (html) VALUES ($1);
```

`AddKalshiScanLog` should insert then purge old entries:
```sql
INSERT INTO kalshi_scan_log (output, error) VALUES ($1, $2);
DELETE FROM kalshi_scan_log WHERE id NOT IN (SELECT id FROM kalshi_scan_log ORDER BY created_at DESC LIMIT 50);
```

**Update: `internal/database/store.go`**
- Add new methods to the Store interface

### Step 4: Create the unified design system

**File: `internal/server/static/css/theme.css`** (new)

Contains all CSS custom properties for both light and dark modes as defined in the design system section above.

**Update: `internal/server/static/css/style.css`** (Infovore reader)
- Remove existing `:root { }` custom properties block
- Replace all hardcoded colors with `var(--token)` references

**Update: `internal/kalshi/scanner/report.go`** (Kalshi report template CSS)

**Note**: This file is ~1750 lines. The HTML/CSS/JS template is a raw string constant inside the Go file. There are approximately 80-100 hardcoded color values to replace with `var(--token)` references. Use search-and-replace systematically:
- `#2563eb` → `var(--primary)` (appears ~15 times)
- `#1d4ed8` → `var(--primary-dark)` (appears ~5 times)
- `#f8fafc` → `var(--bg)` (appears ~3 times)
- `#ffffff` / `white` → `var(--bg-card)` (appears ~10 times)
- `#1e293b` → `var(--text)` (appears ~8 times)
- `#64748b` → `var(--text-muted)` (appears ~10 times)
- `#e2e8f0` → `var(--border)` (appears ~8 times)
- `#16a34a` → `var(--success)` (appears ~5 times)
- `#dc2626` → `var(--danger)` (appears ~5 times)
- Other one-off values: map to nearest token or keep if unique to a specific component
- **Do not replace** colors inside the Kelly criterion dark panel (`.kelly-card`) — those keep their hardcoded gradient

Steps:
- Remove the existing `:root { }` custom properties block from the template
- Add `<link rel="stylesheet" href="/static/css/theme.css">` in `<head>`
- Replace all hardcoded colors with `var(--token)` references as listed above
- For standalone file output (if `--local` mode is re-added): embed the full theme CSS as an inline `<style>` block

### Step 5: Refactor Kalshi config and auth to read from database

The current constructor chain is:
```
config.Load()           → reads env vars → Config{APIKeyID, PrivateKeyPath, Environment, BaseURL}
client.New(cfg)         → auth.NewSigner(cfg.APIKeyID, cfg.PrivateKeyPath)
auth.NewSigner(id,path) → os.ReadFile(path) → pem.Decode → x509.ParsePKCS8/PKCS1 → *rsa.PrivateKey
```

For DB-stored credentials, the chain becomes:
```
config.LoadFromStore(store) → reads settings table → Config{APIKeyID, PrivateKeyData, Environment, BaseURL}
client.New(cfg)             → auth.NewSignerFromBytes(cfg.APIKeyID, cfg.PrivateKeyData)
auth.NewSignerFromBytes(id,pemBytes) → pem.Decode → x509.Parse → *rsa.PrivateKey
```

**Update: `internal/kalshi/config/config.go`**

Add a `PrivateKeyData []byte` field to `Config` (alongside `PrivateKeyPath`). One or the other is populated depending on the source:

```go
type Config struct {
    APIKeyID       string
    PrivateKeyPath string // set by Load() for --local/env-var mode
    PrivateKeyData []byte // set by LoadFromStore() for DB mode
    Environment    string
    BaseURL        string
}

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

    var baseURL string
    switch env {
    case "prod":
        baseURL = ProdBaseURL
    case "demo":
        baseURL = DemoBaseURL
    default:
        return nil, fmt.Errorf("invalid kalshi_environment: %s", env)
    }

    return &Config{
        APIKeyID:       keyID,
        PrivateKeyData: []byte(privateKey),
        Environment:    env,
        BaseURL:        baseURL,
    }, nil
}
```

Keep `Load()` unchanged for env-var/`--local` mode.

**Update: `internal/kalshi/auth/signer.go`**

Add `NewSignerFromBytes` that takes raw PEM bytes instead of a file path. Extract the PEM parsing logic into a shared helper:

```go
// NewSignerFromBytes creates a Signer from PEM-encoded RSA private key bytes.
// Used when the key is stored in the database rather than on disk.
func NewSignerFromBytes(keyID string, pemData []byte) (*Signer, error) {
    privateKey, err := parsePEMKey(pemData)
    if err != nil {
        return nil, err
    }
    return &Signer{keyID: keyID, privateKey: privateKey}, nil
}

// parsePEMKey extracts an RSA private key from PEM data (PKCS#8 or PKCS#1).
func parsePEMKey(pemData []byte) (*rsa.PrivateKey, error) {
    block, _ := pem.Decode(pemData)
    if block == nil {
        return nil, fmt.Errorf("failed to decode PEM block")
    }
    // Try PKCS#8 first, fall back to PKCS#1 (same logic as current NewSigner)
    ...
}
```

Refactor existing `NewSigner` to call `parsePEMKey` after reading the file.

**Update: `internal/kalshi/client/client.go`**

The current `client.New(cfg)` calls `auth.NewSigner(cfg.APIKeyID, cfg.PrivateKeyPath)`. Update to choose the constructor based on which field is populated:

```go
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
    // ... rest unchanged
}
```

This means `client.New` handles both modes — no need for a separate `NewFromConfig`.

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

**Current state of `GenerateHTMLReport`**: The function in `report.go` takes an `outputPath string` and writes directly to disk via `os.Create(outputPath)`. It has 11 parameters:

```go
func GenerateHTMLReport(
    markets []FilteredMarket,
    totalSeries int,
    minPrice, maxPrice float64,
    outputPath string,            // ← writes to file on disk
    balance *models.GetBalanceResponse,
    enrichedPositions []EnrichedPosition,
    settlements []models.Settlement,
    fills []models.Fill,
    marketTitles map[string]string,
    categoryViews []CategoryView,
) error
```

**Required change**: Add a companion function that returns HTML as a string instead of writing to disk:

```go
// GenerateHTMLReportString returns the report HTML as a string (for DB storage).
func GenerateHTMLReportString(
    markets []FilteredMarket,
    totalSeries int,
    minPrice, maxPrice float64,
    balance *models.GetBalanceResponse,
    enrichedPositions []EnrichedPosition,
    settlements []models.Settlement,
    fills []models.Fill,
    marketTitles map[string]string,
    categoryViews []CategoryView,
) (string, error) {
    // Same template execution as GenerateHTMLReport but renders to bytes.Buffer
    // Refactor: extract shared template logic into an internal helper
}
```

Keep the original `GenerateHTMLReport` for `--local` mode file output.

**File: `internal/kalshi/scanner/run.go`** (new)

This extracts the full scan orchestration from `cmd/kalshi/main.go` lines 84-287 into a reusable library function.

**Critical design constraint**: The scanner packages currently have zero dependency on the database layer — `scanner` only imports `models` + stdlib, `client` imports `auth` + `config` + `models`. This clean separation must be preserved. `run.go` does NOT import `database.Store`. Instead, it accepts pre-loaded config values and returns results; the caller (KalshiManager) handles all DB reads/writes.

```go
// ScanConfig holds everything needed to run a scan.
// All values are pre-extracted from the database by the caller.
type ScanConfig struct {
    // Kalshi credentials (extracted from DB settings by caller)
    APIKeyID       string
    PrivateKeyData []byte   // PEM content from settings table
    Environment    string   // "prod" or "demo"

    // Scan parameters
    Categories []string
    MinPrice   float64      // default 0.75
    MaxPrice   float64      // default 0.90

    // Output
    LogWriter  io.Writer    // for streaming progress (each fmt.Printf becomes fmt.Fprintf)
}

// ScanResult contains the outputs of a scan. The caller stores these in the DB.
type ScanResult struct {
    HTML string              // Complete HTML report
    Log  string              // Full scan log output
}

func RunScan(cfg ScanConfig) (*ScanResult, error) {
    // 1. Build config.Config from ScanConfig fields
    //    kalshiCfg := &config.Config{
    //        APIKeyID: cfg.APIKeyID, PrivateKeyData: cfg.PrivateKeyData,
    //        Environment: cfg.Environment, BaseURL: baseURLForEnv(cfg.Environment),
    //    }
    // 2. Create API client: client.New(kalshiCfg)
    // 3. Fetch allCategories via apiClient.GetCategories()
    // 4. Fetch portfolio: apiClient.GetBalance(), GetPositions(), enrich with GetMarket()
    // 5. For each category: GetSeriesList() → GetMarkets() → build []CategoryMarkets
    // 6. FilterByPriceRangeMulti(), SortByAskPrice()
    // 7. Fetch settlements + fills for current year
    // 8. Build CategoryViews
    // 9. html, err := GenerateHTMLReportString(...)
    // 10. Return &ScanResult{HTML: html, Log: logBuf.String()}, nil
    //
    // All fmt.Printf calls from main.go become fmt.Fprintf(cfg.LogWriter, ...)
    // Use a tee writer (io.MultiWriter) to also capture log into logBuf for the result.
}
```

**The caller (KalshiManager in `server/kalshi.go`) handles DB interaction:**
```go
func (km *KalshiManager) runScan() {
    // 1. Read settings from DB
    keyID, _ := km.store.GetSetting("kalshi_api_key_id")
    privateKey, _ := km.store.GetSetting("kalshi_private_key")
    env, _ := km.store.GetSetting("kalshi_environment")
    categories := parseCategories(km.store.GetSetting("kalshi_categories"))

    // 2. Call scanner (DB-agnostic)
    result, err := scanner.RunScan(scanner.ScanConfig{
        APIKeyID: keyID, PrivateKeyData: []byte(privateKey),
        Environment: env, Categories: categories,
        MinPrice: 0.75, MaxPrice: 0.90, LogWriter: &km.liveBuf,
    })

    // 3. Store results in DB
    if err == nil {
        km.store.SaveKalshiReport(result.HTML)
    }
    km.store.AddKalshiScanLog(result.Log, errString(err))
}
```

This preserves the scanner's clean dependency graph:
```
scanner → models + stdlib (no database import)
client  → auth + config + models + stdlib (no database import)
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

The existing Infovore server has these settings-related routes and handlers:
- `GET /api/settings` → `handleGetSettings` — returns `{polling_interval_minutes: N}`
- `POST /api/settings` → `handleSaveSettings` — saves polling interval to DB
- `GET /api/database-settings` → `handleGetDatabaseSettings` — returns DB type + masked URL
- `POST /api/database-settings` → `handleSaveDatabaseSettings` — writes DB_URL to `.env` file

Changes:
- Rewrite `handleGetSettings` to return ALL settings (polling, Kalshi, theme, DB) as one JSON object
- Rewrite `handleSaveSettings` to accept partial updates for any settings key
- Add `handleTestKalshi` endpoint (`POST /api/settings/test-kalshi`)
- Add `GET /settings` route to serve the settings page template
- **Delete** `handleGetDatabaseSettings` and `handleSaveDatabaseSettings` — their functionality is absorbed into the unified settings handlers
- **Delete** routes `GET/POST /api/database-settings` from the chi router

**Update: Kalshi report hamburger menu** in `report.go`
- Remove the inline category settings dialog
- "Settings" menu item now navigates to `/settings` (the full settings page)
- Keep "Refresh" as a direct action (POST `/api/kalshi/refresh`)

**Update: Infovore reader routes** in `server.go`
- Move `r.Get("/", s.handleHome)` → `r.Get("/reader", s.handleHome)`
- Move `r.Get("/feed/{feedID}", ...)` → `r.Get("/reader/feed/{feedID}", ...)`
- Move `r.Get("/folder/{folderID}", ...)` → `r.Get("/reader/folder/{folderID}", ...)`
- All `/api/*` routes stay unchanged (they're called by the reader's JS regardless of iframe context)
- Add `r.Get("/", s.handleShell)` to serve `shell.html`

**Update: Infovore reader** in `layout.html` / `app.js`
- Remove the gear icon settings modal
- Add hamburger menu (matching Kalshi's) with "Settings" linking to `/settings`
- Update all internal links from `/feed/` to `/reader/feed/`, `/folder/` to `/reader/folder/` (in `layout.html` and `app.js`)

### Step 9: Add the tabbed shell page

**File: `internal/server/templates/shell.html`** (new)

Tab bar with Reader and Markets tabs, plus the dark mode toggle (crescent moon / sun icon) in the upper right. Uses iframe for content isolation so each app's CSS/JS doesn't conflict.

**How the tabs work:**
- `GET /` serves `shell.html` — a minimal page with a tab bar and a single `<iframe>` element
- Tab bar has two buttons: "Reader" and "Markets", plus a hamburger menu (☰) and theme toggle (🌙/☀)
- Clicking "Reader" sets `iframe.src = "/reader"` — the full Infovore reader page (renders complete HTML via `layout.html`)
- Clicking "Markets" sets `iframe.src = "/markets"` — either the latest Kalshi report from DB or a placeholder/scanning page
- Default tab on load: "Reader"
- Active tab is tracked in URL hash (`/#reader`, `/#markets`) and `localStorage` for persistence across reloads
- In-iframe navigation (e.g., clicking a feed in the reader) stays within the iframe naturally

**Hamburger menu dropdown** (appears on click):
- "Settings" → navigates the parent page to `/settings` (full page, not in iframe)
- "Refresh Scan" → `POST /api/kalshi/refresh`, shows toast notification

**Route: `GET /markets`** — the KalshiManager handler:
```go
func (km *KalshiManager) HandleMarketsPage(w http.ResponseWriter, r *http.Request) {
    html, ts, err := km.store.GetLatestKalshiReport()
    if err != nil || html == "" {
        // Serve placeholder page (scan status + "Run Scan" button)
        // Similar to current cmd/serve placeholderPage but using theme tokens
        return
    }
    w.Header().Set("Content-Type", "text/html")
    w.Write([]byte(html))
}
```

**Theme propagation:**
- Parent shell sets `document.documentElement.dataset.theme` and saves to `localStorage`
- Parent posts `{type: 'theme-changed', value: 'dark'}` to `iframe.contentWindow`
- Each iframe page has a `window.addEventListener('message', ...)` that sets its own `data-theme`
- On iframe load, iframe reads `localStorage('theme')` to set initial theme (handles the case where iframe loads before parent posts)
- `localStorage` provides instant load; DB `settings.theme` provides cross-device persistence
- Theme toggle also does `POST /api/settings` with `{"theme": "dark"}` to persist to DB (fire-and-forget, non-blocking)

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
- [ ] Add `NewSignerFromBytes()` to `internal/kalshi/auth/signer.go`, refactor `NewSigner` to share `parsePEMKey` helper
- [ ] Update `internal/kalshi/client/client.go` `New()` to handle both `PrivateKeyPath` and `PrivateKeyData`
- [ ] Add `GenerateHTMLReportString()` to `internal/kalshi/scanner/report.go` (returns HTML string instead of writing to disk)
- [ ] Create `internal/kalshi/scanner/run.go` — extract scan orchestration from `cmd/kalshi/main.go`; accepts plain config values, returns `*ScanResult` (HTML + log); does NOT import database
- [ ] `KalshiManager.runScan()` in `kalshi.go` reads settings from DB → calls `scanner.RunScan()` → stores result in DB
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
- [ ] (Optional, low priority) Add inline theme.css fallback in `GenerateHTMLReport` for standalone file output
- [ ] Create `shell.html` template with tab bar, theme toggle, iframe
- [ ] Create `settings.html` template (full settings page)
- [ ] Create `settings.js` (settings page logic)
- [ ] Add FOUC-prevention inline script in shell.html `<head>`
- [ ] Add theme toggle JS with localStorage + DB persistence
- [ ] Add iframe theme propagation (postMessage)
- [ ] Remove Kalshi hamburger menu settings dialog (replace with link to /settings)
- [ ] Remove Infovore gear icon settings modal (replace with hamburger menu link)
- [ ] Update Infovore internal links: `/feed/` → `/reader/feed/`, `/folder/` → `/reader/folder/` (in `layout.html` and `app.js`)
- [ ] Update Kalshi report API paths to `/api/kalshi/*`
- [ ] Update Kalshi placeholder page API paths

### Infrastructure
- [ ] Update Dockerfile (single binary, no cron, no entrypoint, no SQLite)
- [ ] Remove `docker-entrypoint.sh`
- [ ] Update `.dockerignore`
- [ ] Create `docs/db.md` with Proxmox VM + PostgreSQL setup guide

### Automated Tests (Phase 1)
- [ ] Verify carried-over tests pass: `signer_test.go` (15 tests), `kelly_test.go` (10 tests)
- [ ] Write `NewSignerFromBytes` + `parsePEMKey` tests in `signer_test.go` (~6 tests)
- [ ] Write `LoadFromStore` tests in `config_test.go` with mock Store (~7 tests)
- [ ] Write `GenerateHTMLReportString` tests in `report_test.go` (~3 tests)
- [ ] Write `RunScan` config validation tests in `run_test.go` (~3 tests)
- [ ] Write Postgres store method tests in `postgres_test.go` (~5 tests, skipped without `TEST_DB_URL`)
- [ ] Write KalshiManager HTTP handler tests in `kalshi_test.go` (~4 tests)
- [ ] `go build ./... && go vet ./... && go test ./...` passes

### Automated Tests (Phase 2)
- [ ] Write settings API + route tests in `server_test.go` (~7 tests)
- [ ] (Optional) Write CSS token orphan detection test in `report_test.go`
- [ ] `go build ./... && go vet ./... && go test ./...` passes (all Phase 1 + Phase 2)

### Manual Verification

See the **Testing Plan** section below for the full breakdown of manual checklists per phase.

---

## Files Changed (Infovore)

| File | Change |
|------|--------|
| `main.go` | Remove SQLite fallback, require DB_URL, init KalshiManager |
| `go.mod` / `go.sum` | Remove SQLite deps, add Kalshi deps |
| `internal/database/sqlite.go` | DELETED |
| `internal/database/store.go` | Add Kalshi store methods to interface |
| `internal/database/postgres.go` | Add Kalshi tables, settings defaults, new methods |
| `internal/server/server.go` | Move `GET /` → `GET /reader`, `GET /feed/{id}` → `GET /reader/feed/{id}`, `GET /folder/{id}` → `GET /reader/folder/{id}`; add `GET /` (shell.html), `GET /settings`, `GET /markets`, `/api/kalshi/*` routes; rewrite settings handlers; delete database-settings handlers |
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
| `internal/kalshi/config/config_test.go` | NEW — `LoadFromStore` tests with mock Store |
| `internal/kalshi/auth/signer.go` | Add `NewSignerFromBytes()`, extract `parsePEMKey()` |
| `internal/kalshi/auth/signer_test.go` | Add `NewSignerFromBytes` + `parsePEMKey` tests |
| `internal/kalshi/scanner/run.go` | NEW — scan orchestration (DB-agnostic, accepts config values, returns HTML+log) |
| `internal/kalshi/scanner/run_test.go` | NEW — config validation tests |
| `internal/kalshi/scanner/report.go` | Replace hardcoded colors with tokens, update API paths, remove inline settings dialog |
| `internal/kalshi/scanner/report_test.go` | NEW — `GenerateHTMLReportString` output validation |
| `internal/database/postgres_test.go` | NEW — Store method tests (requires `TEST_DB_URL`) |
| `internal/server/kalshi_test.go` | NEW — KalshiManager HTTP handler tests |
| `internal/server/server_test.go` | NEW — Settings API, shell/reader/settings route tests |
| `Dockerfile` | Simplify: single binary, no cron/entrypoint/SQLite |
| `docs/db.md` | NEW — Proxmox VM sizing + PostgreSQL setup guide |

## Files to NOT Copy

- `kalshi_API/cmd/kalshi/main.go` — scan orchestration extracted into `scanner/run.go`; CLI flags not migrated (see note on `--local` mode below)
- `kalshi_API/cmd/serve/main.go` — logic absorbed into `server/kalshi.go`
- `kalshi_API/docker-entrypoint.sh` — no longer needed
- `kalshi_API/Dockerfile` — replaced by unified Dockerfile
- `kalshi_API/.devcontainer/` — use Infovore's devcontainer

## Note on `--local` Mode

The Kalshi scanner currently has a `--local` flag that writes a timestamped HTML report to a Windows desktop path and reads credentials from environment variables. This mode is useful for running the scanner as a standalone CLI without Docker or a database.

**In the merged binary**, `--local` mode is **not preserved as a flag**. The merged app is a webapp that reads credentials from PostgreSQL. If standalone CLI scanning is ever needed again, the user can:
1. Keep the original `kalshi_API` repo's binary for local use, or
2. Add a `scan` subcommand to the merged binary later (low priority, out of scope for this plan)

The `config.Load()` env-var-based function and `auth.NewSigner()` file-path-based constructor are kept in the codebase (not deleted) to minimize code churn and preserve the option, but no CLI entrypoint calls them in the merged binary.

For `--local` report generation (if re-added later): `GenerateHTMLReport(outputPath)` still writes to disk and embeds theme CSS inline as a `<style>` block (since there's no server to serve `theme.css`).

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

- ~30 files touched/created (including 8 test files)
- ~500 lines of new code (KalshiManager, settings page, theme system, DB methods, shell)
- ~400 lines of new tests (~35 new tests across 8 test files, plus ~25 carried over)
- ~150 lines of find-and-replace (hardcoded colors → token references)
- ~100 lines of deletions (SQLite code, old settings modals)
- ~50 lines of API path updates
- ~200 lines of documentation (docs/db.md)
- The Kalshi scanner logic and the Infovore reader logic are completely independent — they share only the HTTP server, the PostgreSQL database, and the design tokens

---

## Testing Plan

### Existing Tests (Carried Over from Kalshi)

These test files are copied into `internal/kalshi/` and must pass after import path updates:

| File | Tests | What it covers |
|------|-------|----------------|
| `internal/kalshi/auth/signer_test.go` | 15 | RSA key parsing (PKCS#1 + PKCS#8), PEM decode, file-not-found, invalid PEM, signing, signature verification, timestamp message format. Has `generateTestKey(t, pkcs8 bool)` helper. |
| `internal/kalshi/scanner/kelly_test.go` | 10 | FavoriteLongshotEdge (extreme/longshot/crossover/favorites), KalshiFee, CalculateKelly (positive/zero/negative edge), GenerateRecommendations, lerp. Pure math — no external deps. |

Note: `internal/kalshi/config/config_test.go` from the source repo (8 tests for env-var-based `Load()`) is NOT copied because `config.go` is modified to add `LoadFromStore()`. However, the existing `Load()` function is preserved, so these tests can optionally be carried over if `Load()` needs to remain tested.

**Infovore has zero existing test files.** All tests below are new.

### Phase 1 Automated Tests

These tests should be written during Phase 1 and must pass at the Phase 1 verification gate (`go test ./...`).

#### 1. `internal/kalshi/auth/signer_test.go` — Add `NewSignerFromBytes` tests

Reuse the existing `generateTestKey(t, pkcs8 bool)` helper. Instead of writing PEM to a temp file and calling `NewSigner(keyID, path)`, generate PEM bytes in memory and call `NewSignerFromBytes(keyID, pemBytes)`.

```go
// Tests to add (alongside existing tests):
func TestNewSignerFromBytes_PKCS1(t *testing.T)          // generate PKCS#1 PEM → NewSignerFromBytes → sign + verify
func TestNewSignerFromBytes_PKCS8(t *testing.T)          // generate PKCS#8 PEM → NewSignerFromBytes → sign + verify
func TestNewSignerFromBytes_InvalidPEM(t *testing.T)     // garbage bytes → expect error
func TestNewSignerFromBytes_EmptyPEM(t *testing.T)       // nil/empty → expect error
func TestNewSignerFromBytes_ECKey(t *testing.T)          // EC PEM → expect "not an RSA key" error
func TestParsePEMKey_SharedLogic(t *testing.T)           // verify parsePEMKey works identically for both constructors
```

Pattern: generate a test key, encode to PEM bytes, call `NewSignerFromBytes`, sign a test message, verify with the public key. Same assertions as existing `TestNewSigner_*` tests but without the filesystem.

#### 2. `internal/kalshi/config/config_test.go` — New tests for `LoadFromStore`

This file needs new tests for the DB-backed config loader. Use a mock Store interface.

```go
// mockStore implements database.Store for testing
type mockStore struct {
    settings map[string]string
}
func (m *mockStore) GetSetting(key string) (string, error) {
    return m.settings[key], nil
}
// ... other Store methods return zero values

func TestLoadFromStore_Success(t *testing.T)              // all settings present → valid Config with PrivateKeyData
func TestLoadFromStore_MissingAPIKey(t *testing.T)        // no kalshi_api_key_id → error
func TestLoadFromStore_MissingPrivateKey(t *testing.T)    // no kalshi_private_key → error
func TestLoadFromStore_DefaultEnvProd(t *testing.T)       // no kalshi_environment → defaults to "prod"
func TestLoadFromStore_DemoEnv(t *testing.T)              // kalshi_environment=demo → DemoBaseURL
func TestLoadFromStore_InvalidEnv(t *testing.T)           // kalshi_environment=invalid → error
func TestLoadFromStore_PrivateKeyDataPopulated(t *testing.T) // verify PrivateKeyData is []byte of the PEM string
```

Note: `LoadFromStore` imports `database.Store`, so this test file needs to import the database package. This is fine — the config package gains a database dependency for `LoadFromStore()` only. The scanner and client packages remain database-free.

#### 3. `internal/kalshi/scanner/report_test.go` — New test for `GenerateHTMLReportString`

```go
func TestGenerateHTMLReportString_BasicOutput(t *testing.T) {
    // Construct minimal synthetic data:
    markets := []FilteredMarket{{
        Market:   models.Market{Ticker: "TEST-MARKET", Title: "Test Market", /* ... */},
        Side:     Yes,
        AskPrice: 0.80,
        Category: "Politics",
    }}
    categoryViews := []CategoryView{{Name: "Politics", Active: true}}

    html, err := GenerateHTMLReportString(
        markets, 1, 0.75, 0.90,
        &models.GetBalanceResponse{Balance: 1000},
        nil, nil, nil, nil, categoryViews,
    )

    if err != nil { t.Fatalf("unexpected error: %v", err) }
    if html == "" { t.Fatal("expected non-empty HTML") }
    // Verify key structural elements
    if !strings.Contains(html, "<html") { t.Error("missing <html> tag") }
    if !strings.Contains(html, "TEST-MARKET") { t.Error("market ticker not in output") }
    if !strings.Contains(html, "Politics") { t.Error("category not in output") }
}

func TestGenerateHTMLReportString_EmptyMarkets(t *testing.T) {
    // Zero markets should still produce valid HTML (with "no markets" message or empty table)
    html, err := GenerateHTMLReportString(nil, 0, 0.75, 0.90, nil, nil, nil, nil, nil, nil)
    if err != nil { t.Fatalf("unexpected error: %v", err) }
    if !strings.Contains(html, "<html") { t.Error("missing <html> tag") }
}

func TestGenerateHTMLReportString_MatchesDiskOutput(t *testing.T) {
    // Generate via both paths with same data, compare HTML output
    // This validates that GenerateHTMLReportString produces identical output to GenerateHTMLReport
    tmpDir := t.TempDir()
    outputPath := filepath.Join(tmpDir, "test_report.html")
    // ... call GenerateHTMLReport(outputPath, ...) and GenerateHTMLReportString(...)
    // ... read file, compare strings
}
```

#### 4. `internal/kalshi/scanner/run_test.go` — RunScan config validation

```go
func TestRunScan_MissingAPIKey(t *testing.T) {
    _, err := RunScan(ScanConfig{PrivateKeyData: []byte("x"), Environment: "prod"})
    if err == nil { t.Fatal("expected error for missing API key") }
}

func TestRunScan_MissingPrivateKey(t *testing.T) {
    _, err := RunScan(ScanConfig{APIKeyID: "test", Environment: "prod"})
    if err == nil { t.Fatal("expected error for missing private key") }
}

func TestRunScan_InvalidEnvironment(t *testing.T) {
    _, err := RunScan(ScanConfig{APIKeyID: "test", PrivateKeyData: []byte("x"), Environment: "invalid"})
    if err == nil { t.Fatal("expected error for invalid environment") }
}
```

Note: Testing a full scan requires live Kalshi API credentials. These tests only validate the early config-validation path. Full integration testing is manual (see below).

#### 5. `internal/database/postgres_test.go` — Store method tests

These tests require a PostgreSQL connection. Use the `DB_URL` env var (or skip if not set).

```go
func setupTestDB(t *testing.T) *PostgresStore {
    dbURL := os.Getenv("TEST_DB_URL")
    if dbURL == "" {
        t.Skip("TEST_DB_URL not set, skipping Postgres tests")
    }
    store, err := NewPostgres(dbURL)
    if err != nil { t.Fatalf("failed to connect: %v", err) }
    t.Cleanup(func() {
        // Clean up test data
        store.db.Exec("DELETE FROM kalshi_reports")
        store.db.Exec("DELETE FROM kalshi_scan_log")
        store.Close()
    })
    return store
}

func TestSaveAndGetKalshiReport(t *testing.T) {
    store := setupTestDB(t)
    // Save a report
    err := store.SaveKalshiReport("<html>test report</html>")
    if err != nil { t.Fatalf("SaveKalshiReport: %v", err) }
    // Retrieve it
    html, ts, err := store.GetLatestKalshiReport()
    if err != nil { t.Fatalf("GetLatestKalshiReport: %v", err) }
    if html != "<html>test report</html>" { t.Errorf("got %q", html) }
    if ts.IsZero() { t.Error("expected non-zero timestamp") }
}

func TestSaveKalshiReport_ReplacesOld(t *testing.T) {
    store := setupTestDB(t)
    store.SaveKalshiReport("report 1")
    store.SaveKalshiReport("report 2")
    html, _, _ := store.GetLatestKalshiReport()
    if html != "report 2" { t.Errorf("expected latest report, got %q", html) }
    // Verify only one row exists
    var count int
    store.db.QueryRow("SELECT COUNT(*) FROM kalshi_reports").Scan(&count)
    if count != 1 { t.Errorf("expected 1 row, got %d", count) }
}

func TestGetLatestKalshiReport_Empty(t *testing.T) {
    store := setupTestDB(t)
    html, _, err := store.GetLatestKalshiReport()
    if err != nil { t.Fatalf("unexpected error: %v", err) }
    if html != "" { t.Errorf("expected empty, got %q", html) }
}

func TestAddAndGetKalshiScanLog(t *testing.T) {
    store := setupTestDB(t)
    store.AddKalshiScanLog("scan output 1", "")
    store.AddKalshiScanLog("scan output 2", "some error")
    logs, err := store.GetKalshiScanLog(10)
    if err != nil { t.Fatalf("GetKalshiScanLog: %v", err) }
    if len(logs) != 2 { t.Fatalf("expected 2 logs, got %d", len(logs)) }
    // Most recent first
    if logs[0].Output != "scan output 2" { t.Error("wrong order") }
    if logs[0].Error != "some error" { t.Error("error not stored") }
}

func TestAddKalshiScanLog_PurgesOldEntries(t *testing.T) {
    store := setupTestDB(t)
    for i := 0; i < 55; i++ {
        store.AddKalshiScanLog(fmt.Sprintf("log %d", i), "")
    }
    logs, _ := store.GetKalshiScanLog(100)
    if len(logs) > 50 { t.Errorf("expected <=50 after purge, got %d", len(logs)) }
}
```

#### 6. `internal/server/kalshi_test.go` — HTTP handler tests

Use `net/http/httptest` to test KalshiManager handlers without a running server.

```go
func TestHandleMarketsPage_NoReport(t *testing.T) {
    store := newMockStore()  // mock with no reports
    km := NewKalshiManager(store, nil)
    req := httptest.NewRequest("GET", "/markets", nil)
    w := httptest.NewRecorder()
    km.HandleMarketsPage(w, req)
    // Should return placeholder page
    if w.Code != 200 { t.Errorf("status %d", w.Code) }
    if !strings.Contains(w.Body.String(), "No report") { t.Error("expected placeholder") }
}

func TestHandleMarketsPage_WithReport(t *testing.T) {
    store := newMockStore()
    store.reports = "<html>test</html>"
    km := NewKalshiManager(store, nil)
    req := httptest.NewRequest("GET", "/markets", nil)
    w := httptest.NewRecorder()
    km.HandleMarketsPage(w, req)
    if w.Code != 200 { t.Errorf("status %d", w.Code) }
    if w.Body.String() != "<html>test</html>" { t.Error("report not served") }
}

func TestHandleStatus_Idle(t *testing.T) {
    km := NewKalshiManager(newMockStore(), nil)
    req := httptest.NewRequest("GET", "/api/kalshi/status", nil)
    w := httptest.NewRecorder()
    km.HandleStatus(w, req)
    var status map[string]interface{}
    json.NewDecoder(w.Body).Decode(&status)
    if status["scanning"].(bool) { t.Error("should not be scanning") }
}

func TestHandleRefresh_MethodNotAllowed(t *testing.T) {
    km := NewKalshiManager(newMockStore(), nil)
    req := httptest.NewRequest("GET", "/api/kalshi/refresh", nil)
    w := httptest.NewRecorder()
    km.HandleRefresh(w, req)
    if w.Code != 405 { t.Errorf("expected 405, got %d", w.Code) }
}
```

### Phase 1 Verification Gate

All of the following must pass:

```bash
go build ./...    # compiles cleanly
go vet ./...      # no issues
go test ./...     # all automated tests pass
```

Then manual testing with a running PostgreSQL:
- [ ] `GET /` → existing Infovore reader works unchanged
- [ ] `GET /markets` → placeholder page (no report yet)
- [ ] Configure Kalshi credentials directly in DB settings table
- [ ] `POST /api/kalshi/refresh` → triggers scan
- [ ] `GET /api/kalshi/status` → shows scan progress / completion
- [ ] `GET /api/kalshi/log` → shows log entries from DB
- [ ] After scan completes: `GET /markets` → serves the generated report HTML
- [ ] Background scheduler goroutine triggers on configured interval (verify via log)
- [ ] Docker build succeeds: `docker build -t infovore .`
- [ ] Docker run works: `docker run -e DB_URL=... infovore`

### Phase 2 Automated Tests

#### 7. `internal/server/server_test.go` — Settings API + route tests

```go
func TestGetSettings_ReturnsAllKeys(t *testing.T) {
    // GET /api/settings should return polling_interval, kalshi_categories,
    // kalshi_api_key_id, kalshi_environment, kalshi_private_key_configured (bool), theme
    s := setupTestServer(t)
    req := httptest.NewRequest("GET", "/api/settings", nil)
    w := httptest.NewRecorder()
    s.router.ServeHTTP(w, req)
    var settings map[string]interface{}
    json.NewDecoder(w.Body).Decode(&settings)
    // kalshi_private_key should NOT be in the response
    if _, ok := settings["kalshi_private_key"]; ok {
        t.Error("private key should never be returned in GET")
    }
    // kalshi_private_key_configured should be a boolean
    if _, ok := settings["kalshi_private_key_configured"]; !ok {
        t.Error("missing kalshi_private_key_configured field")
    }
}

func TestPostSettings_PartialUpdate(t *testing.T) {
    s := setupTestServer(t)
    body := strings.NewReader(`{"kalshi_categories": ["Politics", "Sports"]}`)
    req := httptest.NewRequest("POST", "/api/settings", body)
    req.Header.Set("Content-Type", "application/json")
    w := httptest.NewRecorder()
    s.router.ServeHTTP(w, req)
    if w.Code != 200 { t.Errorf("status %d", w.Code) }
    // Verify only kalshi_categories changed, other settings untouched
}

func TestPostSettings_PrivateKeyNotReturnedInResponse(t *testing.T) {
    s := setupTestServer(t)
    body := strings.NewReader(`{"kalshi_private_key": "-----BEGIN RSA PRIVATE KEY-----\ntest\n-----END RSA PRIVATE KEY-----"}`)
    req := httptest.NewRequest("POST", "/api/settings", body)
    req.Header.Set("Content-Type", "application/json")
    w := httptest.NewRecorder()
    s.router.ServeHTTP(w, req)
    // Response should confirm save but not echo the key back
    if strings.Contains(w.Body.String(), "BEGIN RSA") {
        t.Error("private key leaked in POST response")
    }
}

func TestShellRoute(t *testing.T) {
    s := setupTestServer(t)
    req := httptest.NewRequest("GET", "/", nil)
    w := httptest.NewRecorder()
    s.router.ServeHTTP(w, req)
    if w.Code != 200 { t.Errorf("status %d", w.Code) }
    if !strings.Contains(w.Body.String(), "iframe") { t.Error("shell page should contain iframe") }
}

func TestReaderRoute(t *testing.T) {
    s := setupTestServer(t)
    req := httptest.NewRequest("GET", "/reader", nil)
    w := httptest.NewRecorder()
    s.router.ServeHTTP(w, req)
    if w.Code != 200 { t.Errorf("status %d", w.Code) }
}

func TestSettingsRoute(t *testing.T) {
    s := setupTestServer(t)
    req := httptest.NewRequest("GET", "/settings", nil)
    w := httptest.NewRecorder()
    s.router.ServeHTTP(w, req)
    if w.Code != 200 { t.Errorf("status %d", w.Code) }
}

func TestOldReaderRoutesRemoved(t *testing.T) {
    // After Phase 2, /feed/{id} should 404 (moved to /reader/feed/{id})
    s := setupTestServer(t)
    req := httptest.NewRequest("GET", "/feed/1", nil)
    w := httptest.NewRecorder()
    s.router.ServeHTTP(w, req)
    if w.Code != 404 { t.Errorf("old /feed/ route should 404, got %d", w.Code) }
}
```

#### 8. CSS token coverage test (optional but recommended)

```go
func TestReportCSS_NoOrphanedHardcodedColors(t *testing.T) {
    // Generate a report and scan the HTML for common hardcoded colors
    // that should have been replaced with var(--token) references.
    html, _ := GenerateHTMLReportString(/* minimal data */)

    orphans := []string{
        "#2563eb", "#1d4ed8", "#f8fafc", "#1e293b",
        "#64748b", "#e2e8f0", "#16a34a", "#dc2626",
    }
    for _, color := range orphans {
        // Count occurrences outside of the Kelly criterion dark panel
        // (Kelly panel keeps hardcoded colors by design)
        if strings.Count(html, color) > 0 {
            // Extract surrounding context to check if it's in the Kelly panel
            // This is a rough check — visual review is still needed
            t.Logf("WARNING: found %s in report HTML (may need token replacement)", color)
        }
    }
}
```

### Phase 2 Verification Gate

Automated:
```bash
go build ./...    # compiles
go vet ./...      # no issues
go test ./...     # all Phase 1 + Phase 2 tests pass
```

Manual:
- [ ] `GET /` → tabbed shell loads, Reader tab active by default
- [ ] Click "Markets" tab → shows Kalshi report in iframe
- [ ] Click back to "Reader" tab → reader loads in iframe
- [ ] Tab state persists across page reload (via localStorage / URL hash)
- [ ] Theme toggle (light → dark) → both shell and iframe content switch
- [ ] Theme persists across tab switches
- [ ] Theme persists across page reload
- [ ] Hamburger → Settings → full settings page loads
- [ ] Settings page: change polling interval → save → verify via GET /api/settings
- [ ] Settings page: enter Kalshi API key + private key → save
- [ ] Settings page: "Test Connection" button → shows success/failure
- [ ] Settings page: toggle categories → save → verify via GET /api/settings
- [ ] Settings page: private key textarea never shows the actual stored key
- [ ] RSS reader works at `/reader` in light mode
- [ ] RSS reader works at `/reader` in dark mode
- [ ] Kalshi report works at `/markets` in light mode
- [ ] Kalshi report works at `/markets` in dark mode
- [ ] First-run setup page works (no DB_URL → setup → connect → redirect to shell)
- [ ] Docker build and run with PostgreSQL end-to-end

### What Cannot Be Tested Without Live Credentials

The following require a real Kalshi API key + private key and cannot be automated in CI:
- Full scan execution (API calls to Kalshi)
- Settings page "Test Connection" with real credentials
- Report content accuracy (market data correctness)
- Background scheduler producing real reports

These are covered by the manual verification checklists above.

### Test File Summary

| File (all under `infovore/`) | Phase | Tests | Notes |
|------------------------------|-------|-------|-------|
| `internal/kalshi/auth/signer_test.go` | 1 | 15 existing + 6 new | Add `NewSignerFromBytes` + `parsePEMKey` tests |
| `internal/kalshi/scanner/kelly_test.go` | 1 | 10 existing | Carried over unchanged |
| `internal/kalshi/config/config_test.go` | 1 | ~7 new | `LoadFromStore` with mock Store |
| `internal/kalshi/scanner/report_test.go` | 1 | ~3 new | `GenerateHTMLReportString` output validation |
| `internal/kalshi/scanner/run_test.go` | 1 | ~3 new | `RunScan` config validation (early-exit errors) |
| `internal/database/postgres_test.go` | 1 | ~5 new | Store methods (requires `TEST_DB_URL`, skipped otherwise) |
| `internal/server/kalshi_test.go` | 1 | ~4 new | KalshiManager HTTP handlers via httptest |
| `internal/server/server_test.go` | 2 | ~7 new | Settings API, shell/reader/settings routes |

**Total: ~25 existing tests carried over + ~35 new tests written across both phases.**

---

## Execution Phases

This plan is too large for a single Claude session (~22 files, ~1000 lines of new/changed code). Split into two phases with a clear verification gate between them.

### Phase 1: Backend + Kalshi Integration

**Goal**: Kalshi scanning works end-to-end through the Infovore binary — scan triggers via API, reports stored in PostgreSQL, served at `/markets`. Existing Infovore reader is **completely unchanged** (still at `/`, settings still work as before). No visual changes.

**Steps**: 1, 2, 3, 5, 7 (run.go + GenerateHTMLReportString), 6 (KalshiManager + routes), 10 (scheduler), 11 (Dockerfile), 12 (CLI flags), 13 (docs/db.md)

**Files touched** (~20):
| File | Change |
|------|--------|
| `internal/kalshi/*` | NEW — entire directory copied from kalshi_API, imports updated |
| `internal/kalshi/config/config.go` | Add `LoadFromStore()`, add `PrivateKeyData` field |
| `internal/kalshi/config/config_test.go` | NEW — `LoadFromStore` tests with mock Store (~7 tests) |
| `internal/kalshi/auth/signer.go` | Add `NewSignerFromBytes()`, extract `parsePEMKey()` |
| `internal/kalshi/auth/signer_test.go` | Add `NewSignerFromBytes` + `parsePEMKey` tests (~6 new) |
| `internal/kalshi/client/client.go` | Update `New()` to handle `PrivateKeyData` |
| `internal/kalshi/scanner/run.go` | NEW — `RunScan()` extracted from cmd/kalshi/main.go |
| `internal/kalshi/scanner/run_test.go` | NEW — config validation tests (~3 tests) |
| `internal/kalshi/scanner/report.go` | Add `GenerateHTMLReportString()` |
| `internal/kalshi/scanner/report_test.go` | NEW — report output validation (~3 tests) |
| `internal/database/store.go` | Add Kalshi store methods + `ScanLogEntry` type |
| `internal/database/postgres.go` | Add Kalshi tables, new methods, settings defaults |
| `internal/database/postgres_test.go` | NEW — store method tests, requires `TEST_DB_URL` (~5 tests) |
| `internal/database/sqlite.go` | DELETED |
| `internal/server/server.go` | Register `/api/kalshi/*` and `/markets` routes (reader routes unchanged) |
| `internal/server/kalshi.go` | NEW — KalshiManager, handlers, scheduler |
| `internal/server/kalshi_test.go` | NEW — HTTP handler tests via httptest (~4 tests) |
| `main.go` | Remove SQLite fallback, require DB_URL, init KalshiManager |
| `go.mod` / `go.sum` | Remove SQLite deps |
| `Dockerfile` | Simplify to single binary |
| `docs/db.md` | NEW — Proxmox VM + PostgreSQL setup |

**Verification gate**: See **Testing Plan → Phase 1 Verification Gate** above for the full automated + manual checklist.

### Phase 2: UI Unification

**Goal**: Tabbed shell, unified theme system (light/dark), unified Settings page. Reader moves to `/reader`, shell serves at `/`.

**Steps**: 4 (theme.css), 8 (settings page + route reorg), 9 (shell.html + tabs), plus report.go CSS token replacement

**Files touched** (~13):
| File | Change |
|------|--------|
| `internal/server/static/css/theme.css` | NEW — design tokens (light + dark) |
| `internal/server/static/css/style.css` | Replace hardcoded colors → tokens |
| `internal/kalshi/scanner/report.go` | Replace ~100 hardcoded colors → tokens, remove inline settings dialog, update API paths to `/api/kalshi/*`, add `<link>` to theme.css |
| `internal/server/templates/shell.html` | NEW — tabbed shell with iframe, theme toggle |
| `internal/server/templates/settings.html` | NEW — unified settings page |
| `internal/server/static/js/settings.js` | NEW — settings page logic |
| `internal/server/templates/layout.html` | Add theme.css link, add hamburger menu, remove gear modal |
| `internal/server/static/js/app.js` | Remove settings modal, update links to `/reader/*` |
| `internal/server/server.go` | Move reader routes to `/reader`, add `/` (shell), `/settings`; rewrite settings handlers; delete database-settings handlers |
| `internal/server/server_test.go` | NEW — settings API + route tests (~7 tests) |
| `main.go` | Add first-run setup page logic (no DB_URL → setup) |

**Verification gate**: See **Testing Plan → Phase 2 Verification Gate** above for the full automated + manual checklist.

### Why This Split Works

Phase 1 is **purely additive** — it adds Kalshi functionality alongside the existing Infovore reader without changing any existing routes, templates, CSS, or JS. If Phase 2 is never started, Infovore still works exactly as before, plus there's a Kalshi scanner available at `/markets`.

Phase 2 is a **visual refactor** — it changes the URL structure, adds the shell/tabs/theme, and unifies settings. It builds on the Phase 1 foundation but doesn't need any new backend logic.

Each phase fits within a Claude Pro 5-hour session. Phase 1 is ~20 files (including tests) with more Go backend work. Phase 2 is ~13 files with more HTML/CSS/JS frontend work.
