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
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o dist/$(BINARY_NAME)-linux-amd64 ./cmd/server

.PHONY: build-linux-arm64
build-linux-arm64:
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build $(LDFLAGS) -o dist/$(BINARY_NAME)-linux-arm64 ./cmd/server

# macOS builds
.PHONY: build-darwin
build-darwin: build-darwin-amd64 build-darwin-arm64

.PHONY: build-darwin-amd64
build-darwin-amd64:
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o dist/$(BINARY_NAME)-darwin-amd64 ./cmd/server

.PHONY: build-darwin-arm64
build-darwin-arm64:
	CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o dist/$(BINARY_NAME)-darwin-arm64 ./cmd/server

# Windows build
.PHONY: build-windows
build-windows:
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o dist/$(BINARY_NAME)-windows-amd64.exe ./cmd/server

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

# Development: run frontend dev server with API proxy (local, not Docker)
.PHONY: ensure-frontend-dist
ensure-frontend-dist:
	@if [ ! -f web/dist/index.html ]; then \
		echo "web/dist missing; building frontend..."; \
		cd web/frontend && bun install && bun run build; \
	fi

.PHONY: dev
dev: ensure-frontend-dist
	@sh -c 'trap "kill 0" INT TERM; air & cd web/frontend && bun run dev -- --host 0.0.0.0'

# Create dist directory
dist:
	mkdir -p dist

# Install to $GOPATH/bin
.PHONY: install
install:
	go install $(LDFLAGS) ./cmd/server

# Update and restart (for production servers running as systemd service)
.PHONY: update
update:
	git pull
	$(MAKE) build
	systemctl restart feeds

# Playwright e2e testing
.PHONY: e2e
e2e:
	cd web/frontend && bun run e2e

.PHONY: smoke
smoke:
	cd web/frontend && bun run e2e:smoke

.PHONY: e2e-ui
e2e-ui:
	cd web/frontend && bun run e2e:ui

.PHONY: e2e-screenshots
e2e-screenshots:
	cd web/frontend && bun run e2e:screenshots

.PHONY: playwright-install
playwright-install:
	cd web/frontend && bunx playwright install chromium

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
	@echo "Local development:"
	@echo "  make dev          - Run air (Go) + bun dev (frontend)"
	@echo ""
	@echo "Production deployment:"
	@echo "  make update       - Pull, build, and restart systemd service"
	@echo ""
	@echo "E2E testing (Playwright):"
	@echo "  make playwright-install - Install Playwright browsers"
	@echo "  make smoke              - Run smoke tests (quick verification)"
	@echo "  make e2e                - Run all Playwright tests"
	@echo "  make e2e-ui             - Run Playwright tests with UI"
	@echo "  make e2e-screenshots    - Capture mobile screenshots"
	@echo ""
	@echo "Note: Cross-platform builds no longer require CGO (pure Go sqlite)."
