# Infovore - Project Instructions

## Project Overview

Infovore is a self-hosted Go 1.22 monolith combining an **RSS reader** and a **Kalshi prediction market scanner** in a single tabbed web UI. PostgreSQL is the only supported database (SQLite was removed).

## Tech Stack

| Component   | Technology                                          |
| ----------- | --------------------------------------------------- |
| Language    | Go 1.22                                             |
| HTTP Router | chi/v5                                              |
| Database    | PostgreSQL via lib/pq (Store interface in `internal/database/store.go`) |
| RSS Parsing | gofeed                                              |
| Frontend    | Vanilla HTML/CSS/JS, Go html/template (embedded via go:embed) |
| Container   | Multi-stage Docker (golang:1.22 -> debian:bookworm-slim) |
| Deployment  | Docker -> FluxCD -> Kubernetes on Proxmox VMs       |

## Read on Session Start

Before making changes, read:
- `docs/architecture.md` — package layout and key design decisions
- `Dockerfile` — current deployment config

## Key Architecture

- **Single binary**: all templates, CSS, JS embedded at compile time
- **Database**: `internal/database/store.go` defines the `Store` interface (30+ methods); sole implementation is `PostgresStore` in `postgres.go`
- **RSS fetching**: parallel worker pool (8 workers), domain-level rate limiting (2 concurrent per domain + 500ms delay)
- **Kalshi scanner**: background goroutine checks every 5 min if a scan is due; RSA-PSS auth, 20 req/sec rate limit
- **Config precedence**: CLI flags > environment variables > `.env` file (at project root or `/data/.env`)

## Deployment Context

- In production, `DB_URL` comes from a Kubernetes Secret mounted at `/data/.env` — there are no CLI flags in prod
- PostgreSQL 15+ requires `GRANT CREATE ON SCHEMA public` for the app user (common gotcha)
- App connection pool: 25 max open connections
- The `.env` file is in `.gitignore`; sensitive config never goes in the repo

## After Code Changes Checklist

1. Verify the `Dockerfile` still accurately reflects the build (user has asked this across multiple sessions)
2. Verify `.env` precedence is still respected (env vars set before process start must not be overridden by `loadEnvFile`)
3. RSS domain-level rate limiting must remain intact — do not remove or weaken it
4. Run: `go vet ./... && go build ./...`

## User Preferences

- Gives feature requests in numbered batches (3-5 at a time) — work through them sequentially
- Has intermediate Go experience — do not start tutorials at beginner level
- Prefers analysis before changes (e.g., connection count estimates, VM sizing, performance comparisons)
- This app runs locally/internally — security is not a major concern but don't introduce obvious vulnerabilities
- When the user says "review this codebase", read the claude.md files and key source files before responding

## Documentation

Detailed docs live in `docs/`:
- `architecture.md` — system architecture and package layout
- `api.md` — full HTTP API reference
- `data-flows.md` — mermaid sequence diagrams for all major flows
- `db.md` — schema, Kalshi settings, and Proxmox VM sizing recommendations
- `getting-started.md` — setup, usage, Docker, and Kubernetes deployment

