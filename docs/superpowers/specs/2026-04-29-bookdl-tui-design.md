# bookdl TUI Dashboard Design Spec

## Overview

A full-screen interactive terminal dashboard for bookdl, launched by default when running `bookdl` with no arguments. Provides search, download management, queue control, bookmarks, and search history in a tabbed interface with live progress tracking.

All existing CLI subcommands (`bookdl search`, `bookdl download`, etc.) continue working as-is for scripting and automation.

## Architecture

### Entry Point

- `bookdl` (no args) launches the full-screen TUI dashboard
- `bookdl tui` also launches it explicitly
- All existing subcommands remain unchanged
- Detection: if `!term.IsTerminal(os.Stdin.Fd()) || os.Getenv("NO_TUI") != ""`, fall back to existing CLI help

### Tech Stack

- **bubbletea** — TUI framework (already in go.mod)
- **bubbles** — text input, list, spinner, progress components (already in go.mod)
- **lipgloss** — styling and layout (already in go.mod)
- No new dependencies

### Package Structure

```
internal/tui/
  dashboard/
    model.go          # Root dashboard model, tab routing, Init/Update/View
    tabs.go           # Tab bar component and tab enum
    theme.go          # Semantic theme with AdaptiveColor
    styles.go         # Pre-computed lipgloss styles (allocated once at startup)
    layout.go         # Adaptive layout engine, weight-based sizing
    keys.go           # Key bindings, focus-aware routing
    search.go         # Search panel model
    downloads.go      # Downloads panel model (split pane)
    queue.go          # Queue panel model
    bookmarks.go      # Bookmarks panel model
    history.go        # History panel model
    helpers.go        # Truncation, formatting, shared rendering utils
```

Existing `internal/tui/selector.go`, `history_selector.go`, `styles.go` remain unchanged — they serve the CLI flow.

## Layout

### Overall Structure

```
┌──────────────────────────────────────────────────────────────────┐
│  bookdl      Search ━━  Downloads    Queue    Bookmarks  History │  ← Tab bar
├──────────────────────────────────────────────────────────────────┤
│                                                                  │
│                        Panel Content                             │  ← Content area
│                                                                  │
├──────────────────────────────────────────────────────────────────┤
│  ⇥ switch tab   ↑↓ navigate   d download   q quit               │  ← Status bar
└──────────────────────────────────────────────────────────────────┘
```

- **Tab bar**: 1 line. App name left (bold cyan), tabs right. Active tab: bold purple + `━━` underline. Inactive: subtext color.
- **Content area**: Fills remaining height. Adapts to `tea.WindowSizeMsg`.
- **Status bar**: 1 line. Context-sensitive key hints. Keys in primary (purple), descriptions in subtext.

### Responsive Breakpoints

| Terminal Width | Behavior |
|---|---|
| < 80 cols | Single panel, compact list items, no split view |
| 80-120 cols | Tab bar + full panel content |
| 120+ cols | Downloads panel gets split view (list + detail) |

### Height Calculation (Golden Rule #1)

```
contentHeight = terminalHeight - 1 (tab bar) - 1 (separator) - 1 (status bar) - 2 (panel borders)
```

### Panel Sizing (Golden Rule #4 — Weights)

Downloads split pane: list weight 2, detail weight 3 (40%/60%).

## Semantic Theme

Uses `lipgloss.AdaptiveColor` for automatic light/dark terminal support with WCAG AA contrast.

### Status Colors

| Semantic | Light | Dark | Usage |
|---|---|---|---|
| Downloading | `#e65100` | `#ffb86c` | Active download progress |
| Paused | `#f9a825` | `#f1fa8c` | Suspended downloads |
| Completed | `#2e7d32` | `#50fa7b` | Success states |
| Failed | `#c62828` | `#ff5555` | Error states |
| Pending | `#757575` | `#6272a4` | Queued/waiting |

### UI Chrome Colors

| Semantic | Light | Dark | Usage |
|---|---|---|---|
| Primary | `#7c4dff` | `#bd93f9` | Focused elements, active tab, selection |
| Secondary | `#1565c0` | `#8be9fd` | Labels, info text, app name |
| Text | `#1a1a2e` | `#f8f8f2` | Primary content |
| Subtext | `#757575` | `#6272a4` | Metadata, timestamps, inactive tabs |
| Border | `#bdbdbd` | `#44475a` | Unfocused borders |
| Highlight | `#7c4dff` | `#bd93f9` | Cursor/selection highlight |

### Style Rules

