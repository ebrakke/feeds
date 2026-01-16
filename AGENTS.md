# Agents Guide

## Development Environment

**Important:** Run the app directly on the host for development and testing. Do not use Docker for this project.

## Architecture

- **Backend**: Go HTTP server with SQLite database
- **Frontend**: SvelteKit SPA (Vite dev server)
- **Production**: `make build` embeds `web/dist` into the Go binary
- **Development**: Go server on `:8080`, Vite dev server on `:5173` (API proxied to Go)

### Quick Start (Local Dev)

```bash
# Go server with hot reload + Svelte dev server
make dev
```

The dev environment runs:
- Go server on port 8080 (with air for hot reload)
- Svelte dev server on port 5173 (proxies API to Go)

Access the app at http://localhost:5173

## Architecture

- **Backend**: Go HTTP server with SQLite database
- **Frontend**: SvelteKit SPA with TypeScript
- **Video Provider**: yt-dlp for fetching and streaming

## Key Directories

- `/internal/api` - API handlers and routes
- `/internal/db` - Database layer
- `/internal/youtube` - YouTube RSS fetching
- `/web/frontend` - SvelteKit frontend
- `/web/dist` - Built frontend assets (served by Go)

## API Patterns

- JSON API routes under `/api/*`
- Legacy template routes under `/legacy/*` (deprecated)
- SSE streaming for long-running operations (feed refresh)

## Testing

Run tests locally:

```bash
go test ./...
```
