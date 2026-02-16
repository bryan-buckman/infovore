# Data Flows

This document describes the major data flows through the Infovore application.

## 1. Application Startup

```mermaid
sequenceDiagram
    participant M as main.go
    participant E as .env File
    participant DB as PostgreSQL
    participant S as Server
    participant K as KalshiManager

    M->>E: loadEnvFile()
    M->>M: Parse flags (-addr, -db-url, -data-dir)
    M->>DB: database.NewPostgres(dbURL)
    DB->>DB: migrate() — create tables + seed settings
    M->>S: server.New(db)
    S->>K: NewKalshiManager(db)
    K->>K: go scheduler() (background goroutine)
    S->>S: setupRoutes()
    M->>S: srv.Start(":8080")
```

**Key points:**
- The `.env` file is loaded first, but environment variables already set take precedence
- PostgreSQL tables are created automatically via the `migrate()` function
- The Kalshi scheduler starts immediately but waits 30 seconds before its first check
- Graceful shutdown is handled via `SIGINT`/`SIGTERM` → `srv.Stop()`

---

## 2. RSS Feed Refresh

```mermaid
sequenceDiagram
    participant B as Browser
    participant S as Server
    participant F as Fetcher
    participant R as RSS Source
    participant DB as PostgreSQL

    B->>S: POST /api/refresh
    S->>F: FetchAll(ctx)
    F->>DB: GetAllFeeds()
    DB-->>F: []Feed

    loop For each feed (parallel workers)
        F->>F: domainLimiter.acquire(domain)
        F->>R: HTTP GET feed URL
        R-->>F: XML response
        F->>F: gofeed.Parse(response)
        loop For each item in feed
            F->>DB: AddItem(item)
            DB-->>F: (id, isNew, err)
        end
        F->>DB: UpdateFeedLastFetched(feedID, now)
        F->>DB: UpdateFeedTitle(feedID, title)
        F->>F: domainLimiter.release(domain)
    end

    F-->>S: map[feedID]newItemCount
    S-->>B: JSON { "new_items": {...} }
```

**Key points:**
- Feeds are fetched in parallel using a worker pool (8 workers for PostgreSQL)
- Domain-level rate limiting prevents overwhelming any single host (2 concurrent + 500ms delay)
- Items are de-duplicated by GUID — `INSERT ... ON CONFLICT DO NOTHING`
- Feed titles are updated from the feed's metadata on each refresh
- Errors are stored per-feed via `UpdateFeedError()` for UI display

---

## 3. RSS Item Reading

```mermaid
sequenceDiagram
    participant B as Browser
    participant S as Server
    participant DB as PostgreSQL

    B->>S: GET /reader/feed/{feedID}
    S->>DB: GetFeedByID(feedID)
    S->>DB: GetItems(feedID, onlyUnread=false)
    S->>DB: GetFoldersWithFeeds()
    S->>DB: GetUnfiledFeeds()
    S-->>B: Rendered HTML (layout.html)

    B->>S: POST /api/mark-read { item_ids: [1,2,3] }
    S->>DB: MarkItemsRead([1,2,3])
    S-->>B: { "status": "ok" }
```

---

## 4. Kalshi Market Scan

```mermaid
sequenceDiagram
    participant T as Timer/API
    participant K as KalshiManager
    participant C as config.LoadFromSettings
    participant DB as PostgreSQL
    participant CL as Kalshi Client
    participant A as Kalshi API
    participant SC as Scanner

    T->>K: runScan()
    K->>K: Lock (prevent concurrent scans)
    K->>C: LoadFromSettings(db)
    C->>DB: GetSetting("kalshi_api_key_id")
    C->>DB: GetSetting("kalshi_private_key")
    C->>DB: GetSetting("kalshi_environment")
    C-->>K: Config

    K->>DB: GetSetting("kalshi_categories")
    K->>SC: RunScan(cfg, scanCfg)

    SC->>CL: client.New(cfg)
    CL->>CL: auth.NewSignerFromBytes(keyID, pemData)

    SC->>CL: GetCategories()
    CL->>A: GET /trade-api/v2/exchange/categories
    A-->>CL: categories[]

    SC->>CL: GetBalance()
    CL->>A: GET /trade-api/v2/portfolio/balance
    A-->>CL: balance

    SC->>CL: GetPositions()
    CL->>A: GET /trade-api/v2/portfolio/positions
    A-->>CL: positions[]

    loop For each category
        SC->>CL: GetSeriesList(category)
        CL->>A: GET /trade-api/v2/series?category=X
        loop For each series
            SC->>CL: GetMarkets(seriesTicker, "open")
            CL->>A: GET /trade-api/v2/markets?series=X&status=open
        end
    end

    SC->>SC: FilterByPriceRangeMulti(markets, 0.75, 0.90)
    SC->>SC: SortByAskPrice(filtered)
    SC->>SC: CalculateKelly() for each position
    SC->>SC: GenerateHTMLReportString(...)
    SC-->>K: ScanResult { HTML, Log, NumMarkets }

    K->>DB: SaveKalshiReport(html)
    K->>DB: AddKalshiScanLog(log, "")
```

**Key points:**
- Scans are triggered either by the background scheduler (every N hours) or manually via `POST /api/kalshi/refresh`
- The scheduler checks every 5 minutes whether enough time has elapsed since the last scan
- Concurrent scans are prevented via a mutex-protected `scanning` flag
- API credentials (API key ID, RSA private key) are loaded from the database `settings` table
- The client uses RSA-PSS (SHA-256) signing for Kalshi API authentication
- Rate limiting is enforced at 20 requests/second (Kalshi Basic tier)
- Reports are stored in `kalshi_reports` (only the 5 most recent are kept)
- Scan logs are stored in `kalshi_scan_log` (only the 20 most recent are kept)

---

## 5. OPML Import

```mermaid
sequenceDiagram
    participant B as Browser
    participant S as Server
    participant O as OPML Parser
    participant DB as PostgreSQL

    B->>S: POST /api/import-opml (multipart file upload)
    S->>O: opml.Parse(file)
    O-->>S: []FeedEntry (with folder paths)

    loop For each FeedEntry
        alt Has folder path
            S->>DB: GetOrCreateFolder(folderName, nil)
        end
        S->>DB: GetOrCreateFeed(folderID, title, url)
    end

    S-->>B: { "imported": 15, "skipped": 3 }
```

---

## 6. Settings Update

```mermaid
sequenceDiagram
    participant B as Browser
    participant S as Server
    participant DB as PostgreSQL
    participant F as .env File

    B->>S: POST /api/settings { key: value, ... }

    loop For each setting
        S->>DB: SetSetting(key, value)
    end

    Note over S: Kalshi settings stored in DB (settings table)
    Note over S: DB_URL changes written to .env file

    S-->>B: { "status": "ok" }
```

---

## 7. Graceful Shutdown

```mermaid
sequenceDiagram
    participant OS as OS Signal
    participant M as main.go
    participant S as Server
    participant K as KalshiManager
    participant P as Poller
    participant H as HTTP Server

    OS->>M: SIGINT / SIGTERM
    M->>S: srv.Stop()
    S->>K: kalshi.Stop()
    K->>K: close(stopCh) — scheduler exits
    S->>P: poller.Stop()
    P->>P: close(stopChan) + wg.Wait()
    S->>H: httpServer.Shutdown(5s timeout)
    H-->>S: Connections drained
    S-->>M: Shutdown complete
```
