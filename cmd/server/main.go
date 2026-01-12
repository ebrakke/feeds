package main

import (
	"bufio"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/erik/feeds/internal/ai"
	"github.com/erik/feeds/internal/api"
	"github.com/erik/feeds/internal/db"
	"github.com/erik/feeds/internal/ytdlp"
	"github.com/erik/feeds/web"
)

// loadEnvFile loads environment variables from a .env file if it exists
func loadEnvFile(path string) {
	f, err := os.Open(path)
	if err != nil {
		return // silently ignore missing .env
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if key, val, ok := strings.Cut(line, "="); ok {
			key = strings.TrimSpace(key)
			val = strings.TrimSpace(val)
			// Remove surrounding quotes if present
			if len(val) >= 2 && ((val[0] == '"' && val[len(val)-1] == '"') || (val[0] == '\'' && val[len(val)-1] == '\'')) {
				val = val[1 : len(val)-1]
			}
			// Only set if not already in environment (env vars take precedence)
			if os.Getenv(key) == "" {
				os.Setenv(key, val)
			}
		}
	}
}

// getEnvOrDefault returns the environment variable value or a default
func getEnvOrDefault(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

// Set via ldflags at build time
var (
	Version   = "dev"
	BuildTime = "unknown"
)

// corsMiddleware adds CORS and cache-control headers to all responses
func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		w.Header().Set("Access-Control-Expose-Headers", "HX-Redirect")
		// Prevent caching by proxies/VPNs
		w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
		w.Header().Set("Pragma", "no-cache")
		w.Header().Set("Expires", "0")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func main() {
	// Load .env file first (before flag parsing so env vars are available for defaults)
	loadEnvFile(".env")

	// Flags with environment variable defaults
	addr := flag.String("addr", getEnvOrDefault("FEEDS_ADDR", ":8080"), "HTTP server address")
	dbPath := flag.String("db", getEnvOrDefault("FEEDS_DB", "feeds.db"), "SQLite database path")
	ytdlpPath := flag.String("ytdlp", getEnvOrDefault("FEEDS_YTDLP", "yt-dlp"), "Path to yt-dlp binary")
	showVersion := flag.Bool("version", false, "Show version and exit")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "feeds - YouTube subscription aggregator\n\n")
		fmt.Fprintf(os.Stderr, "Usage: feeds [options]\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nEnvironment (can also be set in .env file):\n")
		fmt.Fprintf(os.Stderr, "  FEEDS_ADDR        Server address (default :8080)\n")
		fmt.Fprintf(os.Stderr, "  FEEDS_DB          Database path (default feeds.db)\n")
		fmt.Fprintf(os.Stderr, "  FEEDS_YTDLP       Path to yt-dlp binary (default yt-dlp)\n")
		fmt.Fprintf(os.Stderr, "  OPENAI_API_KEY    Enable AI-powered subscription organization\n")
	}

	flag.Parse()

	if *showVersion {
		fmt.Printf("feeds %s (built %s)\n", Version, BuildTime)
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

	server, err := api.NewServer(database, yt, aiClient, web.Templates, web.Packs)
	if err != nil {
		log.Fatalf("Failed to create server: %v", err)
	}

	mux := http.NewServeMux()
	server.RegisterRoutes(mux)

	// Serve SPA for all non-API routes
	spaFS, err := fs.Sub(web.SPA, "dist")
	if err != nil {
		log.Fatalf("Failed to get SPA filesystem: %v", err)
	}
	spaHandler := api.NewSPAHandler(spaFS)
	mux.Handle("GET /", spaHandler)

	log.Printf("Starting server on %s", *addr)
	if err := http.ListenAndServe(*addr, corsMiddleware(mux)); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
