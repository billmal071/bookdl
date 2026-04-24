# audbookdl Foundation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Scaffold the audbookdl standalone project with domain types, SQLite database, configuration, basic CLI skeleton, and the Source interface — everything downstream plans (sources, downloader, TUI, player) build on.

**Architecture:** Standalone Go module at `~/Documents/personal/audbookdl`. Mirrors bookdl's proven patterns (Cobra CLI, Viper config, modernc.org/sqlite) but adapted for the audiobook domain. The Source interface defines the contract all source clients will implement in Plan 2.

**Tech Stack:** Go 1.22+, Cobra, Viper, modernc.org/sqlite (pure Go, CGO_ENABLED=0)

---

### Task 1: Initialize Go Module and Project Structure

**Files:**
- Create: `cmd/audbookdl/main.go`
- Create: `go.mod`
- Create: `Makefile`
- Create: `CLAUDE.md`
- Create: `.gitignore`

- [ ] **Step 1: Create the project directory and initialize git**

```bash
mkdir -p ~/Documents/personal/audbookdl
cd ~/Documents/personal/audbookdl
git init
```

- [ ] **Step 2: Create `.gitignore`**

Create file `.gitignore`:

```
# Build output
/build/

# IDE
.idea/
.vscode/
*.swp
*.swo

# OS
.DS_Store
Thumbs.db

# Go
*.exe
*.exe~
*.dll
*.so
*.dylib
*.test
*.out
vendor/

# Config (local)
*.db
```

- [ ] **Step 3: Initialize Go module**

```bash
cd ~/Documents/personal/audbookdl
go mod init github.com/billmal071/audbookdl
```

- [ ] **Step 4: Create entry point `cmd/audbookdl/main.go`**

```go
package main

import (
	"fmt"
	"os"

	"github.com/billmal071/audbookdl/internal/cli"
)

func main() {
	if err := cli.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
```

- [ ] **Step 5: Create `Makefile`**

```makefile
.PHONY: build install clean test run deps fmt lint ci ci-format ci-vet ci-test ci-build build-all

GO=$(shell which go 2>/dev/null || echo "/usr/local/go/bin/go")

BINARY_NAME=audbookdl
VERSION=$(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT=$(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_DIR=./build
LDFLAGS=-ldflags "-X github.com/billmal071/audbookdl/internal/cli.Version=$(VERSION) -X github.com/billmal071/audbookdl/internal/cli.Commit=$(COMMIT)"

all: build

build:
	@echo "Building $(BINARY_NAME)..."
	@mkdir -p $(BUILD_DIR)
	CGO_ENABLED=0 $(GO) build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/audbookdl

install:
	@echo "Installing $(BINARY_NAME)..."
	CGO_ENABLED=0 $(GO) install $(LDFLAGS) ./cmd/audbookdl

clean:
	@echo "Cleaning..."
	@rm -rf $(BUILD_DIR)
	$(GO) clean

test:
	$(GO) test -v ./...

run:
	$(GO) run ./cmd/audbookdl $(ARGS)

deps:
	$(GO) mod tidy
	$(GO) mod download

fmt:
	$(GO) fmt ./...

lint:
	golangci-lint run

ci: ci-format ci-vet ci-test ci-build
	@echo "All CI checks passed!"

ci-format:
	@echo "Checking formatting..."
	@unformatted=$$(gofmt -l .); \
	if [ -n "$$unformatted" ]; then \
		echo "The following files are not formatted:"; \
		echo "$$unformatted"; \
		exit 1; \
	fi
	@echo "Format check passed"

ci-vet:
	@echo "Running go vet..."
	@go vet ./...
	@echo "Vet passed"

ci-test:
	@echo "Running tests..."
	@go test -v ./...
	@echo "Tests passed"

ci-build:
	@echo "Building..."
	@CGO_ENABLED=0 go build -o /dev/null ./cmd/audbookdl
	@echo "Build passed"

build-all: build-linux build-darwin build-windows

build-linux:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GO) build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 ./cmd/audbookdl
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 $(GO) build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-arm64 ./cmd/audbookdl

build-darwin:
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 $(GO) build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-amd64 ./cmd/audbookdl
	CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 $(GO) build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-arm64 ./cmd/audbookdl

build-windows:
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 $(GO) build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-windows-amd64.exe ./cmd/audbookdl

help:
	@echo "Available targets:"
	@echo "  build        - Build the binary"
	@echo "  install      - Install to GOPATH/bin"
	@echo "  clean        - Remove build artifacts"
	@echo "  test         - Run tests"
	@echo "  run          - Run the application (use ARGS= for arguments)"
	@echo "  deps         - Fetch dependencies"
	@echo "  fmt          - Format code"
	@echo "  lint         - Run linter"
	@echo "  build-all    - Build for all platforms"
	@echo "  ci           - Run all CI checks"
```

- [ ] **Step 6: Create `CLAUDE.md`**

```markdown
# CLAUDE.md

## Project Overview

**audbookdl** is a Go CLI tool for searching and downloading free audiobooks from multiple public sources (LibriVox, Internet Archive, Loyal Books, Open Library). It features a full-screen TUI built with the Charm stack, a built-in terminal audio player, resumable chunked downloads, and SQLite-backed state tracking.

## Build & Development Commands

\`\`\`bash
make build          # Build to ./build/audbookdl (CGO_ENABLED=0)
make install        # Install to GOPATH/bin
make test           # go test -v ./...
make fmt            # Format code
make lint           # golangci-lint (must be installed separately)
make run ARGS="..." # Build and run with arguments
make deps           # go mod tidy && go mod download
make build-all      # Cross-compile for Linux, macOS (amd64+arm64), Windows
\`\`\`

CGO is disabled — SQLite uses `modernc.org/sqlite` (pure Go). Do not introduce cgo-dependent drivers.

## Architecture

### Source Interface (`internal/source/`)
Defines `Source` interface and shared types (`Audiobook`, `Chapter`). All source clients implement this.

### Source Clients (`internal/librivox/`, `internal/archive/`, `internal/loyalbooks/`, `internal/openlibrary/`)
Each source implements `source.Source` with API or scraper clients.

### Search Orchestrator (`internal/search/`)
Fans out queries to all sources concurrently via `errgroup`, merges results.

### Download Manager (`internal/downloader/`)
Album-aware downloads — treats an audiobook (multiple chapter files) as one unit. Per-chapter chunked downloads with SQLite tracking for pause/resume.

### Audio Player (`internal/player/`)
Terminal audio player using `gopxl/beep` + `ebitengine/oto`. Supports speed control, volume, sleep timer, position memory.

### Metadata Tagger (`internal/tagger/`)
Post-download ID3/M4B tagging with cover art embedding.

### Database (`internal/db/`)
SQLite with WAL mode. Tables: `audiobook_downloads`, `chapter_downloads`, `chunks`, `bookmarks`, `playback_state`, `search_history`, `search_cache`.

### TUI (`internal/tui/`)
Full-screen bubbletea app with 4 tabs: Search, Downloads, Library, Player. Uses bubbles components and lipgloss styling.

### CLI (`internal/cli/`)
Cobra-based. `root.go` handles init/cleanup. No args launches TUI.

### Config (`internal/config/`)
Viper-based YAML at `~/.config/audbookdl/config.yaml`. Environment overrides via `AUDBOOKDL_*`.

### Version Injection
LDFLAGS inject `Version` and `Commit` into `internal/cli` at build time.
```

- [ ] **Step 7: Commit scaffold**

