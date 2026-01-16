# feeds Makefile
# Build a single binary with embedded templates and SPA

BINARY_NAME = feeds
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_TIME = $(shell date -u '+%Y-%m-%d_%H:%M:%S')
LDFLAGS = -ldflags "-s -w -X main.Version=$(VERSION) -X main.BuildTime=$(BUILD_TIME)"

# Default target
.PHONY: all
all: build

# Build frontend SPA
.PHONY: frontend
frontend:
	cd web/frontend && bun install && bun run build

# Build for current platform (includes frontend)
.PHONY: build
build: frontend
	go build $(LDFLAGS) -o $(BINARY_NAME) ./cmd/server

# Build Go only (skip frontend, assumes web/dist exists)
.PHONY: build-go
build-go:
	go build $(LDFLAGS) -o $(BINARY_NAME) ./cmd/server

# Build for all platforms
.PHONY: build-all
build-all: build-linux build-darwin build-windows

# Linux builds
.PHONY: build-linux
build-linux: build-linux-amd64 build-linux-arm64

.PHONY: build-linux-amd64
build-linux-amd64:
	CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o dist/$(BINARY_NAME)-linux-amd64 ./cmd/server

.PHONY: build-linux-arm64
build-linux-arm64:
	CGO_ENABLED=1 GOOS=linux GOARCH=arm64 go build $(LDFLAGS) -o dist/$(BINARY_NAME)-linux-arm64 ./cmd/server

# macOS builds
.PHONY: build-darwin
build-darwin: build-darwin-amd64 build-darwin-arm64

.PHONY: build-darwin-amd64
build-darwin-amd64:
	CGO_ENABLED=1 GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o dist/$(BINARY_NAME)-darwin-amd64 ./cmd/server

.PHONY: build-darwin-arm64
build-darwin-arm64:
	CGO_ENABLED=1 GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o dist/$(BINARY_NAME)-darwin-arm64 ./cmd/server

# Windows build (requires cross-compiler for CGO)
.PHONY: build-windows
build-windows:
	CGO_ENABLED=1 GOOS=windows GOARCH=amd64 CC=x86_64-w64-mingw32-gcc go build $(LDFLAGS) -o dist/$(BINARY_NAME)-windows-amd64.exe ./cmd/server

# Run the server
.PHONY: run
run: build
	./$(BINARY_NAME)

# Clean build artifacts
.PHONY: clean
clean:
	rm -f $(BINARY_NAME)
	rm -rf dist/
	rm -rf web/dist/

# Docker development commands
.PHONY: docker-dev
docker-dev:
	docker compose --profile dev up dev

.PHONY: docker-dev-build
docker-dev-build:
	docker compose --profile dev up --build dev

.PHONY: docker-dev-down
docker-dev-down:
	docker compose --profile dev down

.PHONY: docker-dev-logs
docker-dev-logs:
	docker compose --profile dev logs -f dev

.PHONY: docker-dev-shell
docker-dev-shell:
	docker compose --profile dev exec dev sh

# Docker production commands
.PHONY: docker-up
docker-up:
	docker compose up -d

.PHONY: docker-down
docker-down:
	docker compose down

.PHONY: docker-logs
docker-logs:
	docker compose logs -f

# Development: run frontend dev server with API proxy (local, not Docker)
.PHONY: dev
dev:
	cd web/frontend && bun run dev

# Create dist directory
dist:
	mkdir -p dist

# Install to $GOPATH/bin
.PHONY: install
install:
	go install $(LDFLAGS) ./cmd/server

.PHONY: help
help:
	@echo "feeds build targets:"
	@echo ""
	@echo "  make              - Build for current platform (frontend + Go)"
	@echo "  make build        - Build for current platform (frontend + Go)"
	@echo "  make build-go     - Build Go only (assumes web/dist exists)"
	@echo "  make frontend     - Build frontend SPA only"
	@echo "  make build-all    - Build for all platforms (requires cross-compilers)"
	@echo "  make build-darwin - Build for macOS (amd64 + arm64)"
	@echo "  make build-linux  - Build for Linux (amd64 + arm64)"
	@echo "  make run          - Build and run the server"
	@echo "  make clean        - Remove build artifacts"
	@echo "  make install      - Install to GOPATH/bin"
	@echo ""
	@echo "Docker development (recommended):"
	@echo "  make docker-dev       - Start dev environment (Go + Svelte hot reload)"
	@echo "  make docker-dev-build - Rebuild and start dev environment"
	@echo "  make docker-dev-down  - Stop dev environment"
	@echo "  make docker-dev-logs  - Follow dev container logs"
	@echo "  make docker-dev-shell - Shell into dev container"
	@echo ""
	@echo "Docker production:"
	@echo "  make docker-up        - Start production container"
	@echo "  make docker-down      - Stop production container"
	@echo "  make docker-logs      - Follow production logs"
	@echo ""
	@echo "Local development (without Docker):"
	@echo "  make dev          - Run frontend dev server with API proxy"
	@echo "  1. Run 'make build-go' to build Go server"
	@echo "  2. Run './feeds' in one terminal"
	@echo "  3. Run 'make dev' in another for frontend hot reload"
	@echo ""
	@echo "Note: Cross-platform builds no longer require CGO (pure Go sqlite)."
