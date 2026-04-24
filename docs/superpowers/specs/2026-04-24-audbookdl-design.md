# audbookdl Design Spec

**Date:** 2026-04-24
**Status:** Approved
**Type:** New standalone CLI tool

## Overview

audbookdl is a standalone Go CLI tool for searching and downloading free audiobooks from multiple public sources. It features a full-screen terminal UI built with the Charm stack (bubbletea, bubbles, lipgloss), a built-in audio player with position memory, resumable chunked downloads, and SQLite-backed state tracking.

It follows the same architectural philosophy as bookdl (pure Go, no CGO, multi-source aggregation) but is purpose-built for the audiobook domain: albums with chapters, narrators, durations, and playback state.

## Goals

- Search LibriVox, Internet Archive, Loyal Books, and Open Library in parallel
- Download full audiobooks (multi-chapter) with per-chapter chunked resume
- Provide a full-screen TUI for browsing, downloading, and playing audiobooks
- Built-in terminal audio player with speed control, sleep timer, and position memory
- Post-download metadata tagging (ID3/M4B) with cover art embedding
- Pure Go, zero CGO, cross-platform (Linux, macOS, Windows)

## Non-Goals

- Streaming from sources (download-first model)
- Paid/DRM audiobook sources (public domain and free sources only)
- Text-to-speech conversion
- Mobile or GUI application

---

## 1. Domain Model

### Core Types

```go
// internal/source/source.go
type Source interface {
    Search(ctx context.Context, query string, opts SearchOptions) ([]*Audiobook, error)
    GetChapters(ctx context.Context, bookID string) ([]*Chapter, error)
    Name() string
}

type SearchOptions struct {
    Limit    int
    Page     int
    Language string
    Author   string
}
```

```go
// internal/source/types.go
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
    Format       string        // mp3, m4b, ogg
    TotalSize    int64
    ChapterCount int
    Source       string        // "librivox", "archive", "loyalbooks", "openlibrary"
}

type Chapter struct {
    Index       int
    Title       string
    Duration    time.Duration
    DownloadURL string
    Format      string
    FileSize    int64
}
```

### Download Model

```go
// internal/db/models.go
type AudiobookDownload struct {
    ID             int64
    AudiobookID    string
    Title          string
    Author         string
    Narrator       string
    Source          string
    Status         DownloadStatus  // pending | downloading | completed | failed | paused
    BasePath       string          // ~/Audiobooks/Author/Title/
    TotalSize      int64
    DownloadedSize int64
    CreatedAt      time.Time
    UpdatedAt      time.Time
    CompletedAt    time.Time
}

type ChapterDownload struct {
    ID            int64
    DownloadID    int64
    ChapterIndex  int
    Title         string
    FilePath      string
    FileSize      int64
    Downloaded    int64
    Status        DownloadStatus
}
```

### Key Domain Differences from bookdl

| Concept | bookdl | audbookdl |
|---------|--------|-----------|
| Unit of content | Single file (Book) | Multi-file album (Audiobook + Chapters) |
| Metadata | Author, publisher, format, pages | Author, narrator, duration, chapters |
| Download | One file with chunks | Album of files, each with chunks |
| Post-download | MD5 verify | Verify + ID3/M4B tag + cover art |
| Playback | N/A | Built-in player with position memory |

---

## 2. Sources

### LibriVox

- **Method:** REST API (`librivox.org/api/feed/audiobooks`)
- **Content:** Public domain audiobooks read by volunteers
- **Data available:** Title, author, reader, language, genre, chapter list with MP3 URLs, total time, cover
- **Rate limits:** Reasonable, no auth required
- **Formats:** MP3, OGG (via catalog)

### Internet Archive

- **Method:** REST API (`archive.org/advancedsearch.php` + metadata API)
- **Content:** Massive audiobook collection including LibriVox mirrors, community uploads
- **Data available:** Title, creator, description, duration, file list with direct download URLs
- **Rate limits:** Generous, no auth required
- **Formats:** MP3, OGG, M4B, FLAC
- **Notes:** Complex metadata extraction required; files are nested in "items" with varying structures

