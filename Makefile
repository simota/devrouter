.PHONY: all build build-frontend build-go clean dev install test lint help

# Version info
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
DATE    ?= $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")

# Build flags
LDFLAGS := -s -w \
	-X main.version=$(VERSION) \
	-X main.commit=$(COMMIT) \
	-X main.buildDate=$(DATE)

# Default target
all: build

# Build everything (frontend + Go binary)
build: build-frontend build-go

# Build frontend assets
build-frontend:
	@echo "Building frontend..."
	cd web && pnpm install && pnpm run build

# Build Go binary (requires frontend to be built first)
build-go:
	@echo "Building Go binary..."
	go build -ldflags "$(LDFLAGS)" -o devrouter ./cmd/devrouter

# Install binary to $GOPATH/bin
install: build
	@echo "Installing devrouter..."
	go install -ldflags "$(LDFLAGS)" ./cmd/devrouter

# Clean build artifacts
clean:
	@echo "Cleaning..."
	rm -f devrouter
	rm -rf internal/devrouter/web/dist/assets
	rm -f internal/devrouter/web/dist/index.html

# Run frontend dev server
dev:
	cd web && pnpm run dev

# Run tests
test:
	go test ./...
	cd web && pnpm run test

# Run linters
lint:
	go vet ./...
	cd web && pnpm run check

# Help
help:
	@echo "Available targets:"
	@echo "  make build          - Build frontend and Go binary"
	@echo "  make build-frontend - Build frontend only"
	@echo "  make build-go       - Build Go binary only (requires frontend)"
	@echo "  make install        - Build and install to GOPATH/bin"
	@echo "  make clean          - Remove build artifacts"
	@echo "  make dev            - Run frontend dev server"
	@echo "  make test           - Run all tests"
	@echo "  make lint           - Run linters"
