package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/erik/yt-app/internal/ai"
	"github.com/erik/yt-app/internal/api"
	"github.com/erik/yt-app/internal/db"
	"github.com/erik/yt-app/internal/ytdlp"
	"github.com/erik/yt-app/web"
)

// Set via ldflags at build time
var (
	Version   = "dev"
	BuildTime = "unknown"
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
	showVersion := flag.Bool("version", false, "Show version and exit")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "yt-app - YouTube subscription aggregator\n\n")
		fmt.Fprintf(os.Stderr, "Usage: yt-app [options]\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nEnvironment:\n")
		fmt.Fprintf(os.Stderr, "  OPENAI_API_KEY    Enable AI-powered subscription organization\n")
	}

	flag.Parse()

	if *showVersion {
		fmt.Printf("yt-app %s (built %s)\n", Version, BuildTime)
		os.Exit(0)
	}

	database, err := db.New(*dbPath)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer database.Close()

	yt := ytdlp.New(*ytdlpPath)

	// OpenAI client (optional - for AI grouping)
	var aiClient *ai.Client
	if key := os.Getenv("OPENAI_API_KEY"); key != "" {
		aiClient = ai.New(key)
		log.Println("OpenAI API key found - AI grouping enabled")
	} else {
		log.Println("No OPENAI_API_KEY set - AI grouping disabled")
	}

	server, err := api.NewServer(database, yt, aiClient, web.Templates)
	if err != nil {
		log.Fatalf("Failed to create server: %v", err)
	}

	mux := http.NewServeMux()
	server.RegisterRoutes(mux)

	log.Printf("Starting server on %s", *addr)
	if err := http.ListenAndServe(*addr, corsMiddleware(mux)); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