- Pre-compute all `lipgloss.Style` objects at startup — zero allocations in `View()`
- Use `strings.Builder` with `Grow()` in all `View()` methods
- Truncate all text to `panelWidth - 4` before rendering (Golden Rule #2)
- No emoji for status indicators — use text glyphs for consistent terminal width

### Status Indicators

```
⬇ Downloading    (downloading color)
‖ Paused         (paused color)
✓ Completed      (completed color)
✗ Failed         (failed color)
○ Pending        (pending color)
```

### Progress Bars

```
████████████▒▒▒▒▒▒▒▒  62%     (status-colored fill, border-colored empty)
```

- `█` (full block) for filled portion in status color
- `▒` (medium shade) for empty portion in border color
- Percentage right-aligned in text color

### List Items

```
  ▸ Clean Code                                         (primary bold, selected)
    Robert C. Martin  ·  EPUB  ·  2.3 MB  ·  English   (subtext, · separators)

    The Clean Coder                                     (text color, unselected)
    Robert C. Martin  ·  PDF  ·  4.1 MB  ·  English    (subtext)
```

### Bordered Containers

- Focused: rounded border in primary (purple) with label
- Unfocused: rounded border in border color (subtle gray)
- Content height filled exactly — no `Height()` on bordered styles (Golden Rule #1)

## Panels

### Search Panel

```
╭─ Search ──────────────────────────────────────────╮
│                                                    │
│   clean code█                                      │
│                                                    │
╰────────────────────────────────────────────────────╯
Source: All ▸ Anna ▸ Z-Library ▸ Liber3

Results (12):

  ▸ Clean Code
    Robert C. Martin  ·  EPUB  ·  2.3 MB  ·  English
    The Clean Coder
    Robert C. Martin  ·  PDF  ·  4.1 MB  ·  English
```

- Search input focused on tab entry. `Enter` submits. `Esc` blurs to results list.
- Source selector: cycle with `s` when input not focused (All -> Anna -> Z-Library -> Liber3).
- Results list reuses existing `BookItem` rendering approach from `internal/tui/selector.go`.
- Multi-select: `Space` toggles checkboxes. `d` downloads selected (or current if none selected).
- `m` loads more results (existing pagination). `i` toggles detail overlay for highlighted book.
- Loading state: braille spinner `⠋⠙⠹⠸` + "Searching..." in secondary color.

### Downloads Panel (Split Pane)

```
╭─ Downloads ────────────╮╭─ Details ─────────────────╮
│                         ││                            │
│  ▸ ⬇ Clean Code        ││  Title     Clean Code      │
│    ████████████▒▒ 62%   ││  Author    Robert C. Martin│
│                         ││  Format    EPUB             │
│    ‖ The Clean Coder    ││  Size      2.3 MB           │
│    ████▒▒▒▒▒▒▒▒ 31%    ││  Source    Anna's Archive   │
│                         ││  Status    ⬇ Downloading    │
│    ✓ Clean Arch...      ││                            │
│    ████████████ 100%    ││  ████████████████▒▒▒▒ 62%  │
│                         ││  1.4 MB / 2.3 MB           │
│                         ││  Speed  245 KB/s  ETA  4s  │
╰─────────────────────────╯╰────────────────────────────╯
```

- **Left pane**: Download list with inline mini progress bars and status glyphs.
- **Right pane**: Detail view for highlighted download — full metadata, large progress bar, speed, ETA.
- **Live updates**: `tea.Tick` at 500ms interval polls download state from DB and re-renders.
- **Actions**: `p` pause, `r` resume, `x` cancel/remove.
- Split pane only shows at 120+ cols. Below that, single list view with inline details.

### Queue Panel

```
Queue (4 pending):

  ▸ 1. [P:10] Design Patterns
       Gang of Four  ·  EPUB  ·  3.2 MB
    2. [P:5]  Pragmatic Programmer
       David Thomas  ·  PDF  ·  5.1 MB
    3.       Refactoring
       Martin Fowler  ·  EPUB  ·  2.8 MB
```

- Priority badge `[P:N]` shown when non-zero, sorted by priority descending.
- `t` moves to top, `B` to bottom (maps to existing `db.SetPriorityTop/Bottom`).
- `d` starts downloading highlighted item. `a` starts all. `x` removes from queue.
- Auto-refresh: items that start downloading disappear and appear in Downloads tab.

### Bookmarks Panel

```
Bookmarks (3):

  ▸ 1. Clean Code
       Robert C. Martin  ·  EPUB  ·  2.3 MB
       Added: 2026-04-15
    2. The Pragmatic Programmer
       David Thomas  ·  PDF  ·  5.1 MB
       Added: 2026-04-10  Note: "Must read"
```

- `d` downloads highlighted, `a` downloads all, `x` removes bookmark.
- `n` opens inline text input to add/edit note.
- `/` opens filter input to narrow by title.
- `o` opens book's page URL in default browser (reuses existing `openBrowser()`).

### History Panel

```
Search History (8):

  ▸ 1. "clean code"
       12 results  ·  format=epub  ·  2026-04-29 14:30
    2. "design patterns"
       8 results  ·  2026-04-28 09:15
```

- `Enter` re-runs search: switches to Search tab with query pre-filled and executed (including original filters).
- `x` deletes single entry. `c` clears all (with confirmation prompt).
- `/` to filter by query text.

## Keyboard Navigation

### Focus State Machine

```go
type focus int
const (
    focusTabBar focus = iota
    focusSearchInput
    focusSearchResults
    focusDownloadList
    focusDownloadDetail
    focusQueueList
    focusBookmarkList
    focusBookmarkNote    // inline text input
    focusHistoryList
    focusConfirmDialog   // modal overlay
)
```

Modal priority: confirm dialogs handled first, then input fields, then panel-level keys.

### Global Keys (always active, unless text input focused)

| Key | Action |
|---|---|
| `Tab` / `Shift+Tab` | Switch between panels/tabs |
| `q` | Quit |
| `?` | Toggle help overlay |

### Input Mode Detection

When a text input is focused (search box, bookmark note, filter):
- All single-letter shortcuts are **disabled**
- Only `Esc` (blur/cancel), `Enter` (submit), and `Ctrl+` combos work
- Status bar changes to show input-relevant hints

### Panel-Specific Keys

**Search** (results focused):
`↑/↓` navigate, `Enter` select, `Space` toggle multi-select, `d` download, `b` bookmark, `s` cycle source, `m` load more, `i` details, `/` focus search input

**Downloads**:
`↑/↓` navigate, `p` pause, `r` resume, `x` cancel, `Tab` switch between list/detail panes

**Queue**:
`↑/↓` navigate, `t` move to top, `B` move to bottom, `d` download now, `a` download all, `x` remove

**Bookmarks**:
`↑/↓` navigate, `d` download, `a` download all, `x` remove, `n` add/edit note, `o` open URL, `/` filter

**History**:
`↑/↓` navigate, `Enter` re-run search, `x` delete, `c` clear all, `/` filter

## Data Flow

### Search Flow

1. User types query in search input, presses `Enter`
2. Dashboard creates `search.Searcher` and calls `Search()` in a background goroutine
3. Spinner shows during search. On completion, `searchResultsMsg` sent via `tea.Cmd`
4. Results rendered in list. Search saved to history via `db.AddSearchHistory()`
5. Cache checked first if enabled (existing `db.GetCachedSearch()` logic)

### Download Flow

1. User selects book(s) and presses `d`
2. Download record created via `db.CreateDownload()`
3. `downloader.Manager.StartDownload()` launched in background goroutine
4. Progress polled every 500ms via `tea.Tick` reading from DB (`db.GetDownload()`)
5. On completion/failure, status updated and notification sent (existing `notify` package)

### Cross-Panel Navigation

- Downloading from Search: creates download record, switches to Downloads tab
- Downloading from Queue: removes from queue view, appears in Downloads
- Re-running from History: switches to Search tab, pre-fills and executes query
- Downloading from Bookmarks: creates download record, switches to Downloads tab

## Performance

- **Pre-computed styles**: All `lipgloss.Style` objects created once at startup in `styles.go`
- **`strings.Builder`**: All `View()` methods use `strings.Builder` with `Grow()` pre-allocation
- **500ms tick**: Download progress polled at 500ms — not every frame
- **Viewport virtualization**: Lists only render visible rows (handled by bubbles `list` component)
- **Text truncation**: All strings truncated before rendering to prevent auto-wrap
- **Weight-based layout**: Proportional sizing, instant resize on `WindowSizeMsg`

## Error Handling

- Network errors during search: show error message in search panel, allow retry
- Download failures: update status in downloads panel, allow restart
- DB errors: show error in status bar, degrade gracefully
- Background goroutine panics: `defer/recover` wrapper, log error, continue TUI operation

## Pre-Flight Checklist

- [ ] Handle `tea.WindowSizeMsg` — resize all components
- [ ] Handle `ctrl+c` — cleanup, restore terminal state
- [ ] Detect piped stdin/stdout — fall back to plain text
- [ ] Test on 80x24 minimum terminal
- [ ] Provide `NO_TUI` env var escape hatch
- [ ] Test with both light and dark terminal backgrounds
- [ ] Test with `NO_COLOR=1` and `TERM=dumb`
- [ ] Ensure existing CLI subcommands still work unchanged
