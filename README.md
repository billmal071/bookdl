# bookdl

A command-line tool for searching and downloading books from Anna's Archive.

## Features

- **Interactive Search**: Search for books with an interactive TUI selector
- **Resumable Downloads**: Pause and resume downloads with chunk-based progress tracking
- **Multiple Access Methods**: Supports API access and web scraping with Cloudflare bypass
- **Download Management**: Track, pause, resume, and restart downloads
- **Format Filtering**: Filter search results by format (EPUB, PDF, etc.)
- **Pagination**: Load more results if you don't find what you're looking for

## Installation

### From Source

```bash
# Clone the repository
git clone https://github.com/billmal071/bookdl.git
cd bookdl

# Build
make build

# Or install to GOPATH/bin
make install
```

### Requirements

- Go 1.21 or later
- Chrome/Chromium (for Cloudflare bypass, optional)

## Usage

### Search for Books

```bash
# Basic search
bookdl search "clean code"

# Search with format filter
bookdl search -f epub "design patterns"

# Filter by language
bookdl search -l english "machine learning"

# Filter by year or year range
bookdl search --year 2020 "python"
bookdl search --year 2020-2024 "algorithms"

# Filter by maximum file size
bookdl search --max-size 10MB "data science"

# Combine filters
bookdl search -f pdf -l english --year 2020-2024 "deep learning"

# Limit number of results
bookdl search -n 10 "golang programming"

# Search and immediately download
bookdl search -d "pragmatic programmer"
```

In the interactive selector:
- `↑/↓` - Navigate through results
- `Enter` - Select a book
- `i` - Show book details
- `o` - Open book page in browser (when details visible)
- `m` - Load more results
- `q/Esc` - Cancel

### Download a Book

```bash
# Download by MD5 hash
bookdl download abc123def456789...

# Specify output directory
bookdl download -o ~/Books abc123def456789...
```

### Manage Downloads

```bash
# List all downloads
bookdl list

# List only active downloads
bookdl list --active

# Pause a download
bookdl pause 1

# Pause all downloads
bookdl pause all

# Resume a download
bookdl resume 1

# Resume all paused downloads
bookdl resume all

# Restart a failed download
bookdl restart 1
```

### Configuration

```bash
# Show config file path
bookdl config path

# Get a config value
bookdl config get downloads.path

# Set a config value
bookdl config set downloads.path ~/Books
```

Configuration file location: `~/.config/bookdl/config.yaml`

```yaml
anna:
  base_url: "annas-archive.li"
  api_key: ""  # Optional API key for faster access

downloads:
  path: "~/Downloads/books"
  concurrent: 1
  chunk_size: 5242880  # 5MB chunks
```

Environment variables can override config values with the `BOOKDL_` prefix:
```bash
export BOOKDL_DOWNLOADS_PATH=~/Books
export BOOKDL_ANNA_API_KEY=your-api-key
```

## How It Works

1. **Search**: Queries Anna's Archive for books matching your search
2. **Selection**: Presents results in an interactive terminal UI
3. **Download**: Fetches the book using available mirrors with automatic fallback
4. **Resumable**: Downloads are split into chunks and tracked in a local SQLite database

When Cloudflare protection is detected, bookdl automatically falls back to a headless browser to bypass the challenge.

## Project Structure

```
bookdl/
├── cmd/bookdl/          # Entry point
├── internal/
│   ├── anna/            # Anna's Archive client (API, scraper, browser)
│   ├── cli/             # CLI commands
│   ├── config/          # Configuration management
│   ├── db/              # SQLite database layer
│   ├── downloader/      # Download manager
│   └── tui/             # Terminal UI components
├── build/               # Build output
├── Makefile             # Build automation
└── IMPROVEMENTS.md      # Planned features
```

## Contributing

Contributions are welcome! See [IMPROVEMENTS.md](IMPROVEMENTS.md) for planned features and ideas.

1. Fork the repository
2. Create a feature branch: `git checkout -b feature/my-feature`
3. Make your changes
4. Run tests: `make test`
5. Submit a pull request

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## Disclaimer

This tool is for educational and personal use only. Please respect copyright laws and the terms of service of the sources accessed. The authors are not responsible for any misuse of this software.