```bash
git add .gitignore cmd/audbookdl/main.go go.mod Makefile CLAUDE.md
git commit -m "feat: initialize audbookdl project scaffold"
```

---

### Task 2: Source Interface and Domain Types

**Files:**
- Create: `internal/source/source.go`
- Create: `internal/source/types.go`
- Test: `internal/source/types_test.go`

- [ ] **Step 1: Write the test for domain types**

Create file `internal/source/types_test.go`:

```go
package source

import (
	"testing"
	"time"
)

func TestAudiobook_DurationFormatted(t *testing.T) {
	tests := []struct {
		name     string
		duration time.Duration
		want     string
	}{
		{"zero", 0, "0m"},
		{"minutes only", 45 * time.Minute, "45m"},
		{"hours and minutes", 2*time.Hour + 30*time.Minute, "2h 30m"},
		{"hours only", 5 * time.Hour, "5h 0m"},
		{"long audiobook", 11*time.Hour + 32*time.Minute, "11h 32m"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := &Audiobook{Duration: tt.duration}
			if got := a.DurationFormatted(); got != tt.want {
				t.Errorf("DurationFormatted() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestAudiobook_SizeFormatted(t *testing.T) {
	tests := []struct {
		name string
		size int64
		want string
	}{
		{"zero", 0, "0 B"},
		{"bytes", 500, "500 B"},
		{"kilobytes", 1536, "1.5 KB"},
		{"megabytes", 5242880, "5.0 MB"},
		{"gigabytes", 1073741824, "1.0 GB"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := &Audiobook{TotalSize: tt.size}
			if got := a.SizeFormatted(); got != tt.want {
				t.Errorf("SizeFormatted() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestChapter_FilenamePrefix(t *testing.T) {
	tests := []struct {
		name  string
		index int
		title string
		want  string
	}{
		{"single digit", 1, "A Scandal in Bohemia", "01 - A Scandal in Bohemia"},
		{"double digit", 12, "The Final Problem", "12 - The Final Problem"},
		{"zero index", 0, "Introduction", "00 - Introduction"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Chapter{Index: tt.index, Title: tt.title}
			if got := c.FilenamePrefix(); got != tt.want {
				t.Errorf("FilenamePrefix() = %q, want %q", got, tt.want)
			}
		})
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd ~/Documents/personal/audbookdl
go test -v ./internal/source/...
```

Expected: FAIL — types and methods not defined yet.

- [ ] **Step 3: Create `internal/source/source.go` with the Source interface**

```go
package source

import "context"

// Source defines the contract all audiobook sources must implement.
type Source interface {
	// Search queries the source for audiobooks matching the query.
	Search(ctx context.Context, query string, opts SearchOptions) ([]*Audiobook, error)

	// GetChapters fetches the chapter list for an audiobook by its source-specific ID.
	GetChapters(ctx context.Context, bookID string) ([]*Chapter, error)

	// Name returns the source identifier (e.g., "librivox", "archive").
	Name() string
}

// SearchOptions configures search behavior.
type SearchOptions struct {
	Limit    int
	Page     int
	Language string
	Author   string
	Format   string
}
```

- [ ] **Step 4: Create `internal/source/types.go` with domain types**

```go
package source

import (
	"fmt"
	"time"
)

// Audiobook represents an audiobook from any source.
type Audiobook struct {
	ID           string
	Title        string
	Author       string
	Narrator     string
	Description  string
	Language     string
	Year         string
	Duration     time.Duration
	CoverURL     string
	PageURL      string
	Format       string // mp3, m4b, ogg
	TotalSize    int64
	ChapterCount int
	Source       string // "librivox", "archive", "loyalbooks", "openlibrary"
}

// DurationFormatted returns a human-readable duration string.
func (a *Audiobook) DurationFormatted() string {
	h := int(a.Duration.Hours())
	m := int(a.Duration.Minutes()) % 60
	if h == 0 {
		return fmt.Sprintf("%dm", m)
	}
	return fmt.Sprintf("%dh %dm", h, m)
}

// SizeFormatted returns a human-readable file size string.
func (a *Audiobook) SizeFormatted() string {
	const (
		KB = 1024
		MB = 1024 * KB
		GB = 1024 * MB
	)
	switch {
	case a.TotalSize >= GB:
		return fmt.Sprintf("%.1f GB", float64(a.TotalSize)/float64(GB))
	case a.TotalSize >= MB:
		return fmt.Sprintf("%.1f MB", float64(a.TotalSize)/float64(MB))
	case a.TotalSize >= KB:
		return fmt.Sprintf("%.1f KB", float64(a.TotalSize)/float64(KB))
	default:
		return fmt.Sprintf("%d B", a.TotalSize)
	}
}

// Chapter represents a single chapter/track in an audiobook.
type Chapter struct {
	Index       int
	Title       string
	Duration    time.Duration
	DownloadURL string
	Format      string
	FileSize    int64
}

// FilenamePrefix returns the formatted prefix for saving this chapter to disk.
func (c *Chapter) FilenamePrefix() string {
	return fmt.Sprintf("%02d - %s", c.Index, c.Title)
}
```

- [ ] **Step 5: Run tests to verify they pass**

```bash
cd ~/Documents/personal/audbookdl
go test -v ./internal/source/...
```

Expected: PASS — all 3 test functions pass.

- [ ] **Step 6: Commit**

```bash
git add internal/source/
git commit -m "feat: add Source interface and domain types (Audiobook, Chapter)"
```

---

### Task 3: Configuration Package

**Files:**
- Create: `internal/config/config.go`
- Test: `internal/config/config_test.go`

- [ ] **Step 1: Write the test**

Create file `internal/config/config_test.go`:

```go
package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDefaults(t *testing.T) {
	// Init with no config file
	if err := Init(""); err != nil {
		t.Fatalf("Init() error: %v", err)
	}

	cfg := Get()

	if cfg.Download.Directory != "~/Audiobooks" {
		t.Errorf("Download.Directory = %q, want %q", cfg.Download.Directory, "~/Audiobooks")
	}
	if cfg.Download.ChunkSize != 5*1024*1024 {
		t.Errorf("Download.ChunkSize = %d, want %d", cfg.Download.ChunkSize, 5*1024*1024)
	}
	if cfg.Download.MaxConcurrent != 3 {
		t.Errorf("Download.MaxConcurrent = %d, want %d", cfg.Download.MaxConcurrent, 3)
	}
	if cfg.Download.PreferredFormat != "mp3" {
		t.Errorf("Download.PreferredFormat = %q, want %q", cfg.Download.PreferredFormat, "mp3")
	}
	if cfg.Player.DefaultSpeed != 1.0 {
		t.Errorf("Player.DefaultSpeed = %f, want %f", cfg.Player.DefaultSpeed, 1.0)
	}
	if cfg.Player.SkipSeconds != 15 {
		t.Errorf("Player.SkipSeconds = %d, want %d", cfg.Player.SkipSeconds, 15)
	}
	if cfg.Search.DefaultLimit != 10 {
		t.Errorf("Search.DefaultLimit = %d, want %d", cfg.Search.DefaultLimit, 10)
	}
	if cfg.Search.CacheTTL != time.Hour {
		t.Errorf("Search.CacheTTL = %v, want %v", cfg.Search.CacheTTL, time.Hour)
	}
	if len(cfg.Search.Sources) != 4 {
		t.Errorf("Search.Sources length = %d, want 4", len(cfg.Search.Sources))
	}
}

func TestGetConfigDir(t *testing.T) {
	dir := GetConfigDir()
	home, _ := os.UserHomeDir()
	want := filepath.Join(home, ".config", "audbookdl")
	if dir != want {
		t.Errorf("GetConfigDir() = %q, want %q", dir, want)
	}
}

func TestGetDBPath(t *testing.T) {
	path := GetDBPath()
	home, _ := os.UserHomeDir()
	want := filepath.Join(home, ".config", "audbookdl", "audbookdl.db")
	if path != want {
		t.Errorf("GetDBPath() = %q, want %q", path, want)
	}
}

func TestExpandPath(t *testing.T) {
	home, _ := os.UserHomeDir()

	tests := []struct {
		name string
		path string
		want string
	}{
		{"tilde prefix", "~/Audiobooks", filepath.Join(home, "Audiobooks")},
		{"absolute path", "/tmp/audiobooks", "/tmp/audiobooks"},
		{"relative path", "audiobooks", "audiobooks"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := expandPath(tt.path); got != tt.want {
				t.Errorf("expandPath(%q) = %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd ~/Documents/personal/audbookdl
go test -v ./internal/config/...
```

