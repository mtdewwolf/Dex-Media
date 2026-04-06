package main

import (
	"dex/backend/api"
	"dex/backend/auth"
	"dex/backend/db"
	"dex/backend/scanner"
	"dex/backend/scraper"
	"dex/backend/stream"
	"log"
	"net/http"
	"os"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
)

func main() {
	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = "./data/dex.db"
	}

	err := os.MkdirAll("./data", 0755)
	if err != nil {
		log.Fatal("Failed to create data directory:", err)
	}

	db.InitDB(dbPath)
	auth.InitSecret()

	// Start background cleanup task
	stream.StartCleanupTask()

	// Initial scan of all libraries in DB
	rows, err := db.DB.Query("SELECT path, type FROM libraries")
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var path, libType string
			if err := rows.Scan(&path, &libType); err == nil {
				log.Printf("Scanning library: %s (%s)\n", path, libType)
				go scanner.ScanDirectory(path, libType)
			}
		}
	}
	
	// Scrape metadata
	go scraper.ScrapeMetadata()

	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins: []string{"*"},
		AllowedMethods: []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders: []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
	}))

	api.SetupRoutes(r)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Println("Starting server on :" + port)
	err = http.ListenAndServe(":"+port, r)
	if err != nil {
		log.Fatal("Server failed to start:", err)
	}
}
