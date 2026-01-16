# Agents Guide

## Development Environment

**Important:** This app should be run in Docker Compose for development and testing. Do not run it directly on the host machine.

### Quick Start (Docker Dev)

```bash
# Start dev environment with hot reload for Go and Svelte
make docker-dev

# Or rebuild and start
make docker-dev-build

# View logs
make docker-dev-logs

# Stop
make docker-dev-down

# Shell into container
make docker-dev-shell
```

The dev environment runs:
- Go server on port 8080 (with air for hot reload)
- Svelte dev server on port 5173 (proxies API to Go)

Access the app at http://localhost:5173

### Production (Docker)

```bash
make docker-up      # Start
make docker-down    # Stop
make docker-logs    # View logs
```

### Manual Docker Commands

```bash
# Start development environment
docker compose --profile dev up dev

# Or run in background
docker compose --profile dev up -d dev

# View logs
docker compose --profile dev logs -f

# Rebuild after changes
docker compose --profile dev up --build dev
```

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

Run tests inside the Docker container:

```bash
docker compose exec app go test ./...
```