Expected: FAIL — package doesn't exist yet.

- [ ] **Step 3: Create `internal/config/config.go`**

```go
package config

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/viper"
)

// Config holds all application configuration.
type Config struct {
	Download      DownloadConfig      `mapstructure:"download"`
	Player        PlayerConfig        `mapstructure:"player"`
	Search        SearchConfig        `mapstructure:"search"`
	Network       NetworkConfig       `mapstructure:"network"`
	Notifications NotificationsConfig `mapstructure:"notifications"`
}

// DownloadConfig holds download settings.
type DownloadConfig struct {
	Directory       string `mapstructure:"directory"`
	ChunkSize       int64  `mapstructure:"chunk_size"`
	MaxConcurrent   int    `mapstructure:"max_concurrent"`
	PreferredFormat string `mapstructure:"preferred_format"`
}

// PlayerConfig holds audio player settings.
type PlayerConfig struct {
	DefaultSpeed      float64 `mapstructure:"default_speed"`
	SkipSeconds       int     `mapstructure:"skip_seconds"`
	SleepTimerMinutes int     `mapstructure:"sleep_timer_minutes"`
}

// SearchConfig holds search settings.
type SearchConfig struct {
	DefaultLimit int           `mapstructure:"default_limit"`
	CacheTTL     time.Duration `mapstructure:"cache_ttl"`
	Sources      []string      `mapstructure:"sources"`
}

// NetworkConfig holds network settings.
type NetworkConfig struct {
	Timeout       time.Duration `mapstructure:"timeout"`
	RetryAttempts int           `mapstructure:"retry_attempts"`
	UserAgent     string        `mapstructure:"user_agent"`
}

// NotificationsConfig holds notification settings.
type NotificationsConfig struct {
	Enabled bool `mapstructure:"enabled"`
	Sound   bool `mapstructure:"sound"`
}

var cfg *Config

// GetConfigDir returns the configuration directory path.
func GetConfigDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "audbookdl")
}

// GetDBPath returns the database file path.
func GetDBPath() string {
	return filepath.Join(GetConfigDir(), "audbookdl.db")
}

// GetConfigPath returns the config file path.
func GetConfigPath() string {
	return filepath.Join(GetConfigDir(), "config.yaml")
}

// Init initializes the configuration.
func Init(cfgFile string) error {
	viper.SetDefault("download.directory", "~/Audiobooks")
	viper.SetDefault("download.chunk_size", 5*1024*1024)
	viper.SetDefault("download.max_concurrent", 3)
	viper.SetDefault("download.preferred_format", "mp3")

	viper.SetDefault("player.default_speed", 1.0)
	viper.SetDefault("player.skip_seconds", 15)
	viper.SetDefault("player.sleep_timer_minutes", 0)

	viper.SetDefault("search.default_limit", 10)
	viper.SetDefault("search.cache_ttl", time.Hour)
	viper.SetDefault("search.sources", []string{"librivox", "archive", "loyalbooks", "openlibrary"})

	viper.SetDefault("network.timeout", 30*time.Second)
	viper.SetDefault("network.retry_attempts", 5)
	viper.SetDefault("network.user_agent", "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")

	viper.SetDefault("notifications.enabled", true)
	viper.SetDefault("notifications.sound", true)

	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		viper.SetConfigName("config")
		viper.SetConfigType("yaml")
		viper.AddConfigPath(GetConfigDir())
	}

	viper.SetEnvPrefix("AUDBOOKDL")
	viper.AutomaticEnv()
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	// Read config file (ignore if not found)
	_ = viper.ReadInConfig()

	// Reset cached config
	cfg = nil

	return nil
}

// Get returns the current configuration.
func Get() *Config {
	if cfg == nil {
		cfg = &Config{}
		viper.Unmarshal(cfg)
		cfg.Download.Directory = expandPath(cfg.Download.Directory)
	}
	return cfg
}

// Set sets a configuration value and writes to disk.
func Set(key, value string) error {
	viper.Set(key, value)

	configDir := GetConfigDir()
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return err
	}

	cfg = nil

	return viper.WriteConfigAs(GetConfigPath())
}

// GetValue retrieves a configuration value.
func GetValue(key string) interface{} {
	return viper.Get(key)
}

func expandPath(path string) string {
	if strings.HasPrefix(path, "~/") {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, path[2:])
	}
	return path
}
```

- [ ] **Step 4: Fetch Viper dependency and run tests**

```bash
cd ~/Documents/personal/audbookdl
go get github.com/spf13/viper
go mod tidy
go test -v ./internal/config/...
```

Expected: PASS — all config tests pass.

- [ ] **Step 5: Commit**

```bash
git add internal/config/ go.mod go.sum
git commit -m "feat: add Viper-based configuration with audiobook defaults"
```

---

### Task 4: Database Package — Schema and Connection

**Files:**
- Create: `internal/db/db.go`
- Test: `internal/db/db_test.go`

- [ ] **Step 1: Write the test**

Create file `internal/db/db_test.go`:

