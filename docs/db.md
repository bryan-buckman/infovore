# Database Configuration

Infovore requires **PostgreSQL** as its database. SQLite is not supported.

## Connection

Set the `DB_URL` environment variable or use the `-db-url` flag:

```
DB_URL=postgres://user:password@localhost:5432/infovore?sslmode=disable
```

### Quick Start (Docker)

```bash
docker run -d --name infovore-db \
  -e POSTGRES_USER=infovore \
  -e POSTGRES_PASSWORD=secret \
  -e POSTGRES_DB=infovore \
  -p 5432:5432 \
  postgres:16-alpine
```

Then run Infovore:
```bash
DB_URL=postgres://infovore:secret@localhost:5432/infovore?sslmode=disable go run .
```

## Schema

Tables are created automatically on first run. The schema includes:

| Table | Purpose |
|-------|---------|
| `folders` | RSS feed folders/categories |
| `feeds` | RSS feed subscriptions |
| `items` | RSS feed items/articles |
| `settings` | Key-value application settings |
| `kalshi_reports` | Stored Kalshi market scan HTML reports (last 5 kept) |
| `kalshi_scan_log` | Scan execution log entries (last 20 kept) |

## Kalshi Settings

The following settings are stored in the `settings` table and configure the Kalshi scanner:

| Key | Default | Description |
|-----|---------|-------------|
| `kalshi_api_key_id` | `""` | Kalshi API key ID |
| `kalshi_private_key` | `""` | PEM-encoded RSA private key |
| `kalshi_environment` | `prod` | `prod` or `demo` |
| `kalshi_categories` | `Politics` | Comma-separated list of categories to scan |
| `kalshi_scan_interval_hours` | `6` | Hours between automatic scans |

These can be set via the Infovore UI settings page or directly in the database:

```sql
UPDATE settings SET value = 'your-api-key-id' WHERE key = 'kalshi_api_key_id';
UPDATE settings SET value = '-----BEGIN PRIVATE KEY-----
...
-----END PRIVATE KEY-----' WHERE key = 'kalshi_private_key';
```

## VM Sizing (Proxmox)

### Workload Analysis

**RSS feed updater** (10 parallel workers):
- Each worker: `SELECT` existing item GUIDs for one feed, then bulk `INSERT` new items
- Peak: 10 simultaneous write transactions against the `items` table
- Initial OPML import of 3,000 feeds = up to ~150,000 new rows inserted (at ~50 items/feed)
- Subsequent updates are incremental — most feeds return 0–5 new items

**Kalshi scanner** (runs concurrently with RSS updates):
- Network-bound: sequentially fetches series/market data from the Kalshi API — no DB reads during the scan
- Single large write at completion: 1 `INSERT` into `kalshi_reports` (HTML blob, typically 200–600 KB)
- Also writes 1 row to `kalshi_scan_log` and `DELETE`s old records (table stays under 20 rows)
- DB impact is minimal but the large TEXT blob write will contend with RSS bulk inserts at WAL flush time

**Peak concurrent connections:**
| Source | Connections |
|--------|-------------|
| RSS workers (PostgreSQL mode) | 10 |
| Kalshi scanner | 1 |
| Web UI / API requests | 1–3 |
| Admin / psql | 1 |
| **Total peak** | **~15** |

The app's connection pool is capped at 25 (`SetMaxOpenConns(25)`), so the DB needs to handle at least 30 `max_connections` comfortably.

### Data Size Estimates

| Table | Rows (3,000 feeds) | Avg row size | Est. size |
|-------|--------------------|--------------|-----------|
| `feeds` | 3,000 | 300 B | ~1 MB |
| `items` (100 items/feed kept) | 300,000 | 2 KB | ~600 MB |
| `items` (500 items/feed kept) | 1,500,000 | 2 KB | ~3 GB |
| `folders` | ~100 | 100 B | negligible |
| `kalshi_reports` | 5 (trimmed) | 400 KB | ~2 MB |
| **Indexes** | — | — | ~20–30% of table size |

The `items` table dominates. The `content` column (full article HTML) is the main size driver — 2 KB average is conservative; real-world content can be 5–20 KB per item.

### Recommended VM Sizes

**Small — up to 1,000 feeds, light Kalshi use**
| Resource | Value |
|----------|-------|
| vCPU | 2 |
| RAM | 4 GB |
| Disk | 20 GB SSD |
| Est. `items` table | ~120 MB |
| Est. initial load time | 3–5 min |

**Medium — 1,000–5,000 feeds (recommended starting point)**
| Resource | Value |
|----------|-------|
| vCPU | 4 |
| RAM | 8 GB |
| Disk | 50 GB SSD |
| Est. `items` table | 600 MB – 3 GB |
| Est. initial load time | 8–15 min |

**Large — 5,000+ feeds or high item retention**
| Resource | Value |
|----------|-------|
| vCPU | 8 |
| RAM | 16 GB |
| Disk | 100 GB SSD (or NVMe) |
| Est. `items` table | 3–15 GB |
| Est. initial load time | 20–40 min |

> The bottleneck during initial load is **network I/O** (rate-limited HTTP fetches), not the database.
> After the initial import, steady-state CPU and RAM usage drops sharply as most feeds return no new items.

### PostgreSQL Configuration

For the Medium VM (8 GB RAM), add these to `postgresql.conf`:

```ini
# Memory
shared_buffers = 2GB                  # 25% of RAM
effective_cache_size = 6GB            # 75% of RAM
work_mem = 8MB                        # per sort/hash per connection (15 conns × 8MB = 120MB peak)
maintenance_work_mem = 256MB          # for VACUUM, CREATE INDEX

# Write performance (bulk insert optimisation)
wal_buffers = 64MB
checkpoint_completion_target = 0.9
max_wal_size = 2GB

# Connections
max_connections = 50                  # app uses 25; leaves room for admin and Kalshi writes
```

For the Small VM (4 GB RAM), halve the memory values:

```ini
shared_buffers = 1GB
effective_cache_size = 3GB
work_mem = 4MB
maintenance_work_mem = 128MB
wal_buffers = 16MB
max_connections = 50
```

### Why These Numbers

- **`shared_buffers = 25% RAM`**: keeps the hot working set (the `items` index + recent rows) in memory. With 300K–1.5M rows, the B-tree index on `(feed_id, guid)` alone is 5–25 MB — easily fits in cache.
- **`work_mem = 4–8 MB`**: RSS queries are simple index lookups and small INSERTs; they don't sort large sets. Keeping `work_mem` low prevents 15 concurrent connections from exhausting RAM.
- **`checkpoint_completion_target = 0.9`**: spreads WAL checkpoints over 90% of the checkpoint interval, smoothing out the I/O spike that would otherwise coincide with a Kalshi report write during a feed refresh.
- **Disk SSD**: the initial 150K-row import issues thousands of small WAL writes. A spinning disk will be the bottleneck; SSD eliminates it.