### Loyal Books

- **Method:** RSS feeds + web scraping (no formal API)
- **Content:** Curated public domain audiobooks (formerly Books Should Be Free)
- **Data available:** Title, author, genre, chapter list via RSS feed items
- **Rate limits:** Standard web scraping courtesy
- **Formats:** MP3, M4B (via iTunes-compatible feeds)

### Open Library

- **Method:** REST API (`openlibrary.org/search.json` + works API)
- **Content:** Audiobook metadata linking to Internet Archive holdings
- **Data available:** Title, author, subjects, edition info, IA identifiers
- **Rate limits:** Reasonable, no auth required
- **Formats:** Delegates to Internet Archive for actual files
- **Notes:** Primarily a metadata bridge; actual downloads go through Internet Archive

---

## 3. Package Structure

```
audbookdl/
├── cmd/audbookdl/
│   └── main.go
├── internal/
│   ├── source/
│   │   ├── source.go              # Source interface, SearchOptions
│   │   └── types.go               # Audiobook, Chapter structs
│   ├── librivox/
│   │   ├── client.go              # API client
│   │   └── parser.go              # Response parsing
│   ├── archive/
│   │   ├── client.go              # Archive.org API
│   │   └── parser.go              # Metadata extraction
│   ├── loyalbooks/
│   │   ├── client.go              # RSS feed + scraper
│   │   └── parser.go              # Feed/HTML parsing
│   ├── openlibrary/
│   │   ├── client.go              # Open Library API
│   │   └── parser.go              # Response mapping
│   ├── search/
│   │   ├── searcher.go            # Parallel multi-source orchestrator
│   │   └── options.go             # Filtering and sorting
│   ├── downloader/
│   │   ├── manager.go             # Album-aware download orchestrator
│   │   ├── chunk.go               # Per-file chunked downloads
│   │   ├── retry.go               # Exponential backoff with jitter
│   │   └── verify.go              # Post-download integrity checks
│   ├── player/
│   │   ├── player.go              # Core playback engine (beep/oto)
│   │   ├── playlist.go            # Chapter queue management
│   │   ├── state.go               # Persist/restore playback position
│   │   └── controls.go            # Speed, volume, sleep timer
│   ├── tagger/
│   │   ├── tagger.go              # ID3/M4B tag writer
│   │   └── cover.go               # Cover art embedding
│   ├── tui/
│   │   ├── app.go                 # Root model, tab navigation
│   │   ├── search.go              # Search tab
│   │   ├── detail.go              # Audiobook detail view
│   │   ├── downloads.go           # Downloads tab
│   │   ├── library.go             # Library tab (grouped by author)
│   │   ├── playerui.go            # Player tab (now-playing screen)
│   │   ├── help.go                # Context-sensitive help overlay
│   │   └── styles.go              # Lipgloss theme
│   ├── db/
│   │   ├── db.go                  # Connection, migrations, WAL
│   │   ├── downloads.go           # AudiobookDownload + ChapterDownload CRUD
│   │   ├── playback.go            # Playback state persistence
│   │   ├── bookmarks.go           # Bookmark CRUD
│   │   └── history.go             # Search history + cache
│   ├── config/
│   │   └── config.go              # Viper YAML config
│   ├── cli/
│   │   ├── root.go                # Init, cleanup, global flags
│   │   ├── search.go              # search command
│   │   ├── download.go            # download command
│   │   ├── play.go                # play command
│   │   ├── queue.go               # queue management
│   │   ├── list.go                # list downloads
│   │   ├── resume.go              # resume download
│   │   ├── pause.go               # pause download
│   │   ├── bookmark.go            # bookmark commands
│   │   ├── history.go             # search history
│   │   ├── config.go              # config get/set
│   │   ├── version.go             # version info
│   │   └── completion.go          # shell completions
│   └── notify/
│       └── notify.go              # Desktop notifications
├── go.mod
├── go.sum
├── Makefile
├── CLAUDE.md
└── README.md
```