```go
package db

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"
)

func setupTestDB(t *testing.T) *sql.DB {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	db, err := InitWithPath(dbPath)
	if err != nil {
		t.Fatalf("InitWithPath() error: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func TestInitWithPath_CreatesTablesAndIndexes(t *testing.T) {
	db := setupTestDB(t)

	// Verify all expected tables exist
	tables := []string{
		"audiobook_downloads",
		"chapter_downloads",
		"chunks",
		"bookmarks",
		"playback_state",
		"search_history",
		"search_cache",
	}

	for _, table := range tables {
		var name string
		err := db.QueryRow(
			"SELECT name FROM sqlite_master WHERE type='table' AND name=?", table,
		).Scan(&name)
		if err != nil {
			t.Errorf("table %q not found: %v", table, err)
		}
	}
}

func TestInitWithPath_WALMode(t *testing.T) {
	db := setupTestDB(t)

	var mode string
	if err := db.QueryRow("PRAGMA journal_mode").Scan(&mode); err != nil {
		t.Fatalf("PRAGMA journal_mode error: %v", err)
	}
	if mode != "wal" {
		t.Errorf("journal_mode = %q, want %q", mode, "wal")
	}
}

func TestInitWithPath_ForeignKeysEnabled(t *testing.T) {
	db := setupTestDB(t)

	var fk int
	if err := db.QueryRow("PRAGMA foreign_keys").Scan(&fk); err != nil {
		t.Fatalf("PRAGMA foreign_keys error: %v", err)
	}
	if fk != 1 {
		t.Errorf("foreign_keys = %d, want 1", fk)
	}
}

func TestInitWithPath_CascadeDelete(t *testing.T) {
	db := setupTestDB(t)

	// Insert a download
	res, err := db.Exec(`
		INSERT INTO audiobook_downloads (audiobook_id, title, author, source, base_path)
		VALUES ('test-id', 'Test Book', 'Test Author', 'librivox', '/tmp/test')
	`)
	if err != nil {
		t.Fatalf("insert download: %v", err)
	}
	dlID, _ := res.LastInsertId()

	// Insert a chapter
	chRes, err := db.Exec(`
		INSERT INTO chapter_downloads (download_id, chapter_index, title, file_path)
		VALUES (?, 1, 'Chapter 1', '/tmp/test/ch1.mp3')
	`, dlID)
	if err != nil {
		t.Fatalf("insert chapter: %v", err)
	}
	chID, _ := chRes.LastInsertId()

	// Insert a chunk
	_, err = db.Exec(`
		INSERT INTO chunks (chapter_download_id, chunk_index, start_byte, end_byte)
		VALUES (?, 0, 0, 1024)
	`, chID)
	if err != nil {
		t.Fatalf("insert chunk: %v", err)
	}

	// Delete the download — should cascade
	_, err = db.Exec("DELETE FROM audiobook_downloads WHERE id = ?", dlID)
	if err != nil {
		t.Fatalf("delete download: %v", err)
	}

	// Verify chapters are gone
	var chCount int
	db.QueryRow("SELECT COUNT(*) FROM chapter_downloads WHERE download_id = ?", dlID).Scan(&chCount)
	if chCount != 0 {
		t.Errorf("chapter_downloads count = %d after cascade delete, want 0", chCount)
	}

	// Verify chunks are gone
	var ckCount int
	db.QueryRow("SELECT COUNT(*) FROM chunks WHERE chapter_download_id = ?", chID).Scan(&ckCount)
	if ckCount != 0 {
		t.Errorf("chunks count = %d after cascade delete, want 0", ckCount)
	}
}

func TestInitWithPath_FileCreated(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "sub", "test.db")
	db, err := InitWithPath(dbPath)
	if err != nil {
		t.Fatalf("InitWithPath() error: %v", err)
	}
	defer db.Close()

	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Error("database file was not created")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd ~/Documents/personal/audbookdl
go test -v ./internal/db/...
```

Expected: FAIL — package doesn't exist.

- [ ] **Step 3: Create `internal/db/db.go`**

```go
package db

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	"github.com/billmal071/audbookdl/internal/config"
	_ "modernc.org/sqlite"
)

var database *sql.DB

const schema = `
CREATE TABLE IF NOT EXISTS audiobook_downloads (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    audiobook_id    TEXT NOT NULL,
    title           TEXT NOT NULL,
    author          TEXT NOT NULL,
    narrator        TEXT DEFAULT '',
    source          TEXT NOT NULL,
    status          TEXT NOT NULL DEFAULT 'pending',
    base_path       TEXT NOT NULL,
    total_size      INTEGER DEFAULT 0,
    downloaded_size INTEGER DEFAULT 0,
    created_at      DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at      DATETIME DEFAULT CURRENT_TIMESTAMP,
    completed_at    DATETIME
);

CREATE INDEX IF NOT EXISTS idx_audiobook_downloads_status ON audiobook_downloads(status);
CREATE INDEX IF NOT EXISTS idx_audiobook_downloads_audiobook_id ON audiobook_downloads(audiobook_id);

CREATE TABLE IF NOT EXISTS chapter_downloads (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    download_id     INTEGER NOT NULL,
    chapter_index   INTEGER NOT NULL,
    title           TEXT NOT NULL,
    file_path       TEXT NOT NULL,
    file_size       INTEGER DEFAULT 0,
    downloaded      INTEGER DEFAULT 0,
    status          TEXT NOT NULL DEFAULT 'pending',
    FOREIGN KEY (download_id) REFERENCES audiobook_downloads(id) ON DELETE CASCADE,
    UNIQUE(download_id, chapter_index)
);

CREATE INDEX IF NOT EXISTS idx_chapter_downloads_download ON chapter_downloads(download_id);

