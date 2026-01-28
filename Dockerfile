# Build stage
FROM golang:1.22-bookworm AS builder

WORKDIR /app

# Copy go mod files first for layer caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o infovore .

# Runtime stage
FROM debian:bookworm-slim

# Install ca-certificates for HTTPS feeds
RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app

# Copy binary from builder
COPY --from=builder /app/infovore .

# Create data directory for SQLite database
RUN mkdir -p /data

# Run as non-root user
RUN useradd -r -u 1001 infovore && chown -R infovore:infovore /app /data
USER infovore

# Expose the web port
EXPOSE 8080

# Database Configuration:
# The app reads DB_URL from environment variables or /data/.env file.
# You can also configure the database via the web UI Settings panel.
#
# Options:
#   SQLite (default): Leave DB_URL empty, uses /data/infovore.db
#   PostgreSQL: Set DB_URL=postgres://user:pass@host:5432/dbname
#
# Configuration methods:
#   1. Environment variable: docker run -e DB_URL="postgres://..."
#   2. .env file: Mount a ConfigMap/Secret to /data/.env
#   3. Web UI: Go to Settings and enter the database URL
#
# Example docker run commands:
#   SQLite:    docker run -p 8080:8080 -v infovore-data:/data infovore
#   PostgreSQL: docker run -p 8080:8080 -v infovore-data:/data -e DB_URL="postgres://..." infovore
#
# Kubernetes example (mount .env from Secret):
#   volumeMounts:
#     - name: db-config
#       mountPath: /data/.env
#       subPath: .env
#   volumes:
#     - name: db-config
#       secret:
#         secretName: infovore-db

# Default command uses SQLite with data directory
CMD ["./infovore", "-addr", ":8080", "-db", "/data/infovore.db", "-data-dir", "/data"]
