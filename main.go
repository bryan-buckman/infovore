// Infovore - An RSS Reader
package main

import (
	"bufio"
	"errors"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/bryan-buckman/infovore/internal/database"
	"github.com/bryan-buckman/infovore/internal/server"
)

// loadEnvFile loads environment variables from a .env file.
// It does not override existing environment variables.
func loadEnvFile(path string) {
	file, err := os.Open(path)
	if err != nil {
		return // File doesn't exist or can't be read, that's fine
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		// Parse KEY=value
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		// Remove surrounding quotes if present
		if len(value) >= 2 && ((value[0] == '"' && value[len(value)-1] == '"') ||
			(value[0] == '\'' && value[len(value)-1] == '\'')) {
			value = value[1 : len(value)-1]
		}
		// Only set if not already set in environment
		if os.Getenv(key) == "" {
			os.Setenv(key, value)
		}
	}
}

// getEnvFilePath returns the path to the .env file.
// Uses ENV_FILE environment variable if set, otherwise looks in data directory or current directory.
func getEnvFilePath() string {
	if envFile := os.Getenv("ENV_FILE"); envFile != "" {
		return envFile
	}
	// Check /data/.env first (for containerized deployments)
	if _, err := os.Stat("/data/.env"); err == nil {
		return "/data/.env"
	}
	// Fall back to current directory
	return ".env"
}

func main() {
	addr := flag.String("addr", ":8080", "HTTP server address")
	dbURL := flag.String("db-url", "", "PostgreSQL connection URL (postgres://user:pass@host:port/dbname)")
	dataDir := flag.String("data-dir", "", "Data directory for .env file (default: /data or current directory)")
	flag.Parse()

	log.Println("Infovore starting...")

	// Set data directory for .env file location
	envFilePath := getEnvFilePath()
	if *dataDir != "" {
		envFilePath = filepath.Join(*dataDir, ".env")
	}

	// Load .env file (environment variables take precedence)
	loadEnvFile(envFilePath)
	log.Printf("Loaded environment from: %s", envFilePath)

	// Check for DB_URL from environment (set via .env or actual environment)
	envDBURL := os.Getenv("DB_URL")
	if envDBURL != "" && *dbURL == "" {
		*dbURL = envDBURL
		log.Printf("Using DB_URL from environment")
	}

	// Store the env file path for the server to use when saving settings
	os.Setenv("INFOVORE_ENV_FILE", envFilePath)

	// PostgreSQL is required
	if *dbURL == "" {
		log.Fatalf("Database URL is required. Set DB_URL environment variable or use -db-url flag.\n" +
			"  Example: DB_URL=postgres://user:pass@localhost:5432/infovore?sslmode=disable")
	}
	if !strings.HasPrefix(*dbURL, "postgres://") && !strings.HasPrefix(*dbURL, "postgresql://") {
		log.Fatalf("Only PostgreSQL is supported. URL must start with postgres:// or postgresql://")
	}

	log.Printf("Connecting to PostgreSQL database...")
	var db database.Store
	var err error
	db, err = database.NewPostgres(*dbURL)

	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	srv, err := server.New(db)
	if err != nil {
		log.Fatalf("Failed to create server: %v", err)
	}

	// Handle graceful shutdown in goroutine.
	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		<-sigChan
		log.Println("Received shutdown signal...")
		srv.Stop()
	}()

	// Start server (blocks until shutdown).
	if err := srv.Start(*addr); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatalf("Server error: %v", err)
	}

	log.Println("Goodbye!")
}
