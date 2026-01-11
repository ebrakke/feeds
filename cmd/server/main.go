package main

import (
	"flag"
	"log"
	"net/http"

	"github.com/erik/yt-app/internal/api"
	"github.com/erik/yt-app/internal/db"
	"github.com/erik/yt-app/internal/ytdlp"
)

// corsMiddleware adds CORS headers to all responses
func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		w.Header().Set("Access-Control-Expose-Headers", "HX-Redirect")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func main() {
	addr := flag.String("addr", ":8080", "HTTP server address")
	dbPath := flag.String("db", "yt-app.db", "SQLite database path")
	ytdlpPath := flag.String("ytdlp", "yt-dlp", "Path to yt-dlp binary")
	flag.Parse()

	database, err := db.New(*dbPath)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer database.Close()

	yt := ytdlp.New(*ytdlpPath)

	server, err := api.NewServer(database, yt, "web/templates")
	if err != nil {
		log.Fatalf("Failed to create server: %v", err)
	}

	mux := http.NewServeMux()
	server.RegisterRoutes(mux)

	// Static files
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.Dir("web/static"))))

	log.Printf("Starting server on %s", *addr)
	if err := http.ListenAndServe(*addr, corsMiddleware(mux)); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
