.PHONY: build install clean test run

# Go binary path (adjust if go is in a different location)
GO=$(shell which go 2>/dev/null || echo "/usr/local/go/bin/go")

# Build variables
BINARY_NAME=bookdl
VERSION=$(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT=$(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_DIR=./build
LDFLAGS=-ldflags "-X github.com/billmal071/bookdl/internal/cli.Version=$(VERSION) -X github.com/billmal071/bookdl/internal/cli.Commit=$(COMMIT)"

# Default target
all: build

# Build the binary (CGO disabled for modernc.org/sqlite compatibility)
build:
	@echo "Building $(BINARY_NAME)..."
	@mkdir -p $(BUILD_DIR)
	CGO_ENABLED=0 $(GO) build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/bookdl

# Install to GOPATH/bin
install:
	@echo "Installing $(BINARY_NAME)..."
	CGO_ENABLED=0 $(GO) install $(LDFLAGS) ./cmd/bookdl

# Clean build artifacts
clean:
	@echo "Cleaning..."
	@rm -rf $(BUILD_DIR)
	$(GO) clean

# Run tests
test:
	$(GO) test -v ./...

# Run the application
run:
	$(GO) run ./cmd/bookdl $(ARGS)

# Fetch dependencies
deps:
	$(GO) mod tidy
	$(GO) mod download

# Format code
fmt:
	$(GO) fmt ./...

# Lint code
lint:
	golangci-lint run

# Build for multiple platforms
build-all: build-linux build-darwin build-windows

build-linux:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GO) build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 ./cmd/bookdl

build-darwin:
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 $(GO) build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-amd64 ./cmd/bookdl
	CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 $(GO) build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-arm64 ./cmd/bookdl

build-windows:
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 $(GO) build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-windows-amd64.exe ./cmd/bookdl

# Help
help:
	@echo "Available targets:"
	@echo "  build       - Build the binary"
	@echo "  install     - Install to GOPATH/bin"
	@echo "  clean       - Remove build artifacts"
	@echo "  test        - Run tests"
	@echo "  run         - Run the application (use ARGS= for arguments)"
	@echo "  deps        - Fetch dependencies"
	@echo "  fmt         - Format code"
	@echo "  lint        - Run linter"
	@echo "  build-all   - Build for all platforms"
