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

# Default command
CMD ["./infovore", "-addr", ":8080", "-db", "/data/infovore.db"]
