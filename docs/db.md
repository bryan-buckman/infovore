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
