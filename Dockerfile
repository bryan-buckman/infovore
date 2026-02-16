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

# Create data directory for .env file
RUN mkdir -p /data

# Run as non-root user
RUN useradd -r -u 1001 infovore && chown -R infovore:infovore /app /data
USER infovore

# Expose the web port
EXPOSE 8080

# Database Configuration:
# PostgreSQL is REQUIRED. Set DB_URL via environment variable or /data/.env file.
#
# Configuration methods:
#   1. Environment variable: docker run -e DB_URL="postgres://..."
#   2. .env file: Mount a ConfigMap/Secret to /data/.env
#
# Example:
#   docker run -p 8080:8080 -v infovore-data:/data \
#     -e DB_URL="postgres://user:pass@host:5432/infovore?sslmode=disable" infovore
#
# Kalshi Configuration:
#   After first boot, set Kalshi API credentials in the Settings UI or via SQL:
#     UPDATE settings SET value = 'your-key-id' WHERE key = 'kalshi_api_key_id';
#     UPDATE settings SET value = '-----BEGIN PRIVATE KEY-----...' WHERE key = 'kalshi_private_key';

# Default command — DB_URL must be provided via environment
CMD ["./infovore", "-addr", ":8080", "-data-dir", "/data"]
