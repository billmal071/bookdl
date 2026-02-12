# Planned Improvements

This document tracks potential improvements and features for bookdl. Items are organized by priority.

## High Priority

### 1. Add Tests
- [ ] Unit tests for `internal/downloader/manager.go`
- [ ] Unit tests for `internal/anna/scraper.go` (mock HTTP responses)
- [ ] Unit tests for `internal/anna/api.go`
- [ ] Integration tests for the CLI commands
- [ ] TUI component tests

### ~~2. Search Filters~~ ✓
- [x] Language filter (`-l english`, `--language spanish`)
- [x] Year range filter (`--year 2020` or `--year 2020-2024`)
- [x] File size limit (`--max-size 50MB`)
- [ ] Source filter (libgen, sci-hub, etc.) - deferred, requires API support

### ~~3. Download Queue~~ ✓
- [x] Select multiple books from search results (space to toggle, enter to confirm)
- [x] Queue management commands (`bookdl queue`, `bookdl queue clear`, `bookdl queue remove`)
- [ ] Priority ordering in queue - deferred

### ~~4. Concurrent Downloads~~ ✓
- [x] Download multiple books simultaneously
- [x] Configurable concurrency limit (`downloads.max_concurrent` in config, default 2)
- [x] Per-download status callbacks
- [x] Overall progress summary with completed/failed counts

## Medium Priority

### ~~5. Book Details View~~ ✓
- [x] Press 'i' in selector to view full book info
- [x] Show publisher, year, language, format, size, MD5, URL
- [x] Option to open book page in browser (press 'o')

### 6. Search History
- [ ] Store recent searches in database
- [ ] Access with `bookdl search --history` or `bookdl history`
- [ ] Arrow up/down in search to cycle through history
- [ ] Clear history command

### ~~7. Better Progress Display~~ ✓
- [x] Show download speed (MB/s)
- [x] Show ETA (estimated time remaining)
- [x] Cleaner progress bar with colors (green/cyan themes)
- [x] Show chunk progress for resumable downloads

### ~~8. Favorites/Bookmarks~~ ✓
- [x] Save books for later: `bookdl bookmark <md5>`
- [x] List bookmarks: `bookdl bookmarks`
- [x] Remove bookmark: `bookdl bookmark -d <md5>`
- [x] Download all bookmarks: `bookdl bookmarks --download`
- [x] Add notes to bookmarks: `bookdl bookmark <md5> -n "note"`

### ~~9. File Organization~~ ✓
- [x] Auto-organize downloads into folders
- [x] Configurable patterns: `{author}/{title}.{format}`
- [x] Options: by author, by format, by year, flat
- [x] Rename files based on metadata
- [x] Configure via: `bookdl config organize [mode]`

## Lower Priority

### 10. Integrity Verification
- [ ] Verify MD5/SHA checksums after download
- [ ] Re-download corrupted files automatically
- [ ] Show verification status in `bookdl list`

### 11. Export/Import
- [ ] Export download history: `bookdl export history.json`
- [ ] Export bookmarks: `bookdl export --bookmarks`
- [ ] Import from backup: `bookdl import history.json`

### 12. Shell Completions
- [ ] Enhanced zsh completions with descriptions
- [ ] Fish shell support
- [ ] Dynamic completion for download IDs

### 13. Retry with Exponential Backoff
- [ ] Smarter retry logic for transient failures
- [ ] Configurable max retries and backoff multiplier
- [ ] Different strategies per error type

### 14. Notifications
- [ ] Desktop notifications on download complete (optional)
- [ ] Sound notification option
- [ ] macOS/Linux/Windows support

### 15. Cache Search Results
- [ ] Cache recent search results locally
- [ ] Configurable cache TTL
- [ ] Offline browsing of cached results

---

## Contributing

To work on an improvement:
1. Check the item you want to implement
2. Create a feature branch: `git checkout -b feature/search-filters`
3. Implement the feature
4. Add tests
5. Submit a pull request

## Completed

- [x] Load more results in search (press 'm' for more) - v0.1.0
- [x] Pagination support for search - v0.1.0
- [x] Fix FormatSize display function - v0.1.0
- [x] Book details view (press 'i') with open in browser (press 'o') - v0.2.0
- [x] Search filters: language (-l), year (--year), max size (--max-size) - v0.2.0
- [x] Download queue with multi-select (search -q) and queue management - v0.2.0
- [x] Concurrent downloads with configurable limit (max_concurrent) - v0.2.0
- [x] Better progress display with speed, ETA, colors, chunk info - v0.2.0
- [x] Favorites/Bookmarks with notes and batch download - v0.2.0
- [x] File organization by author/format/year/custom pattern - v0.2.0
