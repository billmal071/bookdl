# Contributing to bookdl

Thanks for your interest in contributing to bookdl! This guide covers everything you need to get started.

## Prerequisites

- Go 1.22 or later
- Chrome or Chromium (needed for browser-based scraping)
- `golangci-lint` (optional, for linting)

## Getting Started

```bash
# Fork and clone
git clone https://github.com/<your-username>/bookdl.git
cd bookdl

# Install dependencies
make deps

# Build
make build

# Run
./build/bookdl --help
```

## Development Workflow

1. Create a feature branch from `main`:
   ```bash
   git checkout -b feature/my-feature
   ```

2. Make your changes. Build and test frequently:
   ```bash
   make build          # Build to ./build/bookdl
   make test           # Run all tests
   make fmt            # Format code
   make lint           # Run golangci-lint
   ```

3. Commit with a clear message describing **what** and **why**.

4. Push and open a pull request against `main`.

## Build Notes

CGO is disabled (`CGO_ENABLED=0`) because the project uses `modernc.org/sqlite` (pure Go SQLite). Do not introduce cgo-dependent dependencies.

Cross-compile for all platforms:
```bash
make build-all       # Linux amd64, macOS amd64+arm64, Windows amd64
```

## Project Architecture

The codebase is organized under `internal/` with clear package boundaries:

| Package | Purpose |
|---------|---------|
| `anna/` | Anna's Archive client (API, scraper, browser) |
| `zlibrary/` | Z-Library client (browser-only for search due to CSR) |
| `liber3/` | Liber3 client (scraper, browser) |
| `search/` | Multi-source search orchestrator |
| `cli/` | Cobra CLI commands and flags |
| `config/` | Viper-based YAML config management |
| `db/` | SQLite database (WAL mode, migrations) |
| `downloader/` | Chunked download manager with retry |
| `tui/` | Bubbletea interactive selectors |
| `notify/` | Cross-platform desktop notifications |

### Key Patterns

- **Source clients** follow a strategy pattern: each source has a `ScraperClient` (lightweight HTTP) and a `BrowserClient` (headless Chrome via chromedp). The scraper tries first and falls back to the browser on Cloudflare or empty results.
- **Z-Library is an exception** — it uses client-side rendering with `<z-bookcard>` Web Components, so search always goes through the browser. Book data is extracted from element attributes via JavaScript evaluation.
- **The `search.Searcher`** orchestrates multi-source searches concurrently and converts all results to `anna.Book` as the common type.
- **Downloads** are split into configurable chunks tracked in SQLite, enabling pause/resume.

## Adding a New Source

1. Create a new package under `internal/` (e.g., `internal/newsource/`).
2. Define `Book`, `DownloadInfo`, and `Client` types matching the pattern in `zlibrary/types.go`.
3. Implement `ScraperClient` and `BrowserClient`.
4. Add a config section in `internal/config/config.go`.
5. Add the source to `internal/search/searcher.go` with a new `Option` constant and search method.
6. Add the `--source` flag value in `internal/cli/search.go`.

## What to Work On

See [IMPROVEMENTS.md](IMPROVEMENTS.md) for planned features. Notable open items:

- **Tests** — the project currently has no test files. Unit tests for the downloader, scraper, and API clients are the highest priority.
- **Export/Import** — export download history and bookmarks to JSON.

## Code Style

- Run `make fmt` before committing.
- Follow standard Go conventions: `gofmt`, exported names documented, errors wrapped with `%w`.
- Keep packages focused. Don't add cross-cutting concerns to existing packages.
- No debug `fmt.Printf` in committed code — use the `verbose` flag pattern from `cli/` if needed.

## License

By contributing, you agree that your contributions will be licensed under the [MIT License](LICENSE).