### Key Design Decisions

- **`source.Source` interface in `internal/source/`** — each source package implements it, consumer packages depend only on the interface
- **Searcher uses `errgroup`** — fans out to all sources concurrently with context cancellation; partial failures return available results
- **Downloader is album-aware** — `manager.go` orchestrates all chapters as a unit, each chapter uses chunked download with SQLite tracking
- **Player is its own subsystem** — separated from TUI; `player.go` manages the beep audio pipeline, `state.go` handles persistence, TUI just renders
- **Tagger runs post-download** — after all chapters complete, embeds ID3 tags and cover art into each file
- **TUI and CLI coexist** — `audbookdl` with no args launches full TUI; individual commands work standalone for scripting

---

## 4. Database Schema

SQLite with WAL mode, using `modernc.org/sqlite` (pure Go, no CGO).

```sql
CREATE TABLE audiobook_downloads (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    audiobook_id TEXT NOT NULL,
    title TEXT NOT NULL,
    author TEXT NOT NULL,
    narrator TEXT DEFAULT '',
    source TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending',
    base_path TEXT NOT NULL,
    total_size INTEGER DEFAULT 0,
    downloaded_size INTEGER DEFAULT 0,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    completed_at TIMESTAMP
);

CREATE TABLE chapter_downloads (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    download_id INTEGER NOT NULL REFERENCES audiobook_downloads(id) ON DELETE CASCADE,
    chapter_index INTEGER NOT NULL,
    title TEXT NOT NULL,
    file_path TEXT NOT NULL,
    file_size INTEGER DEFAULT 0,
    downloaded INTEGER DEFAULT 0,
    status TEXT NOT NULL DEFAULT 'pending',
    UNIQUE(download_id, chapter_index)
);
CREATE INDEX idx_chapter_downloads_download ON chapter_downloads(download_id);

CREATE TABLE chunks (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    chapter_download_id INTEGER NOT NULL REFERENCES chapter_downloads(id) ON DELETE CASCADE,
    chunk_index INTEGER NOT NULL,
    start_byte INTEGER NOT NULL,
    end_byte INTEGER NOT NULL,
    downloaded INTEGER DEFAULT 0,
    status TEXT NOT NULL DEFAULT 'pending'
);
CREATE INDEX idx_chunks_chapter ON chunks(chapter_download_id);

CREATE TABLE bookmarks (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    audiobook_id TEXT NOT NULL,
    title TEXT NOT NULL,
    author TEXT NOT NULL,
    narrator TEXT DEFAULT '',
    source TEXT NOT NULL,
    page_url TEXT,
    note TEXT DEFAULT '',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE playback_state (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    audiobook_id TEXT NOT NULL UNIQUE,
    chapter_index INTEGER NOT NULL DEFAULT 0,
    position_ms INTEGER NOT NULL DEFAULT 0,
    playback_speed REAL NOT NULL DEFAULT 1.0,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE search_history (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    query TEXT NOT NULL,
    source TEXT DEFAULT '',
    result_count INTEGER DEFAULT 0,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE search_cache (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    cache_key TEXT NOT NULL UNIQUE,
    results BLOB NOT NULL,
    expires_at TIMESTAMP NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
```

---

## 5. CLI Commands

| Command | Usage | Description |
|---------|-------|-------------|
| *(no args)* | `audbookdl` | Launch full-screen TUI |
| `search` | `audbookdl search "query"` | Search all sources |
| `download` | `audbookdl download <id>` | Download audiobook by ID |
| `play` | `audbookdl play <id or path>` | Launch terminal player |
| `list` | `audbookdl list` | List all downloads |
| `queue` | `audbookdl queue add/remove/list/clear` | Manage download queue |
| `resume` | `audbookdl resume <id>` | Resume paused download |
| `pause` | `audbookdl pause <id>` | Pause active download |
| `bookmark` | `audbookdl bookmark <id>` | Save audiobook for later |
| `bookmarks` | `audbookdl bookmarks` | List saved bookmarks |
| `history` | `audbookdl history` | Browse search history |
| `config` | `audbookdl config set/get` | Manage settings |
| `version` | `audbookdl version` | Show version info |
| `completion` | `audbookdl completion bash/zsh/fish` | Shell completions |

