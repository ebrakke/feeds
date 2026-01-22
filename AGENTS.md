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

### Important: Server Architecture

**ALWAYS use `make dev` and access the app through http://localhost:5173**

The Vite dev server (port 5173) proxies ALL `/api/*` requests to the Go backend (port 8080). Do NOT:
- Try to access the Go server directly on port 8080 for frontend work
- Try to call API endpoints directly (they return HTML from Vite, not JSON)
- Start the servers separately - `make dev` handles both

For Playwright tests:
- The dev server must be running (`make dev`)
- Tests use `NO_WEBSERVER=1` env var when the server is already running
- Base URL is always http://localhost:5173

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

Run Go tests locally:

```bash
go test ./...
```

## E2E Testing with Playwright

The frontend includes Playwright for end-to-end testing and visual regression capture.

### Setup

Install Playwright browsers (one-time setup):

```bash
make playwright-install
```

### Running Tests

```bash
# Run smoke tests (quick verification after major changes)
make smoke

# Run all e2e tests
make e2e

# Run tests with interactive UI
make e2e-ui

# Capture mobile screenshots for UI review
make e2e-screenshots
```

### Smoke Tests

**Always run `make smoke` after major changes** to verify core functionality works:

- Home page and navigation
- Feed pages load
- API endpoints respond correctly
- Import page works (without AI)

The smoke tests run in ~10 seconds and catch obvious regressions.

### Screenshot Capture

The `e2e-screenshots` command captures mobile viewport screenshots of all main pages. This is useful for:

- Reviewing mobile UI before/after changes
- Visual regression testing
- Documenting UI states

Screenshots are saved to `web/frontend/e2e/screenshots/`.

### Playwright Config

The configuration is in `web/frontend/playwright.config.ts`. It includes:

- Desktop Chrome, Mobile Chrome (Pixel 5), and Mobile Safari (iPhone 14 Pro) projects
- Auto-starts dev server when not in CI
- `--no-sandbox` flag for running as root

### Writing Tests

Add test files to `web/frontend/e2e/` with `.spec.ts` extension:

```typescript
import { test, expect } from '@playwright/test';

test('home page loads', async ({ page }) => {
  await page.goto('/');
  await expect(page).toHaveTitle(/feeds/i);
});
```
