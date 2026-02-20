# HTTP API Reference

All API endpoints are prefixed with `/api` unless otherwise noted. Responses are JSON unless specified.

## Page Routes

These routes return HTML pages.

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/` | Main tabbed shell (contains Reader, Markets, Settings as iframes) |
| `GET` | `/reader` | RSS reader home — shows all feeds and unread items |
| `GET` | `/reader/feed/{feedID}` | Items for a specific feed |
| `GET` | `/reader/folder/{folderID}` | Items for all feeds in a folder |
| `GET` | `/settings` | Settings/configuration page |
| `GET` | `/markets` | Latest Kalshi market scan report (full HTML) |

---

## RSS / Feed API

### Mark Items as Read

```
POST /api/mark-read
Content-Type: application/json
```

**Body:**
```json
{ "item_ids": [1, 2, 3] }
```

**Response:** `200 OK` with `{ "status": "ok" }`

---

### Delete Read Items

```
POST /api/delete-read
Content-Type: application/json
```

**Body:**
```json
{ "item_ids": [1, 2, 3] }
```

**Response:** `200 OK` with `{ "status": "ok" }`

---

### Add Feed

```
POST /api/feed
Content-Type: application/json
```

**Body:**
```json
{ "url": "https://example.com/feed.xml", "folder_id": 1 }
```

`folder_id` is optional. If omitted, the feed is unfiled.

**Response:** `200 OK` with `{ "id": 42 }`

---

### Delete Feed

```
DELETE /api/feed/{feedID}
```

**Response:** `200 OK` with `{ "status": "ok" }`

> Deleting a feed also deletes all its items (cascading).

---

### Move Feed to Folder

```
POST /api/feed/{feedID}/move
Content-Type: application/json
```

**Body:**
```json
{ "folder_id": 2 }
```

Set `folder_id` to `null` to un-file a feed.

**Response:** `200 OK` with `{ "status": "ok" }`

---

### Manage Feed Folders (Multi-Folder)

```
GET /api/feed/{feedID}/folders
```

Returns current folder assignments and list of all available folders.

**Response:** `200 OK`
```json
{
  "folder_ids": [1, 3],
  "all_folders": [
    { "id": 1, "name": "Tech" },
    { "id": 2, "name": "News" },
    { "id": 3, "name": "Comics" }
  ]
}
```

```
POST /api/feed/{feedID}/folders
Content-Type: application/json
```

Updates the folders a feed belongs to (sets the list explicitly).

**Body:**
```json
{ "folder_ids": [1, 3] }
```

**Response:** `200 OK` with `{ "status": "ok" }`

---

### Add Folder

```
POST /api/folder
Content-Type: application/json
```

**Body:**
```json
{ "name": "Technology" }
```

**Response:** `200 OK` with `{ "id": 5 }`

---

### Delete Folder

```
DELETE /api/folder/{folderID}
```

**Response:** `200 OK` with `{ "status": "ok" }`

---

### Refresh All Feeds

```
POST /api/refresh
```

Fetches all subscribed feeds and stores new items.

**Response:** `200 OK` with summary:
```json
{ "new_items": { "1": 5, "2": 0, "3": 12 } }
```

---

### Refresh Single Feed

```
POST /api/refresh-feed/{feedID}
```

**Response:** `200 OK` with `{ "new_items": 5 }`

---

### Refresh Folder

```
POST /api/refresh-folder/{folderID}
```

**Response:** `200 OK` with per-feed new item counts.

---

### Cleanup Read Items

```
POST /api/cleanup
```

Deletes all items marked as read across all feeds.

**Response:** `200 OK` with `{ "deleted": 42 }`

---

### Get Sidebar Data

```
GET /api/sidebar
```

Returns folders, feeds, and unread counts for rendering the sidebar.

**Response:** `200 OK` (JSON with folder/feed structure)

---

## OPML

### Import OPML

```
POST /api/import-opml
Content-Type: multipart/form-data
```

Upload an OPML file as form field `file`.

**Response:** `200 OK` with `{ "imported": 15, "skipped": 3 }`

---

### Export OPML

```
GET /api/export-opml
```

**Response:** `200 OK` with `Content-Type: application/xml`  
Downloads an OPML 2.0 file of all subscriptions.

---

## Settings

### Get Settings

```
GET /api/settings
```

**Response:** `200 OK`
```json
{
  "polling_interval_minutes": "30",
  "kalshi_api_key_id": "abc123",
  "kalshi_environment": "prod",
  "kalshi_categories": "Politics,Sports",
  "kalshi_scan_interval_hours": "6"
}
```

---

### Save Settings

```
POST /api/settings
Content-Type: application/json
```

**Body:**
```json
{
  "polling_interval_minutes": "60",
  "kalshi_api_key_id": "new-key",
  "kalshi_private_key": "-----BEGIN PRIVATE KEY-----\n...",
  "kalshi_environment": "demo"
}
```

**Response:** `200 OK` with `{ "status": "ok" }`

---

### Test Kalshi Connection

```
POST /api/settings/test-kalshi
```

Tests the current Kalshi API credentials by fetching the account balance.

**Response:** `200 OK` with `{ "status": "ok", "balance": 150.25 }`  
or `500` with error details.

---

### Get Database Settings

```
GET /api/database-settings
```

**Response:** `200 OK` with current database configuration.

---

### Save Database Settings

```
POST /api/database-settings
Content-Type: application/json
```

**Body:**
```json
{ "db_url": "postgres://user:pass@host:5432/dbname?sslmode=disable" }
```

Saves the DB URL to the `.env` file. Requires an app restart to take effect.

**Response:** `200 OK`

---

## Kalshi Scanner API

### Get Scanner Status

```
GET /api/kalshi/status
```

**Response:** `200 OK`
```json
{
  "scanning": false,
  "last_scan": "2026-02-16T22:00:00Z",
  "last_error": ""
}
```

---

### Trigger Manual Scan

```
POST /api/kalshi/refresh
```

Starts a background scan. Returns immediately.

**Response:** `200 OK` with `{ "status": "scan started" }`  
or `409 Conflict` if a scan is already running.

---

### Get Scan Log

```
GET /api/kalshi/log
```

Returns the 10 most recent scan log entries.

**Response:** `200 OK`
```json
[
  {
    "ID": 1,
    "Output": "Scanning categories: Politics\nFound 42 contracts...",
    "Error": "",
    "CreatedAt": "2026-02-16T22:00:00Z"
  }
]
```

---

### Get Kalshi Overrides

```
GET /api/kalshi/overrides
```

Returns all user-defined overrides (edge values, purchase prices) for Kalshi markets.

**Response:** `200 OK`
```json
{
  "KXDH...|YES|edge": 0.05,
  "KXdh...|YES|portfolio_price": 7500.0
}
```

---

### Set Kalshi Override

```
POST /api/kalshi/overrides
Content-Type: application/json
```

Updates a single override value. Keys are typically `ticker|side|field`.

**Body:**
```json
{
  "ticker": "KXDH...",
  "side": "YES",
  "field": "edge",
  "value": 0.05
}
```

**Response:** `200 OK` with `{ "status": "ok" }`
