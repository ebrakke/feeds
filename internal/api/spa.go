package api

import (
	"io"
	"io/fs"
	"net/http"
	"path"
	"strings"
)

// SPAHandler serves a single-page application from an embedded filesystem.
// It serves static files when they exist, and falls back to index.html for
// client-side routing.
type SPAHandler struct {
	fs fs.FS
}

// NewSPAHandler creates a handler for serving an embedded SPA.
// The fsys should contain the built SPA files (index.html, assets, etc.)
func NewSPAHandler(fsys fs.FS) *SPAHandler {
	return &SPAHandler{fs: fsys}
}

func (h *SPAHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Clean the path
	urlPath := r.URL.Path
	if urlPath == "" {
		urlPath = "/"
	}

	// Try to serve the actual file first
	filePath := strings.TrimPrefix(urlPath, "/")
	if filePath == "" {
		filePath = "index.html"
	}

	// Check if file exists
	f, err := h.fs.Open(filePath)
	if err == nil {
		defer f.Close()

		// Check if it's a directory
		stat, err := f.Stat()
		if err == nil && !stat.IsDir() {
			// Serve the file with appropriate content type
			contentType := getContentType(filePath)
			w.Header().Set("Content-Type", contentType)

			// Cache static assets (not index.html)
			if strings.HasPrefix(filePath, "_app/") {
				w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
			}

			io.Copy(w, f)
			return
		}
	}

	// File doesn't exist - serve index.html for client-side routing
	indexFile, err := h.fs.Open("index.html")
	if err != nil {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}
	defer indexFile.Close()

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	io.Copy(w, indexFile)
}

func getContentType(filePath string) string {
	ext := path.Ext(filePath)
	switch ext {
	case ".html":
		return "text/html; charset=utf-8"
	case ".css":
		return "text/css; charset=utf-8"
	case ".js":
		return "application/javascript"
	case ".json":
		return "application/json"
	case ".svg":
		return "image/svg+xml"
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".gif":
		return "image/gif"
	case ".ico":
		return "image/x-icon"
	case ".woff":
		return "font/woff"
	case ".woff2":
		return "font/woff2"
	case ".ttf":
		return "font/ttf"
	default:
		return "application/octet-stream"
	}
}
