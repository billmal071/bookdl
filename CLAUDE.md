# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

**bookdl** is a Go CLI tool for searching and downloading books from Anna's Archive. It supports interactive TUI selection, resumable chunked downloads, browser automation for Cloudflare bypass, and SQLite-backed state tracking.

## Build & Development Commands

```bash
make build          # Build to ./build/bookdl (CGO_ENABLED=0)
make install        # Install to GOPATH/bin
make test           # go test -v ./...
make fmt            # Format code
make lint           # golangci-lint (must be installed separately)
make run ARGS="..." # Build and run with arguments
make deps           # go mod tidy && go mod download
make build-all      # Cross-compile for Linux, macOS (amd64+arm64), Windows
```

CGO is disabled — SQLite uses `modernc.org/sqlite` (pure Go). Do not introduce cgo-dependent SQLite drivers.

## Architecture

### Multi-Source Search (`internal/search/`)
Unified `Searcher` aggregates results from multiple book sources in parallel.

### Source Clients (`internal/anna/`, `internal/zlibrary/`, `internal/liber3/`)
Each source follows the same strategy pattern with interchangeable clients:
- **APIClient** — uses API key when configured
- **ScraperClient** — web scraping fallback via colly/goquery; auto-detects Cloudflare challenges
- **BrowserClient** — headless Chrome via chromedp for Cloudflare bypass; shared browser pool

Client selection: API key present → APIClient, otherwise → ScraperClient (which may escalate to BrowserClient).

### Download Manager (`internal/downloader/`)
- **Chunked downloads** with configurable chunk size (default 5MB)
- Per-chunk progress tracked in SQLite for pause/resume
- Exponential backoff retry with jitter (`retry.go`)
- Post-download MD5 verification (`verify.go`)

### Database (`internal/db/`)
SQLite with WAL mode. Tables: `downloads`, `chunks`, `bookmarks`, `search_history`, `search_cache`. Schema migrations run at startup in `db.go`.

### CLI (`internal/cli/`)
Cobra-based with 16 subcommands. `root.go` handles init (config + DB) in `PersistentPreRunE` and cleanup (browser + DB close) in `PersistentPostRun`.

### TUI (`internal/tui/`)
Bubbletea-based interactive selectors for book picking and search history browsing.

### Config (`internal/config/`)
Viper-based YAML config at `~/.config/bookdl/config.yaml`. Environment overrides via `BOOKDL_*` prefix.

### Version Injection
LDFLAGS inject `Version` and `Commit` into `internal/cli` package variables at build time.