### Search Flags

```
-n, --limit       Number of results (default 10)
-s, --source      Filter source (librivox, archive, loyalbooks, openlibrary)
-l, --language    Filter by language
-a, --author      Filter by author
-f, --format      Prefer format (mp3, m4b, ogg)
-d, --download    Download selected result immediately
-q, --quiet       Non-interactive, return first result
```

---

## 6. TUI Design

### Architecture

Full-screen bubbletea application with 4 tabs, using the Charm stack:

- **bubbletea** — elm-architecture framework
- **bubbles** — pre-built components (textinput, list, viewport, progress, spinner, help)
- **lipgloss** — styling, colors, borders, layout

### Tab Structure

Running `audbookdl` with no args launches the TUI with tab navigation:

```
[Search]  [Downloads]  [Library]  [Player]
```

### Component Mapping

| TUI Element | Bubbles Component |
|-------------|-------------------|
| Search input | `textinput` |
| Result/chapter lists | `list` with custom delegates |
| Scrollable detail view | `viewport` |
| Download progress bars | `progress` |
| Loading states | `spinner` |
| Help bar | `help` |
| Source filter toggles | Custom `key.Binding` set |
| Tab bar | Custom model with lipgloss |

### Search Tab

- Text input for query with inline source toggles
- Results displayed as a list with title, author, narrator, duration, chapter count, source
- Press enter for detail view with full chapter listing
- Keybindings: `d` download, `b` bookmark, `/` focus search

### Downloads Tab

- Sections: Active (with progress bars), Queued, Completed
- Per-download: title, source, progress bar, chapter progress (Ch X/Y), speed
- Keybindings: `p` pause, `r` resume, `x` cancel, `enter` play completed

### Library Tab

- Downloaded audiobooks grouped by author (nested tree view)
- Bookmarks section at bottom
- Sort options: Recent, Author, Title, Duration
- Filter input for quick search
- Keybindings: `enter` play, `d` download bookmark, `x` remove

### Player Tab

- Now-playing display: title, author, narrator, chapter name
- Progress bar with current/total time
- Transport controls: play/pause, skip, next/prev chapter
- Speed, volume, and sleep timer indicators
- Up-next chapter list
- Keybindings: `space` play/pause, `left/right` skip 15s, `n/p` next/prev chapter, `s` speed, `v` volume, `t` sleep timer

---

## 7. Audio Player Engine

### Technology

- **gopxl/beep** — audio streaming, decoding, effects
- **ebitengine/oto** — cross-platform audio output backend

### Streamer Pipeline

```
File decoder (MP3/M4B/OGG)
    -> beep.Resample (playback speed)
    -> effects.Volume (volume control)
    -> speaker.Ctrl (play/pause)
    -> oto output (system audio)
```

### Feature Implementation

| Feature | Implementation |
|---------|----------------|
| Play/pause | `speaker.Ctrl` toggle |
| Skip +/-15s | Seek on decoder streamer |
| Next/prev chapter | Close current decoder, open next file, rebuild pipeline |
| Playback speed | `beep.Resample` with rate adjustment (0.5x - 3.0x) |
| Volume | `effects.Volume` gain control (0% - 100%) |
| Sleep timer | `time.AfterFunc` goroutine, pauses playback on expiry |
| Remember position | Poll position every 5s, write to `playback_state` table |
| Playlists | In-memory chapter queue backed by file order on disk |

### Lifecycle

On quit or chapter transition, the player saves state before cleanup:

```go
func (p *Player) saveAndClose(ctx context.Context) error {
    pos := p.currentPosition()
    if err := p.db.SavePlaybackState(ctx, p.audiobookID, p.chapterIndex, pos, p.speed); err != nil {
        return fmt.Errorf("save playback state: %w", err)
    }
    speaker.Close()
    return nil
}
```