CREATE TABLE IF NOT EXISTS chunks (
    id                  INTEGER PRIMARY KEY AUTOINCREMENT,
    chapter_download_id INTEGER NOT NULL,
    chunk_index         INTEGER NOT NULL,
    start_byte          INTEGER NOT NULL,
    end_byte            INTEGER NOT NULL,
    downloaded          INTEGER DEFAULT 0,
    status              TEXT NOT NULL DEFAULT 'pending',
    FOREIGN KEY (chapter_download_id) REFERENCES chapter_downloads(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_chunks_chapter ON chunks(chapter_download_id);

CREATE TABLE IF NOT EXISTS bookmarks (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    audiobook_id    TEXT NOT NULL,
    title           TEXT NOT NULL,
    author          TEXT NOT NULL,
    narrator        TEXT DEFAULT '',
    source          TEXT NOT NULL,
    page_url        TEXT,
    note            TEXT DEFAULT '',
    created_at      DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_bookmarks_audiobook_id ON bookmarks(audiobook_id);

CREATE TABLE IF NOT EXISTS playback_state (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    audiobook_id    TEXT NOT NULL UNIQUE,
    chapter_index   INTEGER NOT NULL DEFAULT 0,
    position_ms     INTEGER NOT NULL DEFAULT 0,
    playback_speed  REAL NOT NULL DEFAULT 1.0,
    updated_at      DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS search_history (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    query           TEXT NOT NULL,
    source          TEXT DEFAULT '',
    result_count    INTEGER DEFAULT 0,
    created_at      DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_search_history_created ON search_history(created_at DESC);

CREATE TABLE IF NOT EXISTS search_cache (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    cache_key       TEXT UNIQUE NOT NULL,
    results         BLOB NOT NULL,
    expires_at      DATETIME NOT NULL,
    created_at      DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_search_cache_key ON search_cache(cache_key);
CREATE INDEX IF NOT EXISTS idx_search_cache_expires ON search_cache(expires_at);
`

// InitWithPath initializes the database at the given path. Returns the db for testing.
func InitWithPath(dbPath string) (*sql.DB, error) {
	if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
		return nil, fmt.Errorf("create db directory: %w", err)
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	if _, err := db.Exec("PRAGMA foreign_keys = ON"); err != nil {
		return nil, fmt.Errorf("enable foreign keys: %w", err)
	}

	if _, err := db.Exec("PRAGMA busy_timeout = 10000"); err != nil {
		return nil, fmt.Errorf("set busy timeout: %w", err)
	}

	if _, err := db.Exec("PRAGMA journal_mode = WAL"); err != nil {
		return nil, fmt.Errorf("set WAL mode: %w", err)
	}

	if _, err := db.Exec("PRAGMA synchronous = NORMAL"); err != nil {
		return nil, fmt.Errorf("set synchronous: %w", err)
	}

	if _, err := db.Exec(schema); err != nil {
		return nil, fmt.Errorf("create schema: %w", err)
	}

	return db, nil
}

// Init initializes the database using the configured path.
func Init() error {
	dbPath := config.GetDBPath()
	db, err := InitWithPath(dbPath)
	if err != nil {
		return err
	}
	database = db
	return nil
}

// DB returns the database connection.
func DB() *sql.DB {
	return database
}

// Close closes the database connection.
func Close() error {
	if database != nil {
		return database.Close()
	}
	return nil
}
```

- [ ] **Step 4: Fetch sqlite dependency and run tests**

```bash
cd ~/Documents/personal/audbookdl
go get modernc.org/sqlite
go mod tidy
go test -v ./internal/db/...
```

Expected: PASS — all 5 test functions pass.

- [ ] **Step 5: Commit**

```bash
git add internal/db/ go.mod go.sum
git commit -m "feat: add SQLite database with audiobook schema and WAL mode"
```

---

### Task 5: Database CRUD — Downloads

**Files:**
- Create: `internal/db/models.go`
- Create: `internal/db/downloads.go`
- Test: `internal/db/downloads_test.go`

- [ ] **Step 1: Create `internal/db/models.go` with status constants and model types**

```go
package db

import "time"

// DownloadStatus represents the state of a download.
type DownloadStatus string

const (
	StatusPending     DownloadStatus = "pending"
	StatusDownloading DownloadStatus = "downloading"
	StatusCompleted   DownloadStatus = "completed"
	StatusFailed      DownloadStatus = "failed"
	StatusPaused      DownloadStatus = "paused"
)

// AudiobookDownload represents a tracked audiobook download.
type AudiobookDownload struct {
	ID             int64
	AudiobookID    string
	Title          string
	Author         string
	Narrator       string
	Source         string
	Status         DownloadStatus
	BasePath       string
	TotalSize      int64
	DownloadedSize int64
	CreatedAt      time.Time
	UpdatedAt      time.Time
	CompletedAt    *time.Time
}

// ChapterDownload represents a tracked chapter download within an audiobook.
type ChapterDownload struct {
	ID           int64
	DownloadID   int64
	ChapterIndex int
	Title        string
	FilePath     string
	FileSize     int64
	Downloaded   int64
	Status       DownloadStatus
}

// Chunk represents a download chunk for resumable downloads.
type Chunk struct {
	ID                int64
	ChapterDownloadID int64
	ChunkIndex        int
	StartByte         int64
	EndByte           int64
	Downloaded        int64
	Status            DownloadStatus
}
```

- [ ] **Step 2: Write the downloads test**

Create file `internal/db/downloads_test.go`:

```go
package db

import (
	"testing"
)

func TestCreateAndGetDownload(t *testing.T) {
	dbConn := setupTestDB(t)

	dl := &AudiobookDownload{
		AudiobookID: "lv-12345",
		Title:       "The Adventures of Sherlock Holmes",
		Author:      "Arthur Conan Doyle",
		Narrator:    "Mark Nelson",
		Source:      "librivox",
		BasePath:    "/tmp/audiobooks/Doyle/Sherlock",
		TotalSize:   230686720,
	}

	id, err := CreateDownload(dbConn, dl)
	if err != nil {
		t.Fatalf("CreateDownload() error: %v", err)
	}
	if id == 0 {
		t.Fatal("CreateDownload() returned id 0")
	}

	got, err := GetDownload(dbConn, id)
	if err != nil {
		t.Fatalf("GetDownload() error: %v", err)
	}

	if got.AudiobookID != "lv-12345" {
		t.Errorf("AudiobookID = %q, want %q", got.AudiobookID, "lv-12345")
	}
	if got.Title != "The Adventures of Sherlock Holmes" {
		t.Errorf("Title = %q, want %q", got.Title, "The Adventures of Sherlock Holmes")
	}
	if got.Status != StatusPending {
		t.Errorf("Status = %q, want %q", got.Status, StatusPending)
	}
	if got.TotalSize != 230686720 {
		t.Errorf("TotalSize = %d, want %d", got.TotalSize, 230686720)
	}
}

func TestUpdateDownloadStatus(t *testing.T) {
	dbConn := setupTestDB(t)

	id, err := CreateDownload(dbConn, &AudiobookDownload{
		AudiobookID: "test-1",
		Title:       "Test",
		Author:      "Author",
		Source:      "archive",
		BasePath:    "/tmp/test",
	})
	if err != nil {
		t.Fatalf("CreateDownload() error: %v", err)
	}

	if err := UpdateDownloadStatus(dbConn, id, StatusDownloading); err != nil {
		t.Fatalf("UpdateDownloadStatus() error: %v", err)
	}

	got, _ := GetDownload(dbConn, id)
	if got.Status != StatusDownloading {
		t.Errorf("Status = %q, want %q", got.Status, StatusDownloading)
	}
}

func TestListDownloads(t *testing.T) {
	dbConn := setupTestDB(t)

	for i, title := range []string{"Book A", "Book B", "Book C"} {
		_, err := CreateDownload(dbConn, &AudiobookDownload{
			AudiobookID: "id-" + string(rune('a'+i)),
			Title:       title,
			Author:      "Author",
			Source:      "librivox",
			BasePath:    "/tmp/" + title,
		})
		if err != nil {
			t.Fatalf("CreateDownload(%q) error: %v", title, err)
		}
	}

	downloads, err := ListDownloads(dbConn)
	if err != nil {
		t.Fatalf("ListDownloads() error: %v", err)
	}
	if len(downloads) != 3 {
		t.Errorf("ListDownloads() returned %d, want 3", len(downloads))
	}
}

func TestCreateAndListChapters(t *testing.T) {
	dbConn := setupTestDB(t)

	dlID, _ := CreateDownload(dbConn, &AudiobookDownload{
		AudiobookID: "test-ch",
		Title:       "Test",
		Author:      "Author",
		Source:      "librivox",
		BasePath:    "/tmp/test",
	})

	chapters := []*ChapterDownload{
		{DownloadID: dlID, ChapterIndex: 1, Title: "Chapter 1", FilePath: "/tmp/ch1.mp3", FileSize: 5000000},
		{DownloadID: dlID, ChapterIndex: 2, Title: "Chapter 2", FilePath: "/tmp/ch2.mp3", FileSize: 6000000},
	}

	for _, ch := range chapters {
		if _, err := CreateChapterDownload(dbConn, ch); err != nil {
			t.Fatalf("CreateChapterDownload() error: %v", err)
		}
	}

	got, err := ListChapterDownloads(dbConn, dlID)
	if err != nil {
		t.Fatalf("ListChapterDownloads() error: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("ListChapterDownloads() returned %d, want 2", len(got))
	}
	if got[0].Title != "Chapter 1" {
		t.Errorf("first chapter Title = %q, want %q", got[0].Title, "Chapter 1")
	}
}

func TestUpdateDownloadProgress(t *testing.T) {
	dbConn := setupTestDB(t)

	dlID, _ := CreateDownload(dbConn, &AudiobookDownload{
		AudiobookID: "test-prog",
		Title:       "Test",
		Author:      "Author",
		Source:      "librivox",
		BasePath:    "/tmp/test",
		TotalSize:   10000000,
	})

	if err := UpdateDownloadProgress(dbConn, dlID, 5000000); err != nil {
		t.Fatalf("UpdateDownloadProgress() error: %v", err)
	}

	got, _ := GetDownload(dbConn, dlID)
	if got.DownloadedSize != 5000000 {
		t.Errorf("DownloadedSize = %d, want %d", got.DownloadedSize, 5000000)
	}
}
```

- [ ] **Step 3: Run tests to verify they fail**

```bash
cd ~/Documents/personal/audbookdl
go test -v ./internal/db/...
```

Expected: FAIL — CRUD functions not defined.

- [ ] **Step 4: Create `internal/db/downloads.go`**

```go
package db

import (
	"database/sql"
	"fmt"
	"time"
)

// CreateDownload inserts a new audiobook download record.
func CreateDownload(db *sql.DB, dl *AudiobookDownload) (int64, error) {
	res, err := db.Exec(`
		INSERT INTO audiobook_downloads (audiobook_id, title, author, narrator, source, base_path, total_size)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, dl.AudiobookID, dl.Title, dl.Author, dl.Narrator, dl.Source, dl.BasePath, dl.TotalSize)
	if err != nil {
		return 0, fmt.Errorf("insert download: %w", err)
	}
	return res.LastInsertId()
}

// GetDownload retrieves a download by ID.
func GetDownload(db *sql.DB, id int64) (*AudiobookDownload, error) {
	dl := &AudiobookDownload{}
	err := db.QueryRow(`
		SELECT id, audiobook_id, title, author, narrator, source, status, base_path,
		       total_size, downloaded_size, created_at, updated_at, completed_at
		FROM audiobook_downloads WHERE id = ?
	`, id).Scan(
		&dl.ID, &dl.AudiobookID, &dl.Title, &dl.Author, &dl.Narrator, &dl.Source,
		&dl.Status, &dl.BasePath, &dl.TotalSize, &dl.DownloadedSize,
		&dl.CreatedAt, &dl.UpdatedAt, &dl.CompletedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("get download: %w", err)
	}
	return dl, nil
}

// ListDownloads returns all audiobook downloads ordered by creation time.
func ListDownloads(db *sql.DB) ([]*AudiobookDownload, error) {
	rows, err := db.Query(`
		SELECT id, audiobook_id, title, author, narrator, source, status, base_path,
		       total_size, downloaded_size, created_at, updated_at, completed_at
		FROM audiobook_downloads ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, fmt.Errorf("list downloads: %w", err)
	}
	defer rows.Close()

	var downloads []*AudiobookDownload
	for rows.Next() {
		dl := &AudiobookDownload{}
		if err := rows.Scan(
			&dl.ID, &dl.AudiobookID, &dl.Title, &dl.Author, &dl.Narrator, &dl.Source,
			&dl.Status, &dl.BasePath, &dl.TotalSize, &dl.DownloadedSize,
			&dl.CreatedAt, &dl.UpdatedAt, &dl.CompletedAt,
		); err != nil {
			return nil, fmt.Errorf("scan download: %w", err)
		}
		downloads = append(downloads, dl)
	}
	return downloads, rows.Err()
}

// UpdateDownloadStatus changes the status of a download.
func UpdateDownloadStatus(db *sql.DB, id int64, status DownloadStatus) error {
	query := "UPDATE audiobook_downloads SET status = ?, updated_at = ? WHERE id = ?"
	args := []interface{}{status, time.Now(), id}

	if status == StatusCompleted {
		query = "UPDATE audiobook_downloads SET status = ?, updated_at = ?, completed_at = ? WHERE id = ?"
		now := time.Now()
		args = []interface{}{status, now, now, id}
	}

	_, err := db.Exec(query, args...)
	if err != nil {
		return fmt.Errorf("update download status: %w", err)
	}
	return nil
}

// UpdateDownloadProgress updates the downloaded size.
func UpdateDownloadProgress(db *sql.DB, id int64, downloadedSize int64) error {
	_, err := db.Exec(
		"UPDATE audiobook_downloads SET downloaded_size = ?, updated_at = ? WHERE id = ?",
		downloadedSize, time.Now(), id,
	)
	if err != nil {
		return fmt.Errorf("update download progress: %w", err)
	}
	return nil
}

// CreateChapterDownload inserts a chapter download record.
func CreateChapterDownload(db *sql.DB, ch *ChapterDownload) (int64, error) {
	res, err := db.Exec(`
		INSERT INTO chapter_downloads (download_id, chapter_index, title, file_path, file_size)
		VALUES (?, ?, ?, ?, ?)
	`, ch.DownloadID, ch.ChapterIndex, ch.Title, ch.FilePath, ch.FileSize)
	if err != nil {
		return 0, fmt.Errorf("insert chapter download: %w", err)
	}
	return res.LastInsertId()
}

// ListChapterDownloads returns all chapters for a given download, ordered by index.
func ListChapterDownloads(db *sql.DB, downloadID int64) ([]*ChapterDownload, error) {
	rows, err := db.Query(`
		SELECT id, download_id, chapter_index, title, file_path, file_size, downloaded, status
		FROM chapter_downloads WHERE download_id = ? ORDER BY chapter_index
	`, downloadID)
	if err != nil {
		return nil, fmt.Errorf("list chapter downloads: %w", err)
	}
	defer rows.Close()

	var chapters []*ChapterDownload
	for rows.Next() {
		ch := &ChapterDownload{}
		if err := rows.Scan(
			&ch.ID, &ch.DownloadID, &ch.ChapterIndex, &ch.Title,
			&ch.FilePath, &ch.FileSize, &ch.Downloaded, &ch.Status,
		); err != nil {
			return nil, fmt.Errorf("scan chapter download: %w", err)
		}
		chapters = append(chapters, ch)
	}
	return chapters, rows.Err()
}

// UpdateChapterStatus changes the status of a chapter download.
func UpdateChapterStatus(db *sql.DB, id int64, status DownloadStatus) error {
	_, err := db.Exec(
		"UPDATE chapter_downloads SET status = ? WHERE id = ?",
		status, id,
	)
	if err != nil {
		return fmt.Errorf("update chapter status: %w", err)
	}
	return nil
}

// UpdateChapterProgress updates the downloaded bytes for a chapter.
func UpdateChapterProgress(db *sql.DB, id int64, downloaded int64) error {
	_, err := db.Exec(
		"UPDATE chapter_downloads SET downloaded = ? WHERE id = ?",
		downloaded, id,
	)
	if err != nil {
		return fmt.Errorf("update chapter progress: %w", err)
	}
	return nil
}
```

- [ ] **Step 5: Run tests**

```bash
cd ~/Documents/personal/audbookdl
go test -v ./internal/db/...
```

Expected: PASS — all download CRUD tests pass.

- [ ] **Step 6: Commit**

```bash
git add internal/db/models.go internal/db/downloads.go internal/db/downloads_test.go
git commit -m "feat: add download and chapter CRUD operations"
```

---

### Task 6: Database CRUD — Bookmarks, Playback State, Search History

**Files:**
- Create: `internal/db/bookmarks.go`
- Create: `internal/db/playback.go`
- Create: `internal/db/history.go`
- Test: `internal/db/bookmarks_test.go`
- Test: `internal/db/playback_test.go`
- Test: `internal/db/history_test.go`

- [ ] **Step 1: Write the bookmarks test**

Create file `internal/db/bookmarks_test.go`:

```go
package db

import (
	"testing"
)

func TestCreateAndListBookmarks(t *testing.T) {
	dbConn := setupTestDB(t)

	bm := &Bookmark{
		AudiobookID: "lv-123",
		Title:       "Sherlock Holmes",
		Author:      "Arthur Conan Doyle",
		Narrator:    "Mark Nelson",
		Source:      "librivox",
		PageURL:     "https://librivox.org/123",
		Note:        "Great narration",
	}

	id, err := CreateBookmark(dbConn, bm)
	if err != nil {
		t.Fatalf("CreateBookmark() error: %v", err)
	}
	if id == 0 {
		t.Fatal("CreateBookmark() returned id 0")
	}

	bookmarks, err := ListBookmarks(dbConn)
	if err != nil {
		t.Fatalf("ListBookmarks() error: %v", err)
	}
	if len(bookmarks) != 1 {
		t.Fatalf("ListBookmarks() returned %d, want 1", len(bookmarks))
	}
	if bookmarks[0].Title != "Sherlock Holmes" {
		t.Errorf("Title = %q, want %q", bookmarks[0].Title, "Sherlock Holmes")
	}
}

func TestDeleteBookmark(t *testing.T) {
	dbConn := setupTestDB(t)

	id, _ := CreateBookmark(dbConn, &Bookmark{
		AudiobookID: "del-1",
		Title:       "Delete Me",
		Author:      "Author",
		Source:      "archive",
	})

	if err := DeleteBookmark(dbConn, id); err != nil {
		t.Fatalf("DeleteBookmark() error: %v", err)
	}

	bookmarks, _ := ListBookmarks(dbConn)
	if len(bookmarks) != 0 {
		t.Errorf("ListBookmarks() returned %d after delete, want 0", len(bookmarks))
	}
}
```

- [ ] **Step 2: Write the playback state test**

Create file `internal/db/playback_test.go`:

```go
package db

import (
	"testing"
)

func TestSaveAndGetPlaybackState(t *testing.T) {
	dbConn := setupTestDB(t)

	state := &PlaybackState{
		AudiobookID:   "lv-123",
		ChapterIndex:  5,
		PositionMS:    123456,
		PlaybackSpeed: 1.5,
	}

	if err := SavePlaybackState(dbConn, state); err != nil {
		t.Fatalf("SavePlaybackState() error: %v", err)
	}

	got, err := GetPlaybackState(dbConn, "lv-123")
	if err != nil {
		t.Fatalf("GetPlaybackState() error: %v", err)
	}
	if got.ChapterIndex != 5 {
		t.Errorf("ChapterIndex = %d, want 5", got.ChapterIndex)
	}
	if got.PositionMS != 123456 {
		t.Errorf("PositionMS = %d, want 123456", got.PositionMS)
	}
	if got.PlaybackSpeed != 1.5 {
		t.Errorf("PlaybackSpeed = %f, want 1.5", got.PlaybackSpeed)
	}
}

func TestSavePlaybackState_Upsert(t *testing.T) {
	dbConn := setupTestDB(t)

	// First save
	SavePlaybackState(dbConn, &PlaybackState{
		AudiobookID:   "upsert-1",
		ChapterIndex:  1,
		PositionMS:    1000,
		PlaybackSpeed: 1.0,
	})

	// Update same audiobook
	SavePlaybackState(dbConn, &PlaybackState{
		AudiobookID:   "upsert-1",
		ChapterIndex:  3,
		PositionMS:    50000,
		PlaybackSpeed: 2.0,
	})

	got, _ := GetPlaybackState(dbConn, "upsert-1")
	if got.ChapterIndex != 3 {
		t.Errorf("ChapterIndex = %d, want 3 after upsert", got.ChapterIndex)
	}
	if got.PositionMS != 50000 {
		t.Errorf("PositionMS = %d, want 50000 after upsert", got.PositionMS)
	}
}

func TestGetPlaybackState_NotFound(t *testing.T) {
	dbConn := setupTestDB(t)

	_, err := GetPlaybackState(dbConn, "nonexistent")
	if err == nil {
		t.Error("GetPlaybackState() expected error for nonexistent, got nil")
	}
}
```

- [ ] **Step 3: Write the search history test**

Create file `internal/db/history_test.go`:

```go
package db

import (
	"testing"
)

func TestAddAndListSearchHistory(t *testing.T) {
	dbConn := setupTestDB(t)

	if err := AddSearchHistory(dbConn, "sherlock holmes", "librivox", 12); err != nil {
		t.Fatalf("AddSearchHistory() error: %v", err)
	}
	if err := AddSearchHistory(dbConn, "war and peace", "", 5); err != nil {
		t.Fatalf("AddSearchHistory() error: %v", err)
	}

	history, err := ListSearchHistory(dbConn, 10)
	if err != nil {
		t.Fatalf("ListSearchHistory() error: %v", err)
	}
	if len(history) != 2 {
		t.Fatalf("ListSearchHistory() returned %d, want 2", len(history))
	}
	// Most recent first
	if history[0].Query != "war and peace" {
		t.Errorf("first entry Query = %q, want %q", history[0].Query, "war and peace")
	}
}

func TestClearSearchHistory(t *testing.T) {
	dbConn := setupTestDB(t)

	AddSearchHistory(dbConn, "query1", "", 1)
	AddSearchHistory(dbConn, "query2", "", 2)

	if err := ClearSearchHistory(dbConn); err != nil {
		t.Fatalf("ClearSearchHistory() error: %v", err)
	}

	history, _ := ListSearchHistory(dbConn, 10)
	if len(history) != 0 {
		t.Errorf("ListSearchHistory() returned %d after clear, want 0", len(history))
	}
}
```

- [ ] **Step 4: Run tests to verify they fail**

```bash
cd ~/Documents/personal/audbookdl
go test -v ./internal/db/...
```

Expected: FAIL — Bookmark, PlaybackState types and CRUD functions not defined.

- [ ] **Step 5: Add model types to `internal/db/models.go`**

Append to `internal/db/models.go`:

```go
// Bookmark represents a saved audiobook.
type Bookmark struct {
	ID          int64
	AudiobookID string
	Title       string
	Author      string
	Narrator    string
	Source      string
	PageURL     string
	Note        string
	CreatedAt   time.Time
}

// PlaybackState tracks where the user left off in an audiobook.
type PlaybackState struct {
	ID            int64
	AudiobookID   string
	ChapterIndex  int
	PositionMS    int64
	PlaybackSpeed float64
	UpdatedAt     time.Time
}

// SearchHistoryEntry represents a past search query.
type SearchHistoryEntry struct {
	ID          int64
	Query       string
	Source      string
	ResultCount int
	CreatedAt   time.Time
}
```

- [ ] **Step 6: Create `internal/db/bookmarks.go`**

```go
package db

import (
	"database/sql"
	"fmt"
)

// CreateBookmark inserts a new bookmark.
func CreateBookmark(db *sql.DB, bm *Bookmark) (int64, error) {
	res, err := db.Exec(`
		INSERT INTO bookmarks (audiobook_id, title, author, narrator, source, page_url, note)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, bm.AudiobookID, bm.Title, bm.Author, bm.Narrator, bm.Source, bm.PageURL, bm.Note)
	if err != nil {
		return 0, fmt.Errorf("insert bookmark: %w", err)
	}
	return res.LastInsertId()
}

// ListBookmarks returns all bookmarks ordered by creation time.
func ListBookmarks(db *sql.DB) ([]*Bookmark, error) {
	rows, err := db.Query(`
		SELECT id, audiobook_id, title, author, narrator, source, page_url, note, created_at
		FROM bookmarks ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, fmt.Errorf("list bookmarks: %w", err)
	}
	defer rows.Close()

	var bookmarks []*Bookmark
	for rows.Next() {
		bm := &Bookmark{}
		if err := rows.Scan(
			&bm.ID, &bm.AudiobookID, &bm.Title, &bm.Author, &bm.Narrator,
			&bm.Source, &bm.PageURL, &bm.Note, &bm.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan bookmark: %w", err)
		}
		bookmarks = append(bookmarks, bm)
	}
	return bookmarks, rows.Err()
}

// DeleteBookmark removes a bookmark by ID.
func DeleteBookmark(db *sql.DB, id int64) error {
	_, err := db.Exec("DELETE FROM bookmarks WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("delete bookmark: %w", err)
	}
	return nil
}
```

- [ ] **Step 7: Create `internal/db/playback.go`**

```go
package db

import (
	"database/sql"
	"fmt"
	"time"
)

// SavePlaybackState upserts the playback position for an audiobook.
func SavePlaybackState(db *sql.DB, state *PlaybackState) error {
	_, err := db.Exec(`
		INSERT INTO playback_state (audiobook_id, chapter_index, position_ms, playback_speed, updated_at)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(audiobook_id) DO UPDATE SET
			chapter_index = excluded.chapter_index,
			position_ms = excluded.position_ms,
			playback_speed = excluded.playback_speed,
			updated_at = excluded.updated_at
	`, state.AudiobookID, state.ChapterIndex, state.PositionMS, state.PlaybackSpeed, time.Now())
	if err != nil {
		return fmt.Errorf("save playback state: %w", err)
	}
	return nil
}

// GetPlaybackState retrieves the playback position for an audiobook.
func GetPlaybackState(db *sql.DB, audiobookID string) (*PlaybackState, error) {
	state := &PlaybackState{}
	err := db.QueryRow(`
		SELECT id, audiobook_id, chapter_index, position_ms, playback_speed, updated_at
		FROM playback_state WHERE audiobook_id = ?
	`, audiobookID).Scan(
		&state.ID, &state.AudiobookID, &state.ChapterIndex,
		&state.PositionMS, &state.PlaybackSpeed, &state.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("get playback state: %w", err)
	}
	return state, nil
}
```

- [ ] **Step 8: Create `internal/db/history.go`**

```go
package db

import (
	"database/sql"
	"fmt"
)

// AddSearchHistory records a search query.
func AddSearchHistory(db *sql.DB, query, source string, resultCount int) error {
	_, err := db.Exec(`
		INSERT INTO search_history (query, source, result_count)
		VALUES (?, ?, ?)
	`, query, source, resultCount)
	if err != nil {
		return fmt.Errorf("add search history: %w", err)
	}
	return nil
}

// ListSearchHistory returns recent search history entries.
func ListSearchHistory(db *sql.DB, limit int) ([]*SearchHistoryEntry, error) {
	rows, err := db.Query(`
		SELECT id, query, source, result_count, created_at
		FROM search_history ORDER BY created_at DESC LIMIT ?
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("list search history: %w", err)
	}
	defer rows.Close()

	var entries []*SearchHistoryEntry
	for rows.Next() {
		e := &SearchHistoryEntry{}
		if err := rows.Scan(&e.ID, &e.Query, &e.Source, &e.ResultCount, &e.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan search history: %w", err)
		}
		entries = append(entries, e)
	}
	return entries, rows.Err()
}

// ClearSearchHistory removes all search history entries.
func ClearSearchHistory(db *sql.DB) error {
	_, err := db.Exec("DELETE FROM search_history")
	if err != nil {
		return fmt.Errorf("clear search history: %w", err)
	}
	return nil
}
```

- [ ] **Step 9: Run all tests**

```bash
cd ~/Documents/personal/audbookdl
go test -v ./internal/db/...
```

Expected: PASS — all db tests pass (bookmarks, playback, history, downloads, schema).

- [ ] **Step 10: Commit**

```bash
git add internal/db/
git commit -m "feat: add bookmarks, playback state, and search history CRUD"
```

---

### Task 7: CLI Skeleton — Root Command and Version

**Files:**
- Create: `internal/cli/root.go`
- Create: `internal/cli/version.go`

- [ ] **Step 1: Create `internal/cli/root.go`**

```go
package cli

import (
	"fmt"
	"os"

	"github.com/billmal071/audbookdl/internal/config"
	"github.com/billmal071/audbookdl/internal/db"
	"github.com/spf13/cobra"
)

var (
	// Version and Commit are set via LDFLAGS at build time.
	Version = "dev"
	Commit  = "unknown"

	cfgFile string
	verbose bool
)

var rootCmd = &cobra.Command{
	Use:   "audbookdl",
	Short: "Search and download free audiobooks",
	Long: `audbookdl is a CLI tool for searching and downloading free audiobooks
from LibriVox, Internet Archive, Loyal Books, and Open Library.

It features a full-screen TUI, built-in audio player, resumable downloads,
and SQLite-backed state tracking.

Run without arguments to launch the interactive TUI.

Examples:
  audbookdl                                Launch full TUI
  audbookdl search "sherlock holmes"        Search for audiobooks
  audbookdl download <id>                   Download an audiobook
  audbookdl play <id>                       Play a downloaded audiobook
  audbookdl list                            List all downloads`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		if err := config.Init(cfgFile); err != nil {
			return fmt.Errorf("failed to initialize config: %w", err)
		}

		if err := db.Init(); err != nil {
			return fmt.Errorf("failed to initialize database: %w", err)
		}

		return nil
	},
	PersistentPostRun: func(cmd *cobra.Command, args []string) {
		db.Close()
	},
}

// Execute runs the root command.
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default $HOME/.config/audbookdl/config.yaml)")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "verbose output")

	rootCmd.AddCommand(versionCmd)
}

// Verbose returns whether verbose mode is enabled.
func Verbose() bool {
	return verbose
}

// Printf prints if verbose mode is enabled.
func Printf(format string, args ...interface{}) {
	if verbose {
		fmt.Printf(format, args...)
	}
}

// Errorf prints an error message to stderr.
func Errorf(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, "Error: "+format+"\n", args...)
}
```

- [ ] **Step 2: Create `internal/cli/version.go`**

```go
package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Show version information",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("audbookdl %s (commit: %s)\n", Version, Commit)
	},
}
```

- [ ] **Step 3: Fetch Cobra dependency**

```bash
cd ~/Documents/personal/audbookdl
go get github.com/spf13/cobra
go mod tidy
```

- [ ] **Step 4: Verify the project builds**

```bash
cd ~/Documents/personal/audbookdl
CGO_ENABLED=0 go build -o ./build/audbookdl ./cmd/audbookdl
./build/audbookdl version
```

Expected output: `audbookdl dev (commit: <hash>)`

- [ ] **Step 5: Verify all tests pass**

```bash
cd ~/Documents/personal/audbookdl
go test -v ./...
```

Expected: PASS — all tests across all packages pass.

- [ ] **Step 6: Commit**

```bash
git add internal/cli/ go.mod go.sum
git commit -m "feat: add Cobra CLI skeleton with root and version commands"
```

---

### Task 8: Verify Full Build and Run All Tests

**Files:** None (verification only)

- [ ] **Step 1: Run full test suite**

```bash
cd ~/Documents/personal/audbookdl
go test -v ./...
```

Expected: PASS — all tests pass across `internal/source`, `internal/config`, `internal/db`.

- [ ] **Step 2: Run full build**

```bash
cd ~/Documents/personal/audbookdl
make build
```

Expected: Binary built at `./build/audbookdl`.

- [ ] **Step 3: Verify binary runs**

```bash
./build/audbookdl version
./build/audbookdl --help
```

Expected: Version prints, help shows usage with description and examples.

- [ ] **Step 4: Run format and vet checks**

```bash
cd ~/Documents/personal/audbookdl
go fmt ./...
go vet ./...
```

Expected: No errors, no unformatted files.

- [ ] **Step 5: Verify project structure**

```bash
find ~/Documents/personal/audbookdl -name "*.go" | sort
```

Expected file list:
```
cmd/audbookdl/main.go
internal/cli/root.go
internal/cli/version.go
internal/config/config.go
internal/config/config_test.go
internal/db/bookmarks.go
internal/db/bookmarks_test.go
internal/db/db.go
internal/db/db_test.go
internal/db/downloads.go
internal/db/downloads_test.go
internal/db/history.go
internal/db/history_test.go
internal/db/models.go
internal/db/playback.go
internal/db/playback_test.go
internal/source/source.go
internal/source/types.go
internal/source/types_test.go
```
