# Agents Guide

## Development Environment

**Important:** This app should be run in Docker Compose for development and testing. Do not run it directly on the host machine.

```bash
# Start development environment
docker compose up

# Or run in background
docker compose up -d

# View logs
docker compose logs -f

# Rebuild after changes
docker compose up --build
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
