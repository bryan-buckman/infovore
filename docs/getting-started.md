# Getting Started

This guide walks through setting up Infovore from source, configuring it for both RSS reading and Kalshi market scanning, and deploying it with Docker.

## Prerequisites

| Requirement | Version | Notes |
|-------------|---------|-------|
| Go | 1.22+ | [golang.org/dl](https://go.dev/dl/) |
| PostgreSQL | 14+ | Any hosted or local instance |
| Git | any | For cloning the repository |

## 1. Build from Source

```bash
git clone https://github.com/bryan-buckman/infovore.git
cd infovore
go build -o infovore .
```

This produces a single `infovore` binary with all templates and static assets embedded.

## 2. Create the Database

```bash
# Using psql
createdb infovore

# Or with full connection:
psql -c "CREATE DATABASE infovore;" \
  "postgres://user:pass@localhost:5432/postgres?sslmode=disable"
```

Tables are created automatically on first application startup — no manual migration is needed.

## 3. Configure the Database Connection

You can provide the connection URL in three ways (in priority order):

1. **Command-line flag:**
   ```bash
   ./infovore -db-url "postgres://user:pass@localhost:5432/infovore?sslmode=disable"
   ```

2. **Environment variable:**
   ```bash
   export DB_URL="postgres://user:pass@localhost:5432/infovore?sslmode=disable"
   ./infovore
   ```

3. **.env file** (in the project directory or `/data/.env`):
   ```
   DB_URL=postgres://user:pass@localhost:5432/infovore?sslmode=disable
   ```

## 4. Start the Server

```bash
./infovore
```

Default address is `:8080`. Override with:

```bash
./infovore -addr ":3000"
```

Open `http://localhost:8080` in your browser.

## 5. Using the RSS Reader

### Add Feeds

1. Click the **+ Feed** button in the sidebar
2. Enter the RSS/Atom feed URL
3. Optionally select a folder

### Multi-Folder Feeds

Feeds can belong to multiple folders. Right-click a feed in the sidebar and select **Add to Folders...** to manage its assignments.


### Import from OPML

1. Go to **Settings** (gear icon or `/settings`)
2. Under "OPML", click **Import**
3. Upload your `.opml` file

Feeds will be organized into folders based on the OPML structure.

### Reading

- Click a feed or folder in the sidebar to view items
- Items are displayed newest-first
- Click an item title to open it in a new tab
- Use **Mark Read** to mark items as read
- Use **Cleanup** to delete all read items

### Refresh Feeds

Click the **Refresh** button to fetch new items from all feeds. You can also refresh individual feeds or folders by right-clicking.

## 6. Configuring the Kalshi Scanner

### Get API Credentials

1. Sign up at [kalshi.com](https://kalshi.com)
2. Go to **Settings → API Access** in your Kalshi account
3. Create an API key pair (you'll get a Key ID and a Private Key)

### Enter Credentials in Infovore

1. Go to **Settings** in the Infovore UI
2. In the **Kalshi** section, enter:
   - **API Key ID** — your Kalshi API key ID
   - **Private Key** — paste the full PEM-encoded private key
   - **Environment** — `prod` for real markets, `demo` for paper trading
3. Click **Save**

### Configure Scan Parameters

In Settings, you can also set:

| Setting | Default | Description |
|---------|---------|-------------|
| Categories | `Politics` | Comma-separated list (e.g., `Politics,Sports,Economics`) |
| Scan Interval | `6` hours | How often the background scanner runs |

### View Reports

- The **Markets** tab in the main UI shows the latest scan report
- Reports include:
  - Filtered markets in your price range (75¢–90¢ by default)
  - Kelly Criterion recommendations
  - Your current portfolio and positions
  - Transaction history for the calendar year

### Editable Reports & Persistence

You can manually override certain values in the report to test scenarios:
- **Edge**: Edit the edge/Kelly bias in the Portfolio table.
- **Purchase Price**: Edit the purchase price in both Portfolio and Transactions tables to fix API data gaps.

Changes are **auto-saved** to the database and persist across page reloads and new scans.


### Manual Scan

You can trigger a scan at any time via the API:

```bash
curl -X POST http://localhost:8080/api/kalshi/refresh
```

Check status:

```bash
curl http://localhost:8080/api/kalshi/status
```

## 7. Docker Deployment

### Build the Image

```bash
docker build -t infovore .
```

### Run with Docker

```bash
docker run -d \
  --name infovore \
  -p 8080:8080 \
  -e DB_URL="postgres://user:pass@db-host:5432/infovore?sslmode=disable" \
  infovore
```

### Docker Compose Example

```yaml
version: '3.8'

services:
  db:
    image: postgres:16-alpine
    environment:
      POSTGRES_USER: infovore
      POSTGRES_PASSWORD: secret
      POSTGRES_DB: infovore
    volumes:
      - pgdata:/var/lib/postgresql/data
    ports:
      - "5432:5432"

  app:
    build: .
    ports:
      - "8080:8080"
    environment:
      DB_URL: "postgres://infovore:secret@db:5432/infovore?sslmode=disable"
    depends_on:
      - db

volumes:
  pgdata:
```

### Kubernetes

For Kubernetes deployments, provide `DB_URL` via a Secret mounted as a `.env` file:

```yaml
volumeMounts:
  - name: db-config
    mountPath: /data/.env
    subPath: .env
volumes:
  - name: db-config
    secret:
      secretName: infovore-db
```

## 8. Development Workflow

### Run in Development

```bash
# Quick start with go run
DB_URL="postgres://user:pass@localhost:5432/infovore?sslmode=disable" go run .

# Or use the Makefile
make run
```

### Run Tests

```bash
go test ./...
```

### Code Checks

```bash
go vet ./...
go build ./...
```

### Dev Container

A `.devcontainer/devcontainer.json` is provided for VS Code Dev Containers. It sets up a Go development environment automatically.

## Troubleshooting

| Problem | Solution |
|---------|----------|
| `Database URL is required` | Set `DB_URL` env var or use `-db-url` flag |
| `Only PostgreSQL is supported` | URL must start with `postgres://` or `postgresql://` |
| Feed returns 403 errors | Some feeds block automated fetchers; this is normal |
| Kalshi scan fails with "config error" | Check that API Key ID and Private Key are set in Settings |
| Port already in use | Use `-addr ":3001"` to pick a different port |
