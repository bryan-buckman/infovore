================================================================================
                              I N F O V O R E
                    RSS Reader  +  Kalshi Market Scanner
================================================================================

Infovore is a self-hosted web application written in Go that combines two
tools in a single tabbed interface:

  1. RSS Reader    - Subscribe, organize, and read RSS/Atom feeds
  2. Market Scanner - Scan Kalshi prediction markets for underpriced contracts
                     using the Kelly Criterion

--------------------------------------------------------------------------------
 QUICK START
--------------------------------------------------------------------------------

  Prerequisites:
    - Go 1.22+
    - PostgreSQL 14+ (running and accessible)

  1. Clone the repo:
       git clone https://github.com/bryan-buckman/infovore.git
       cd infovore

  2. Create a PostgreSQL database:
       createdb infovore

  3. Set the database connection URL:
       export DB_URL="postgres://user:pass@localhost:5432/infovore?sslmode=disable"

  4. Run the application:
       go run .

  5. Open in browser:
       http://localhost:8080

  Tables are created automatically on first run.

--------------------------------------------------------------------------------
 DOCKER
--------------------------------------------------------------------------------

  Build:
    docker build -t infovore .

  Run:
    docker run -p 8080:8080 \
      -e DB_URL="postgres://user:pass@host:5432/infovore?sslmode=disable" \
      infovore

--------------------------------------------------------------------------------
 COMMAND-LINE FLAGS
--------------------------------------------------------------------------------

  -addr       HTTP listen address (default ":8080")
  -db-url     PostgreSQL connection URL  (or set DB_URL env var)
  -data-dir   Directory for the .env file (default "/data" or ".")

--------------------------------------------------------------------------------
 CONFIGURATION
--------------------------------------------------------------------------------

  Database URL:
    Set via DB_URL environment variable, .env file, or -db-url flag.
    PostgreSQL is required; SQLite is not supported.

  Kalshi API:
    After first boot, go to Settings in the web UI and enter:
      - Kalshi API Key ID
      - Kalshi Private Key (PEM format)
      - Environment (prod / demo)
    The scanner runs automatically on a configurable interval (default 6h).

  RSS Feeds:
    Add feeds manually via the UI, or import an OPML file from Settings.

--------------------------------------------------------------------------------
 DOCUMENTATION
--------------------------------------------------------------------------------

  docs/
    architecture.md   - System architecture and package layout
    api.md            - HTTP API reference (all endpoints)
    getting-started.md - Detailed setup and usage guide
    db.md             - Database schema and settings reference
    data-flows.md     - Data flow diagrams for RSS and Kalshi pipelines

--------------------------------------------------------------------------------
 PROJECT STRUCTURE
--------------------------------------------------------------------------------

  infovore/
  |-- main.go                   Entry point, .env loading, DB connection
  |-- Dockerfile                Multi-stage Docker build
  |-- Makefile                  Build shortcuts
  |-- go.mod / go.sum           Go module dependencies
  |-- docs/                     Documentation
  |-- internal/
  |   |-- model/                Data models (Folder, Feed, Item)
  |   |-- database/             Store interface + PostgreSQL implementation
  |   |-- rss/                  RSS fetching, parsing, polling
  |   |-- opml/                 OPML import/export
  |   |-- server/               HTTP server, routes, templates, static assets
  |   |   |-- kalshi.go         KalshiManager (scheduler + handlers)
  |   |   |-- templates/        HTML templates (shell, layout, settings)
  |   |   +-- static/           CSS and JavaScript
  |   +-- kalshi/               Kalshi prediction market integration
  |       |-- auth/             RSA-PSS request signing
  |       |-- client/           API client (rate-limited)
  |       |-- config/           Configuration loading (env + DB)
  |       |-- models/           Market, Portfolio, Series, Transaction types
  |       +-- scanner/          Market filtering, Kelly Criterion, HTML reports

--------------------------------------------------------------------------------
 LICENSE
--------------------------------------------------------------------------------

  MIT License - see LICENSE file for details.

================================================================================
