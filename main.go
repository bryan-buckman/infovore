// Infovore - An RSS Reader
package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/bryan-buckman/infovore/internal/database"
	"github.com/bryan-buckman/infovore/internal/server"
)

func main() {
	addr := flag.String("addr", ":8080", "HTTP server address")
	dbPath := flag.String("db", "infovore.db", "SQLite database path")
	flag.Parse()

	log.Println("Infovore starting...")

	db, err := database.New(*dbPath)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	srv, err := server.New(db)
	if err != nil {
		log.Fatalf("Failed to create server: %v", err)
	}

	// Handle graceful shutdown.
	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		<-sigChan
		log.Println("Shutting down...")
		srv.Stop()
		db.Close()
		os.Exit(0)
	}()

	if err := srv.Start(*addr); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}