On launch, if a `playback_state` entry exists for the audiobook, the player resumes from the saved chapter and position.

---

## 8. Download Manager

### Album-Aware Downloads

The download manager treats an entire audiobook as a single unit:

1. Create `AudiobookDownload` record with status `pending`
2. Fetch chapter list via `Source.GetChapters()`
3. Create `ChapterDownload` records for each chapter
4. Download chapters with configurable concurrency (`max_concurrent` in config, default 3)
5. Each chapter uses chunked download with per-chunk SQLite tracking for resume
6. On all chapters complete: run tagger, update status to `completed`, send notification

### Resume Logic

On `audbookdl resume <id>`:

1. Load `AudiobookDownload` and its `ChapterDownload` records
2. Skip chapters with status `completed`
3. For in-progress chapters, resume from last completed chunk
4. For pending chapters, start fresh

### File Organization

Files are saved in a nested author/title structure:

```
~/Audiobooks/
└── Arthur Conan Doyle/
    └── The Adventures of Sherlock Holmes/
        ├── 01 - A Scandal in Bohemia.mp3
        ├── 02 - The Red-Headed League.mp3
        ├── ...
        └── cover.jpg
```

---

## 9. Metadata Tagger

Runs automatically after all chapters of an audiobook finish downloading.

### Tags Written

| Tag | Source |
|-----|--------|
| Title | Chapter title |
| Album | Audiobook title |
| Artist | Author |
| Album Artist | Author |
| Composer | Narrator |
| Track Number | Chapter index |
| Total Tracks | Chapter count |
| Genre | "Audiobook" |
| Year | Publication year |
| Cover Art | Downloaded from CoverURL, embedded as front cover |

### Libraries

- MP3: Pure Go ID3v2 tag writer
- M4B/M4A: MP4 atom manipulation
- Cover art: Download image, resize if needed, embed

---

## 10. Configuration

Config file at `~/.config/audbookdl/config.yaml`, managed by Viper with `AUDBOOKDL_*` env var overrides.

```yaml
download:
  directory: ~/Audiobooks
  chunk_size: 5242880         # 5MB
  max_concurrent: 3           # parallel chapter downloads
  preferred_format: mp3

player:
  default_speed: 1.0
  skip_seconds: 15
  sleep_timer_minutes: 0      # 0 = disabled

search:
  default_limit: 10
  cache_ttl: 3600             # 1 hour
  sources:
    - librivox
    - archive
    - loyalbooks
    - openlibrary

notifications:
  enabled: true
  sound: true
```

---

## 11. Build & Distribution

### Makefile

```makefile
build:          # Build to ./build/audbookdl (CGO_ENABLED=0)
install:        # Install to GOPATH/bin
test:           # go test -v ./...
fmt:            # gofmt + goimports
lint:           # golangci-lint
build-all:      # Cross-compile: linux/macOS (amd64+arm64), windows
```

### Version Injection

LDFLAGS inject `Version` and `Commit` into `internal/cli` package at build time.

---

## 12. Key Dependencies

| Package | Purpose |
|---------|---------|
| `github.com/spf13/cobra` | CLI framework |
| `github.com/spf13/viper` | Configuration |
| `github.com/charmbracelet/bubbletea` | TUI framework |
| `github.com/charmbracelet/bubbles` | TUI components |
| `github.com/charmbracelet/lipgloss` | TUI styling |
| `github.com/gopxl/beep` | Audio playback + effects |
| `github.com/ebitengine/oto` | Audio output backend |
| `modernc.org/sqlite` | Pure Go SQLite driver |
| `github.com/PuerkitoBio/goquery` | HTML parsing (Loyal Books scraper) |
| `github.com/mmcdole/gofeed` | RSS feed parsing (Loyal Books) |
| `golang.org/x/sync/errgroup` | Concurrent source searching |
