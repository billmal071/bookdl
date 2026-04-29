# bookdl TUI Dashboard Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a full-screen interactive TUI dashboard to bookdl, launched by default when running `bookdl` with no arguments.

**Architecture:** A root bubbletea model manages a tab bar and routes to five panel sub-models (search, downloads, queue, bookmarks, history). Each panel is an independent `tea.Model` composed into the root. The Downloads panel uses a split pane with weight-based sizing. All styling uses `lipgloss.AdaptiveColor` for light/dark terminal support.

**Tech Stack:** bubbletea v0.25.0, bubbles v0.18.0, lipgloss v0.9.1 (all already in go.mod). No new dependencies.

**Spec:** `docs/superpowers/specs/2026-04-29-bookdl-tui-design.md`

---

## File Structure

```
internal/tui/dashboard/
  theme.go          — Semantic color theme with AdaptiveColor definitions
  styles.go         — Pre-computed lipgloss styles (allocated once at startup)
  helpers.go        — Text truncation, progress bar rendering, size formatting
  tabs.go           — Tab enum, tab bar rendering
  keys.go           — Focus state machine, key binding definitions
  layout.go         — Adaptive layout engine, weight-based panel sizing
  search.go         — Search panel model (input + results list)
  downloads.go      — Downloads panel model (split pane: list + detail)
  queue.go          — Queue panel model
  bookmarks.go      — Bookmarks panel model
  history.go        — History panel model
  model.go          — Root dashboard model, Init/Update/View, tab routing
```

Modified files:
```
internal/cli/root.go    — Add `tui` subcommand, make no-args launch TUI
cmd/bookdl/main.go      — No changes needed (already calls cli.Execute)
```

---

### Task 1: Theme and Styles

**Files:**
- Create: `internal/tui/dashboard/theme.go`
- Create: `internal/tui/dashboard/styles.go`

- [ ] **Step 1: Create theme.go with semantic AdaptiveColor definitions**

```go
package dashboard

import "github.com/charmbracelet/lipgloss"

// Theme defines semantic colors that adapt to light/dark terminals.
type Theme struct {
	// Status
	Downloading lipgloss.AdaptiveColor
	Paused      lipgloss.AdaptiveColor
	Completed   lipgloss.AdaptiveColor
	Failed      lipgloss.AdaptiveColor
	Pending     lipgloss.AdaptiveColor

	// UI chrome
	Primary   lipgloss.AdaptiveColor
	Secondary lipgloss.AdaptiveColor
	Text      lipgloss.AdaptiveColor
	Subtext   lipgloss.AdaptiveColor
	Border    lipgloss.AdaptiveColor
	Highlight lipgloss.AdaptiveColor
}

var DefaultTheme = Theme{
	Downloading: lipgloss.AdaptiveColor{Light: "#e65100", Dark: "#ffb86c"},
	Paused:      lipgloss.AdaptiveColor{Light: "#f9a825", Dark: "#f1fa8c"},
	Completed:   lipgloss.AdaptiveColor{Light: "#2e7d32", Dark: "#50fa7b"},
	Failed:      lipgloss.AdaptiveColor{Light: "#c62828", Dark: "#ff5555"},
	Pending:     lipgloss.AdaptiveColor{Light: "#757575", Dark: "#6272a4"},

	Primary:   lipgloss.AdaptiveColor{Light: "#7c4dff", Dark: "#bd93f9"},
	Secondary: lipgloss.AdaptiveColor{Light: "#1565c0", Dark: "#8be9fd"},
	Text:      lipgloss.AdaptiveColor{Light: "#1a1a2e", Dark: "#f8f8f2"},
	Subtext:   lipgloss.AdaptiveColor{Light: "#757575", Dark: "#6272a4"},
	Border:    lipgloss.AdaptiveColor{Light: "#bdbdbd", Dark: "#44475a"},
	Highlight: lipgloss.AdaptiveColor{Light: "#7c4dff", Dark: "#bd93f9"},
}
```

- [ ] **Step 2: Create styles.go with pre-computed styles**

```go
package dashboard

import "github.com/charmbracelet/lipgloss"

// Styles holds all pre-computed lipgloss styles. Allocated once at startup.
type Styles struct {
	// Tab bar
	AppName       lipgloss.Style
	ActiveTab     lipgloss.Style
	InactiveTab   lipgloss.Style
	TabSeparator  lipgloss.Style

	// List items
	SelectedItem  lipgloss.Style
	NormalItem    lipgloss.Style
	ItemMeta      lipgloss.Style
	Cursor        lipgloss.Style

	// Status
	StatusDownloading lipgloss.Style
	StatusPaused      lipgloss.Style
	StatusCompleted   lipgloss.Style
	StatusFailed      lipgloss.Style
	StatusPending     lipgloss.Style

	// Panels
	FocusedBorder lipgloss.Style
	BlurredBorder lipgloss.Style

	// Detail pane
	Label lipgloss.Style
	Value lipgloss.Style

	// Status bar
	HelpKey  lipgloss.Style
	HelpDesc lipgloss.Style

	// Search input
	SearchBorderFocused lipgloss.Style
	SearchBorderBlurred lipgloss.Style

	// Misc
	Error   lipgloss.Style
	Warning lipgloss.Style
	Success lipgloss.Style
	Spinner lipgloss.Style
}

// NewStyles creates all styles from the theme. Call once at startup.
func NewStyles(t Theme) Styles {
	return Styles{
		AppName:     lipgloss.NewStyle().Bold(true).Foreground(t.Secondary),
		ActiveTab:   lipgloss.NewStyle().Bold(true).Foreground(t.Primary),
		InactiveTab: lipgloss.NewStyle().Foreground(t.Subtext),
		TabSeparator: lipgloss.NewStyle().Foreground(t.Border),

		SelectedItem: lipgloss.NewStyle().Bold(true).Foreground(t.Primary),
		NormalItem:   lipgloss.NewStyle().Foreground(t.Text),
		ItemMeta:     lipgloss.NewStyle().Foreground(t.Subtext),
		Cursor:       lipgloss.NewStyle().Bold(true).Foreground(t.Primary),

		StatusDownloading: lipgloss.NewStyle().Foreground(t.Downloading),
		StatusPaused:      lipgloss.NewStyle().Foreground(t.Paused),
		StatusCompleted:   lipgloss.NewStyle().Foreground(t.Completed),
		StatusFailed:      lipgloss.NewStyle().Foreground(t.Failed),
		StatusPending:     lipgloss.NewStyle().Foreground(t.Pending),

		FocusedBorder: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(t.Primary),
		BlurredBorder: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(t.Border),

		Label: lipgloss.NewStyle().Foreground(t.Secondary).Bold(true),
		Value: lipgloss.NewStyle().Foreground(t.Text),

		HelpKey:  lipgloss.NewStyle().Bold(true).Foreground(t.Primary),
		HelpDesc: lipgloss.NewStyle().Foreground(t.Subtext),

		SearchBorderFocused: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(t.Primary).
			Padding(0, 1),
		SearchBorderBlurred: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(t.Border).
			Padding(0, 1),

		Error:   lipgloss.NewStyle().Foreground(t.Failed),
		Warning: lipgloss.NewStyle().Foreground(t.Paused),
		Success: lipgloss.NewStyle().Foreground(t.Completed),
		Spinner: lipgloss.NewStyle().Foreground(t.Secondary),
	}
}
```

- [ ] **Step 3: Verify it compiles**

Run: `cd /home/williams/Documents/personal/bookdl && go build ./internal/tui/dashboard/`
Expected: No errors

- [ ] **Step 4: Commit**

```bash
git add internal/tui/dashboard/theme.go internal/tui/dashboard/styles.go
git commit -m "feat(tui): add semantic theme and pre-computed styles"
```

---

### Task 2: Helpers (truncation, progress bars, formatting)

**Files:**
- Create: `internal/tui/dashboard/helpers.go`

- [ ] **Step 1: Create helpers.go**

```go
package dashboard

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// truncate cuts a string to maxLen, appending "…" if truncated.
func truncate(s string, maxLen int) string {
	if maxLen <= 0 {
		return ""
	}
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	if maxLen <= 1 {
		return "…"
	}
	return string(runes[:maxLen-1]) + "…"
}

// renderProgressBar draws a progress bar: ████▒▒▒▒ 62%
// width is the total character width of the bar (excluding the percentage).
func renderProgressBar(percent float64, width int, fillStyle, emptyStyle lipgloss.Style) string {
	if width <= 0 {
		return ""
	}
	if percent < 0 {
		percent = 0
	}
	if percent > 100 {
		percent = 100
	}
	filled := int(percent / 100 * float64(width))
	if filled > width {
		filled = width
	}
	empty := width - filled

	bar := fillStyle.Render(strings.Repeat("█", filled)) +
		emptyStyle.Render(strings.Repeat("▒", empty))

	return fmt.Sprintf("%s %3.0f%%", bar, percent)
}

// formatSize converts bytes to human-readable format.
func formatSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// formatSpeed converts bytes/sec to human-readable format.
func formatSpeed(bytesPerSec float64) string {
	if bytesPerSec <= 0 {
		return "0 B/s"
	}
	return formatSize(int64(bytesPerSec)) + "/s"
}

// statusIcon returns a text glyph and style for a download status.
func statusIcon(status string, s Styles) (string, lipgloss.Style) {
	switch status {
	case "downloading":
		return "⬇", s.StatusDownloading
	case "paused":
		return "‖", s.StatusPaused
	case "completed":
		return "✓", s.StatusCompleted
	case "failed":
		return "✗", s.StatusFailed
	default:
		return "○", s.StatusPending
	}
}

// joinMeta joins non-empty strings with " · " separator.
func joinMeta(parts ...string) string {
	var nonEmpty []string
	for _, p := range parts {
		if p != "" {
			nonEmpty = append(nonEmpty, p)
		}
	}
	return strings.Join(nonEmpty, "  ·  ")
}
```

- [ ] **Step 2: Verify it compiles**

Run: `cd /home/williams/Documents/personal/bookdl && go build ./internal/tui/dashboard/`
Expected: No errors

- [ ] **Step 3: Commit**

```bash
git add internal/tui/dashboard/helpers.go
git commit -m "feat(tui): add helper functions for truncation, progress bars, formatting"
```

---

### Task 3: Tab Bar and Focus State Machine

**Files:**
- Create: `internal/tui/dashboard/tabs.go`
- Create: `internal/tui/dashboard/keys.go`

- [ ] **Step 1: Create tabs.go**

```go
package dashboard

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

type tab int

const (
	tabSearch tab = iota
	tabDownloads
	tabQueue
	tabBookmarks
	tabHistory
	tabCount // sentinel for wrapping
)

var tabNames = [tabCount]string{
	"Search",
	"Downloads",
	"Queue",
	"Bookmarks",
	"History",
}

func (t tab) String() string {
	if t >= 0 && t < tabCount {
		return tabNames[t]
	}
	return "?"
}

func (t tab) next() tab {
	return (t + 1) % tabCount
}

func (t tab) prev() tab {
	return (t - 1 + tabCount) % tabCount
}

// renderTabBar renders the tab bar line.
// width is the available terminal width.
func renderTabBar(active tab, s Styles, width int) string {
	var b strings.Builder

	// App name
	b.WriteString(s.AppName.Render("bookdl"))
	b.WriteString("    ")

	for i := tab(0); i < tabCount; i++ {
		name := i.String()
		if i == active {
			b.WriteString(s.ActiveTab.Render(name))
			b.WriteString(s.ActiveTab.Render(" ━━"))
		} else {
			b.WriteString(s.InactiveTab.Render(name))
		}
		if i < tabCount-1 {
			b.WriteString("    ")
		}
	}

	line := b.String()
	separator := s.TabSeparator.Render(strings.Repeat("─", width))

	return line + "\n" + separator
}
```

- [ ] **Step 2: Create keys.go with focus state machine**

```go
package dashboard

// focus represents which UI element has keyboard focus.
type focus int

const (
	focusSearchInput focus = iota
	focusSearchResults
	focusDownloadList
	focusDownloadDetail
	focusQueueList
	focusBookmarkList
	focusBookmarkNote
	focusHistoryList
	focusConfirmDialog
)

// isTextInput returns true when focus is on a text input field.
// When true, single-letter shortcuts are disabled.
func (f focus) isTextInput() bool {
	switch f {
	case focusSearchInput, focusBookmarkNote:
		return true
	}
	return false
}

// panelTab returns which tab a focus state belongs to.
func (f focus) panelTab() tab {
	switch f {
	case focusSearchInput, focusSearchResults:
		return tabSearch
	case focusDownloadList, focusDownloadDetail:
		return tabDownloads
	case focusQueueList:
		return tabQueue
	case focusBookmarkList, focusBookmarkNote:
		return tabBookmarks
	case focusHistoryList:
		return tabHistory
	default:
		return tabSearch
	}
}

// defaultFocusForTab returns the default focus state when switching to a tab.
func defaultFocusForTab(t tab) focus {
	switch t {
	case tabSearch:
		return focusSearchInput
	case tabDownloads:
		return focusDownloadList
	case tabQueue:
		return focusQueueList
	case tabBookmarks:
		return focusBookmarkList
	case tabHistory:
		return focusHistoryList
	default:
		return focusSearchInput
	}
}
```

- [ ] **Step 3: Verify it compiles**

Run: `cd /home/williams/Documents/personal/bookdl && go build ./internal/tui/dashboard/`
Expected: No errors

- [ ] **Step 4: Commit**

```bash
git add internal/tui/dashboard/tabs.go internal/tui/dashboard/keys.go
git commit -m "feat(tui): add tab bar component and focus state machine"
```

---

### Task 4: Layout Engine

**Files:**
- Create: `internal/tui/dashboard/layout.go`

- [ ] **Step 1: Create layout.go**

```go
package dashboard

// layout holds computed dimensions for the current terminal size.
type layout struct {
	width         int
	height        int
	contentWidth  int
	contentHeight int
	splitView     bool // true when Downloads can show split pane
}

const (
	tabBarHeight   = 2 // tab names + separator
	statusBarHeight = 1
	panelBorderH   = 2 // top + bottom border

	splitViewMinWidth = 120
)

// computeLayout calculates available space from terminal dimensions.
func computeLayout(termWidth, termHeight int) layout {
	l := layout{
		width:  termWidth,
		height: termHeight,
	}
	l.contentWidth = termWidth
	l.contentHeight = termHeight - tabBarHeight - statusBarHeight - panelBorderH
	if l.contentHeight < 1 {
		l.contentHeight = 1
	}
	l.splitView = termWidth >= splitViewMinWidth
	return l
}

// splitPaneWidths returns list and detail pane widths using weight-based sizing.
// List weight 2, detail weight 3. -1 for divider.
func (l layout) splitPaneWidths() (int, int) {
	if !l.splitView {
		return l.contentWidth, 0
	}
	available := l.contentWidth - 1 // -1 for divider
	listWeight, detailWeight := 2, 3
	totalWeight := listWeight + detailWeight
	listW := (available * listWeight) / totalWeight
	detailW := available - listW
	return listW, detailW
}
```

- [ ] **Step 2: Verify it compiles**

Run: `cd /home/williams/Documents/personal/bookdl && go build ./internal/tui/dashboard/`
Expected: No errors

- [ ] **Step 3: Commit**

```bash
git add internal/tui/dashboard/layout.go
git commit -m "feat(tui): add adaptive layout engine with weight-based sizing"
```

---

### Task 5: History Panel

**Files:**
- Create: `internal/tui/dashboard/history.go`

Starting with the simplest panel (read-only list from DB) to validate the panel pattern before the more complex ones.

- [ ] **Step 1: Create history.go**

```go
package dashboard

import (
	"fmt"
	"strings"

	"github.com/billmal071/bookdl/internal/db"
	tea "github.com/charmbracelet/bubbletea"
)

// historyLoadedMsg carries loaded search history.
type historyLoadedMsg struct {
	history []*db.SearchHistory
	err     error
}

// historyDeleteMsg signals a history entry was deleted.
type historyDeleteMsg struct {
	err error
}

// historyClearMsg signals all history was cleared.
type historyClearMsg struct {
	err error
}

// rerunSearchMsg tells the root model to switch to search with this query.
type rerunSearchMsg struct {
	query   string
	filters db.SearchFilters
}

type historyModel struct {
	items    []*db.SearchHistory
	cursor   int
	styles   Styles
	width    int
	height   int
	err      error
	loaded   bool
	confirm  bool // confirming clear all
}

func newHistoryModel(s Styles) historyModel {
	return historyModel{styles: s}
}

func (m historyModel) Init() tea.Cmd {
	return loadHistory
}

func loadHistory() tea.Msg {
	history, err := db.GetUniqueSearchHistory(50)
	return historyLoadedMsg{history: history, err: err}
}

func (m historyModel) Update(msg tea.Msg) (historyModel, tea.Cmd) {
	switch msg := msg.(type) {
	case historyLoadedMsg:
		m.loaded = true
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		m.items = msg.history
		m.clampCursor()
		return m, nil

	case historyDeleteMsg:
		if msg.err != nil {
			m.err = msg.err
		}
		return m, loadHistory

	case historyClearMsg:
		m.confirm = false
		if msg.err != nil {
			m.err = msg.err
		}
		return m, loadHistory

	case tea.KeyMsg:
		if m.confirm {
			switch msg.String() {
			case "y", "Y":
				m.confirm = false
				return m, func() tea.Msg {
					err := db.ClearSearchHistory()
					return historyClearMsg{err: err}
				}
			default:
				m.confirm = false
				return m, nil
			}
		}

		switch msg.String() {
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.items)-1 {
				m.cursor++
			}
		case "enter":
			if m.cursor < len(m.items) {
				h := m.items[m.cursor]
				return m, func() tea.Msg {
					return rerunSearchMsg{query: h.Query, filters: h.Filters}
				}
			}
		case "x":
			if m.cursor < len(m.items) {
				h := m.items[m.cursor]
				return m, func() tea.Msg {
					err := db.DeleteSearchHistory(h.ID)
					return historyDeleteMsg{err: err}
				}
			}
		case "c":
			if len(m.items) > 0 {
				m.confirm = true
			}
		}
	}
	return m, nil
}

func (m historyModel) View(l layout) string {
	var b strings.Builder
	b.Grow(1024)

	if !m.loaded {
		b.WriteString(m.styles.Spinner.Render("  Loading history..."))
		return b.String()
	}

	if m.err != nil {
		b.WriteString(m.styles.Error.Render(fmt.Sprintf("  Error: %v", m.err)))
		return b.String()
	}

	if m.confirm {
		b.WriteString(m.styles.Warning.Render("  Clear all history? (y/N)"))
		return b.String()
	}

	if len(m.items) == 0 {
		b.WriteString(m.styles.ItemMeta.Render("  No search history."))
		return b.String()
	}

	maxW := l.contentWidth - 6 // padding + cursor

	b.WriteString(m.styles.Label.Render(fmt.Sprintf("  Search History (%d):", len(m.items))))
	b.WriteString("\n\n")

	visible := l.contentHeight - 3 // header + gap
	if visible < 1 {
		visible = 1
	}

	start := 0
	if m.cursor >= visible {
		start = m.cursor - visible + 1
	}
	end := start + visible
	if end > len(m.items) {
		end = len(m.items)
	}

	for i := start; i < end; i++ {
		h := m.items[i]
		query := truncate(fmt.Sprintf("\"%s\"", h.Query), maxW)

		if i == m.cursor {
			b.WriteString(m.styles.SelectedItem.Render(fmt.Sprintf("  ▸ %d. %s", i+1, query)))
		} else {
			b.WriteString(m.styles.NormalItem.Render(fmt.Sprintf("    %d. %s", i+1, query)))
		}
		b.WriteString("\n")

		// Meta line
		meta := joinMeta(
			fmt.Sprintf("%d results", h.ResultCount),
			filterString(h.Filters),
			h.CreatedAt.Format("2006-01-02 15:04"),
		)
		b.WriteString(m.styles.ItemMeta.Render(fmt.Sprintf("       %s", truncate(meta, maxW))))
		b.WriteString("\n")
	}

	return b.String()
}

func filterString(f db.SearchFilters) string {
	var parts []string
	if f.Format != "" {
		parts = append(parts, "format="+f.Format)
	}
	if f.Language != "" {
		parts = append(parts, "language="+f.Language)
	}
	if f.Year != "" {
		parts = append(parts, "year="+f.Year)
	}
	if f.MaxSize != "" {
		parts = append(parts, "max-size="+f.MaxSize)
	}
	return strings.Join(parts, ", ")
}

func (m *historyModel) clampCursor() {
	if m.cursor >= len(m.items) {
		m.cursor = len(m.items) - 1
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
}

func (m *historyModel) setSize(w, h int) {
	m.width = w
	m.height = h
}
```

- [ ] **Step 2: Check if db.DeleteSearchHistory and db.ClearSearchHistory exist**

Run: `cd /home/williams/Documents/personal/bookdl && grep -n "func DeleteSearchHistory\|func ClearSearchHistory" internal/db/*.go`

If they don't exist, add them to `internal/db/search_history.go`:

```go
// DeleteSearchHistory deletes a single search history entry by ID.
func DeleteSearchHistory(id int64) error {
	_, err := database.Exec("DELETE FROM search_history WHERE id = ?", id)
	return err
}

// ClearSearchHistory deletes all search history entries.
func ClearSearchHistory() error {
	_, err := database.Exec("DELETE FROM search_history")
	return err
}
```

- [ ] **Step 3: Verify it compiles**

Run: `cd /home/williams/Documents/personal/bookdl && go build ./internal/tui/dashboard/`
Expected: No errors

- [ ] **Step 4: Commit**

```bash
git add internal/tui/dashboard/history.go internal/db/search_history.go
git commit -m "feat(tui): add history panel with re-run and delete support"
```

---

### Task 6: Queue Panel

**Files:**
- Create: `internal/tui/dashboard/queue.go`

- [ ] **Step 1: Create queue.go**

```go
package dashboard

import (
	"fmt"
	"strings"

	"github.com/billmal071/bookdl/internal/db"
	tea "github.com/charmbracelet/bubbletea"
)

type queueLoadedMsg struct {
	downloads []*db.Download
	err       error
}

type queueActionMsg struct {
	err error
}

// startDownloadMsg tells the root model to start downloading this item.
type startDownloadMsg struct {
	download *db.Download
}

// startAllDownloadsMsg tells the root model to start all queued downloads.
type startAllDownloadsMsg struct{}

type queueModel struct {
	items  []*db.Download
	cursor int
	styles Styles
	width  int
	height int
	err    error
	loaded bool
}

func newQueueModel(s Styles) queueModel {
	return queueModel{styles: s}
}

func (m queueModel) Init() tea.Cmd {
	return loadQueue
}

func loadQueue() tea.Msg {
	downloads, err := db.ListDownloads(db.StatusPending, true)
	return queueLoadedMsg{downloads: downloads, err: err}
}

func (m queueModel) Update(msg tea.Msg) (queueModel, tea.Cmd) {
	switch msg := msg.(type) {
	case queueLoadedMsg:
		m.loaded = true
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		m.items = msg.downloads
		m.clampCursor()
		return m, nil

	case queueActionMsg:
		if msg.err != nil {
			m.err = msg.err
		}
		return m, loadQueue

	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.items)-1 {
				m.cursor++
			}
		case "t":
			if m.cursor < len(m.items) {
				d := m.items[m.cursor]
				return m, func() tea.Msg {
					err := db.SetPriorityTop(d.ID)
					return queueActionMsg{err: err}
				}
			}
		case "B":
			if m.cursor < len(m.items) {
				d := m.items[m.cursor]
				return m, func() tea.Msg {
					err := db.SetPriorityBottom(d.ID)
					return queueActionMsg{err: err}
				}
			}
		case "x":
			if m.cursor < len(m.items) {
				d := m.items[m.cursor]
				return m, func() tea.Msg {
					err := db.DeleteDownload(d.ID)
					return queueActionMsg{err: err}
				}
			}
		case "d":
			if m.cursor < len(m.items) {
				d := m.items[m.cursor]
				return m, func() tea.Msg {
					return startDownloadMsg{download: d}
				}
			}
		case "a":
			if len(m.items) > 0 {
				return m, func() tea.Msg {
					return startAllDownloadsMsg{}
				}
			}
		}
	}
	return m, nil
}

func (m queueModel) View(l layout) string {
	var b strings.Builder
	b.Grow(1024)

	if !m.loaded {
		b.WriteString(m.styles.Spinner.Render("  Loading queue..."))
		return b.String()
	}

	if m.err != nil {
		b.WriteString(m.styles.Error.Render(fmt.Sprintf("  Error: %v", m.err)))
		return b.String()
	}

	if len(m.items) == 0 {
		b.WriteString(m.styles.ItemMeta.Render("  Queue is empty."))
		return b.String()
	}

	maxW := l.contentWidth - 6

	b.WriteString(m.styles.Label.Render(fmt.Sprintf("  Queue (%d pending):", len(m.items))))
	b.WriteString("\n\n")

	visible := l.contentHeight - 3
	if visible < 1 {
		visible = 1
	}

	start := 0
	if m.cursor >= visible {
		start = m.cursor - visible + 1
	}
	end := start + visible
	if end > len(m.items) {
		end = len(m.items)
	}

	for i := start; i < end; i++ {
		d := m.items[i]
		title := truncate(d.Title, maxW)

		priorityBadge := ""
		if d.Priority != 0 {
			priorityBadge = fmt.Sprintf("[P:%d] ", d.Priority)
		}

		if i == m.cursor {
			b.WriteString(m.styles.SelectedItem.Render(fmt.Sprintf("  ▸ %d. %s%s", i+1, priorityBadge, title)))
		} else {
			b.WriteString(m.styles.NormalItem.Render(fmt.Sprintf("    %d. %s%s", i+1, priorityBadge, title)))
		}
		b.WriteString("\n")

		// Meta line
		var sizePart string
		if d.FileSize > 0 {
			sizePart = formatSize(d.FileSize)
		}
		meta := joinMeta(d.Authors, d.Format, sizePart)
		if meta != "" {
			b.WriteString(m.styles.ItemMeta.Render(fmt.Sprintf("       %s", truncate(meta, maxW))))
			b.WriteString("\n")
		}
	}

	return b.String()
}

func (m *queueModel) clampCursor() {
	if m.cursor >= len(m.items) {
		m.cursor = len(m.items) - 1
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
}

func (m *queueModel) setSize(w, h int) {
	m.width = w
	m.height = h
}
```

- [ ] **Step 2: Verify it compiles**

Run: `cd /home/williams/Documents/personal/bookdl && go build ./internal/tui/dashboard/`
Expected: No errors

- [ ] **Step 3: Commit**

```bash
git add internal/tui/dashboard/queue.go
git commit -m "feat(tui): add queue panel with priority reorder and download actions"
```

---

### Task 7: Bookmarks Panel

**Files:**
- Create: `internal/tui/dashboard/bookmarks.go`

- [ ] **Step 1: Create bookmarks.go**

```go
package dashboard

import (
	"fmt"
	"strings"

	"github.com/billmal071/bookdl/internal/db"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

type bookmarksLoadedMsg struct {
	bookmarks []*db.Bookmark
	err       error
}

type bookmarkActionMsg struct {
	err error
}

// bookmarkDownloadMsg tells root model to download this bookmark.
type bookmarkDownloadMsg struct {
	md5Hash string
}

// bookmarkDownloadAllMsg tells root model to download all bookmarks.
type bookmarkDownloadAllMsg struct{}

type bookmarksModel struct {
	items     []*db.Bookmark
	cursor    int
	styles    Styles
	width     int
	height    int
	err       error
	loaded    bool
	editing   bool          // editing a note
	noteInput textinput.Model
	filter    string
	filtering bool
	filterInput textinput.Model
}

func newBookmarksModel(s Styles) bookmarksModel {
	ni := textinput.New()
	ni.Placeholder = "Enter note..."
	ni.CharLimit = 200

	fi := textinput.New()
	fi.Placeholder = "Filter bookmarks..."
	fi.CharLimit = 100

	return bookmarksModel{
		styles:      s,
		noteInput:   ni,
		filterInput: fi,
	}
}

func (m bookmarksModel) Init() tea.Cmd {
	return loadBookmarks
}

func loadBookmarks() tea.Msg {
	bookmarks, err := db.ListBookmarks()
	return bookmarksLoadedMsg{bookmarks: bookmarks, err: err}
}

func (m bookmarksModel) Update(msg tea.Msg) (bookmarksModel, tea.Cmd) {
	switch msg := msg.(type) {
	case bookmarksLoadedMsg:
		m.loaded = true
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		m.items = msg.bookmarks
		m.clampCursor()
		return m, nil

	case bookmarkActionMsg:
		if msg.err != nil {
			m.err = msg.err
		}
		return m, loadBookmarks

	case tea.KeyMsg:
		// Handle note editing mode
		if m.editing {
			switch msg.String() {
			case "enter":
				if m.cursor < len(m.items) {
					bm := m.items[m.cursor]
					note := m.noteInput.Value()
					m.editing = false
					m.noteInput.Blur()
					return m, func() tea.Msg {
						err := db.UpdateBookmarkNotes(bm.ID, note)
						return bookmarkActionMsg{err: err}
					}
				}
				m.editing = false
				m.noteInput.Blur()
				return m, nil
			case "esc":
				m.editing = false
				m.noteInput.Blur()
				return m, nil
			default:
				var cmd tea.Cmd
				m.noteInput, cmd = m.noteInput.Update(msg)
				return m, cmd
			}
		}

		// Handle filter mode
		if m.filtering {
			switch msg.String() {
			case "enter", "esc":
				m.filter = m.filterInput.Value()
				m.filtering = false
				m.filterInput.Blur()
				return m, nil
			default:
				var cmd tea.Cmd
				m.filterInput, cmd = m.filterInput.Update(msg)
				m.filter = m.filterInput.Value()
				m.clampCursor()
				return m, cmd
			}
		}

		switch msg.String() {
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.filtered())-1 {
				m.cursor++
			}
		case "x":
			items := m.filtered()
			if m.cursor < len(items) {
				bm := items[m.cursor]
				return m, func() tea.Msg {
					err := db.DeleteBookmarkByHash(bm.MD5Hash)
					return bookmarkActionMsg{err: err}
				}
			}
		case "d":
			items := m.filtered()
			if m.cursor < len(items) {
				bm := items[m.cursor]
				return m, func() tea.Msg {
					return bookmarkDownloadMsg{md5Hash: bm.MD5Hash}
				}
			}
		case "a":
			if len(m.items) > 0 {
				return m, func() tea.Msg {
					return bookmarkDownloadAllMsg{}
				}
			}
		case "n":
			items := m.filtered()
			if m.cursor < len(items) {
				bm := items[m.cursor]
				m.editing = true
				m.noteInput.SetValue(bm.Notes)
				m.noteInput.Focus()
				return m, textinput.Blink
			}
		case "/":
			m.filtering = true
			m.filterInput.SetValue(m.filter)
			m.filterInput.Focus()
			return m, textinput.Blink
		}
	}
	return m, nil
}

func (m bookmarksModel) filtered() []*db.Bookmark {
	if m.filter == "" {
		return m.items
	}
	f := strings.ToLower(m.filter)
	var result []*db.Bookmark
	for _, bm := range m.items {
		if strings.Contains(strings.ToLower(bm.Title), f) ||
			strings.Contains(strings.ToLower(bm.Authors), f) {
			result = append(result, bm)
		}
	}
	return result
}

func (m bookmarksModel) View(l layout) string {
	var b strings.Builder
	b.Grow(1024)

	if !m.loaded {
		b.WriteString(m.styles.Spinner.Render("  Loading bookmarks..."))
		return b.String()
	}

	if m.err != nil {
		b.WriteString(m.styles.Error.Render(fmt.Sprintf("  Error: %v", m.err)))
		return b.String()
	}

	items := m.filtered()

	if len(m.items) == 0 {
		b.WriteString(m.styles.ItemMeta.Render("  No bookmarks saved."))
		return b.String()
	}

	maxW := l.contentWidth - 6

	// Filter input
	if m.filtering {
		b.WriteString(m.styles.SearchBorderFocused.Render(m.filterInput.View()))
		b.WriteString("\n\n")
	} else if m.filter != "" {
		b.WriteString(m.styles.ItemMeta.Render(fmt.Sprintf("  Filter: %s (/ to change)", m.filter)))
		b.WriteString("\n\n")
	}

	b.WriteString(m.styles.Label.Render(fmt.Sprintf("  Bookmarks (%d):", len(items))))
	b.WriteString("\n\n")

	visible := l.contentHeight - 5
	if m.filtering || m.filter != "" {
		visible -= 2
	}
	if visible < 1 {
		visible = 1
	}

	start := 0
	if m.cursor >= visible {
		start = m.cursor - visible + 1
	}
	end := start + visible
	if end > len(items) {
		end = len(items)
	}

	for i := start; i < end; i++ {
		bm := items[i]
		title := truncate(bm.Title, maxW)

		if i == m.cursor {
			b.WriteString(m.styles.SelectedItem.Render(fmt.Sprintf("  ▸ %d. %s", i+1, title)))
		} else {
			b.WriteString(m.styles.NormalItem.Render(fmt.Sprintf("    %d. %s", i+1, title)))
		}
		b.WriteString("\n")

		// Meta
		meta := joinMeta(bm.Authors, bm.Format, bm.Size)
		if meta != "" {
			b.WriteString(m.styles.ItemMeta.Render(fmt.Sprintf("       %s", truncate(meta, maxW))))
			b.WriteString("\n")
		}

		// Date + note
		dateLine := "       Added: " + bm.CreatedAt.Format("2006-01-02")
		if bm.Notes != "" {
			dateLine += fmt.Sprintf("  Note: \"%s\"", truncate(bm.Notes, maxW-30))
		}
		b.WriteString(m.styles.ItemMeta.Render(truncate(dateLine, maxW+7)))
		b.WriteString("\n")

		// Note editor
		if m.editing && i == m.cursor {
			b.WriteString("       ")
			b.WriteString(m.noteInput.View())
			b.WriteString("\n")
		}
	}

	return b.String()
}

func (m *bookmarksModel) clampCursor() {
	items := m.filtered()
	if m.cursor >= len(items) {
		m.cursor = len(items) - 1
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
}

func (m *bookmarksModel) setSize(w, h int) {
	m.width = w
	m.height = h
}

func (m bookmarksModel) isFocusedOnInput() bool {
	return m.editing || m.filtering
}
```

- [ ] **Step 2: Verify it compiles**

Run: `cd /home/williams/Documents/personal/bookdl && go build ./internal/tui/dashboard/`
Expected: No errors

- [ ] **Step 3: Commit**

```bash
git add internal/tui/dashboard/bookmarks.go
git commit -m "feat(tui): add bookmarks panel with note editing and filtering"
```

---

### Task 8: Downloads Panel (Split Pane)

**Files:**
- Create: `internal/tui/dashboard/downloads.go`

- [ ] **Step 1: Create downloads.go**

```go
package dashboard

import (
	"fmt"
	"strings"
	"time"

	"github.com/billmal071/bookdl/internal/db"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type downloadsLoadedMsg struct {
	downloads []*db.Download
	err       error
}

type downloadPauseMsg struct{ err error }
type downloadResumeMsg struct{ err error }
type downloadCancelMsg struct{ err error }

// tickDownloadsMsg triggers periodic refresh of download state.
type tickDownloadsMsg time.Time

type downloadsModel struct {
	items       []*db.Download
	cursor      int
	focusDetail bool // true when detail pane has focus
	styles      Styles
	width       int
	height      int
	err         error
	loaded      bool
	// For speed/ETA calculation
	prevSizes map[int64]int64
	prevTime  time.Time
	speeds    map[int64]float64 // bytes per second
}

func newDownloadsModel(s Styles) downloadsModel {
	return downloadsModel{
		styles:    s,
		prevSizes: make(map[int64]int64),
		speeds:    make(map[int64]float64),
		prevTime:  time.Now(),
	}
}

func (m downloadsModel) Init() tea.Cmd {
	return tea.Batch(loadDownloads, tickDownloads())
}

func loadDownloads() tea.Msg {
	downloads, err := db.ListDownloads("", true)
	return downloadsLoadedMsg{downloads: downloads, err: err}
}

func tickDownloads() tea.Cmd {
	return tea.Tick(500*time.Millisecond, func(t time.Time) tea.Msg {
		return tickDownloadsMsg(t)
	})
}

func (m downloadsModel) Update(msg tea.Msg) (downloadsModel, tea.Cmd) {
	switch msg := msg.(type) {
	case downloadsLoadedMsg:
		m.loaded = true
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		m.updateSpeeds(msg.downloads)
		m.items = msg.downloads
		m.clampCursor()
		return m, nil

	case tickDownloadsMsg:
		return m, tea.Batch(loadDownloads, tickDownloads())

	case downloadPauseMsg, downloadResumeMsg, downloadCancelMsg:
		return m, loadDownloads

	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.items)-1 {
				m.cursor++
			}
		case "tab":
			m.focusDetail = !m.focusDetail
			return m, nil
		case "p":
			if m.cursor < len(m.items) {
				d := m.items[m.cursor]
				if d.Status == db.StatusDownloading {
					return m, func() tea.Msg {
						err := db.UpdateStatus(d.ID, db.StatusPaused, "")
						return downloadPauseMsg{err: err}
					}
				}
			}
		case "r":
			if m.cursor < len(m.items) {
				d := m.items[m.cursor]
				if d.Status == db.StatusPaused || d.Status == db.StatusFailed {
					return m, func() tea.Msg {
						err := db.UpdateStatus(d.ID, db.StatusPending, "")
						return downloadResumeMsg{err: err}
					}
				}
			}
		case "x":
			if m.cursor < len(m.items) {
				d := m.items[m.cursor]
				return m, func() tea.Msg {
					err := db.DeleteDownload(d.ID)
					return downloadCancelMsg{err: err}
				}
			}
		}
	}
	return m, nil
}

func (m *downloadsModel) updateSpeeds(downloads []*db.Download) {
	now := time.Now()
	elapsed := now.Sub(m.prevTime).Seconds()
	if elapsed <= 0 {
		elapsed = 0.5
	}

	for _, d := range downloads {
		prev, ok := m.prevSizes[d.ID]
		if ok && d.Status == db.StatusDownloading {
			diff := d.DownloadedSize - prev
			if diff > 0 {
				m.speeds[d.ID] = float64(diff) / elapsed
			}
		}
		m.prevSizes[d.ID] = d.DownloadedSize
	}
	m.prevTime = now
}

func (m downloadsModel) View(l layout) string {
	var b strings.Builder
	b.Grow(2048)

	if !m.loaded {
		b.WriteString(m.styles.Spinner.Render("  Loading downloads..."))
		return b.String()
	}

	if m.err != nil {
		b.WriteString(m.styles.Error.Render(fmt.Sprintf("  Error: %v", m.err)))
		return b.String()
	}

	if len(m.items) == 0 {
		b.WriteString(m.styles.ItemMeta.Render("  No downloads."))
		return b.String()
	}

	if l.splitView {
		return m.viewSplit(l)
	}
	return m.viewSingle(l)
}

func (m downloadsModel) viewSingle(l layout) string {
	var b strings.Builder
	b.Grow(2048)

	maxW := l.contentWidth - 6

	b.WriteString(m.styles.Label.Render(fmt.Sprintf("  Downloads (%d):", len(m.items))))
	b.WriteString("\n\n")

	visible := (l.contentHeight - 3) / 3 // 3 lines per item
	if visible < 1 {
		visible = 1
	}

	start := 0
	if m.cursor >= visible {
		start = m.cursor - visible + 1
	}
	end := start + visible
	if end > len(m.items) {
		end = len(m.items)
	}

	for i := start; i < end; i++ {
		d := m.items[i]
		icon, iconStyle := statusIcon(string(d.Status), m.styles)
		title := truncate(d.Title, maxW-4)

		if i == m.cursor {
			b.WriteString(m.styles.SelectedItem.Render(fmt.Sprintf("  ▸ %s ", icon)))
			b.WriteString(m.styles.SelectedItem.Render(title))
		} else {
			b.WriteString(fmt.Sprintf("    %s ", iconStyle.Render(icon)))
			b.WriteString(m.styles.NormalItem.Render(title))
		}
		b.WriteString("\n")

		// Progress bar
		if d.FileSize > 0 {
			pct := float64(d.DownloadedSize) / float64(d.FileSize) * 100
			fillStyle, emptyStyle := m.progressStyles(d.Status)
			barWidth := maxW - 10
			if barWidth > 30 {
				barWidth = 30
			}
			if barWidth < 10 {
				barWidth = 10
			}
			bar := renderProgressBar(pct, barWidth, fillStyle, emptyStyle)
			b.WriteString(fmt.Sprintf("       %s\n", bar))
		} else {
			b.WriteString(fmt.Sprintf("       %s\n", iconStyle.Render(string(d.Status))))
		}
	}

	return b.String()
}

func (m downloadsModel) viewSplit(l layout) string {
	listW, detailW := l.splitPaneWidths()

	listContent := m.renderList(listW, l.contentHeight)
	detailContent := m.renderDetail(detailW, l.contentHeight)

	listBorder := m.styles.BlurredBorder
	detailBorder := m.styles.BlurredBorder
	if !m.focusDetail {
		listBorder = m.styles.FocusedBorder
	} else {
		detailBorder = m.styles.FocusedBorder
	}

	// Fill content to exact height
	listLines := strings.Split(listContent, "\n")
	innerH := l.contentHeight
	for len(listLines) < innerH {
		listLines = append(listLines, "")
	}
	listLines = listLines[:innerH]

	detailLines := strings.Split(detailContent, "\n")
	for len(detailLines) < innerH {
		detailLines = append(detailLines, "")
	}
	detailLines = detailLines[:innerH]

	leftPanel := listBorder.
		Width(listW - 2). // subtract border
		Render(strings.Join(listLines, "\n"))

	rightPanel := detailBorder.
		Width(detailW - 2).
		Render(strings.Join(detailLines, "\n"))

	return lipgloss.JoinHorizontal(lipgloss.Top, leftPanel, rightPanel)
}

func (m downloadsModel) renderList(width, height int) string {
	var b strings.Builder
	b.Grow(1024)

	maxW := width - 6

	b.WriteString(m.styles.Label.Render(fmt.Sprintf(" Downloads (%d):", len(m.items))))
	b.WriteString("\n\n")

	visible := (height - 3) / 3
	if visible < 1 {
		visible = 1
	}

	start := 0
	if m.cursor >= visible {
		start = m.cursor - visible + 1
	}
	end := start + visible
	if end > len(m.items) {
		end = len(m.items)
	}

	for i := start; i < end; i++ {
		d := m.items[i]
		icon, iconStyle := statusIcon(string(d.Status), m.styles)
		title := truncate(d.Title, maxW-4)

		if i == m.cursor {
			b.WriteString(m.styles.SelectedItem.Render(fmt.Sprintf(" ▸ %s %s", icon, title)))
		} else {
			b.WriteString(fmt.Sprintf("   %s %s", iconStyle.Render(icon), m.styles.NormalItem.Render(title)))
		}
		b.WriteString("\n")

		// Mini progress bar
		if d.FileSize > 0 {
			pct := float64(d.DownloadedSize) / float64(d.FileSize) * 100
			fillStyle, emptyStyle := m.progressStyles(d.Status)
			barW := maxW - 10
			if barW > 20 {
				barW = 20
			}
			if barW < 8 {
				barW = 8
			}
			bar := renderProgressBar(pct, barW, fillStyle, emptyStyle)
			b.WriteString(fmt.Sprintf("     %s\n", bar))
		} else {
			b.WriteString(fmt.Sprintf("     %s\n", iconStyle.Render(string(d.Status))))
		}
	}

	return b.String()
}

func (m downloadsModel) renderDetail(width, height int) string {
	if m.cursor >= len(m.items) {
		return ""
	}

	d := m.items[m.cursor]
	maxW := width - 4

	var b strings.Builder
	b.Grow(512)

	b.WriteString(m.styles.Label.Render(" Details"))
	b.WriteString("\n\n")

	// Metadata
	fields := []struct{ label, value string }{
		{"Title", truncate(d.Title, maxW-12)},
		{"Author", truncate(d.Authors, maxW-12)},
		{"Format", d.Format},
	}
	if d.FileSize > 0 {
		fields = append(fields, struct{ label, value string }{"Size", formatSize(d.FileSize)})
	}
	fields = append(fields, struct{ label, value string }{"Source", d.Source})

	icon, iconStyle := statusIcon(string(d.Status), m.styles)
	fields = append(fields, struct{ label, value string }{"Status", ""})

	for _, f := range fields {
		label := m.styles.Label.Render(fmt.Sprintf(" %-9s", f.label))
		if f.label == "Status" {
			b.WriteString(label + iconStyle.Render(fmt.Sprintf("%s %s", icon, d.Status)))
		} else {
			b.WriteString(label + m.styles.Value.Render(f.value))
		}
		b.WriteString("\n")
	}

	b.WriteString("\n")

	// Large progress bar
	if d.FileSize > 0 {
		pct := float64(d.DownloadedSize) / float64(d.FileSize) * 100
		fillStyle, emptyStyle := m.progressStyles(d.Status)
		barW := maxW - 8
		if barW > 30 {
			barW = 30
		}
		if barW < 10 {
			barW = 10
		}
		bar := renderProgressBar(pct, barW, fillStyle, emptyStyle)
		b.WriteString(fmt.Sprintf(" %s\n", bar))
		b.WriteString(m.styles.ItemMeta.Render(fmt.Sprintf(" %s / %s",
			formatSize(d.DownloadedSize), formatSize(d.FileSize))))
		b.WriteString("\n")

		// Speed and ETA
		speed := m.speeds[d.ID]
		if speed > 0 && d.Status == db.StatusDownloading {
			remaining := float64(d.FileSize-d.DownloadedSize) / speed
			eta := formatETA(remaining)
			b.WriteString(m.styles.Label.Render(" Speed  "))
			b.WriteString(m.styles.Value.Render(formatSpeed(speed)))
			b.WriteString(m.styles.Label.Render("     ETA  "))
			b.WriteString(m.styles.Value.Render(eta))
			b.WriteString("\n")
		}
	}

	// Error message
	if d.ErrorMessage != "" {
		b.WriteString("\n")
		b.WriteString(m.styles.Error.Render(fmt.Sprintf(" Error: %s", truncate(d.ErrorMessage, maxW-8))))
		b.WriteString("\n")
	}

	return b.String()
}

func (m downloadsModel) progressStyles(status db.DownloadStatus) (lipgloss.Style, lipgloss.Style) {
	emptyStyle := lipgloss.NewStyle().Foreground(m.styles.BlurredBorder.GetBorderBottomForeground())
	switch status {
	case db.StatusDownloading:
		return m.styles.StatusDownloading, emptyStyle
	case db.StatusPaused:
		return m.styles.StatusPaused, emptyStyle
	case db.StatusCompleted:
		return m.styles.StatusCompleted, emptyStyle
	case db.StatusFailed:
		return m.styles.StatusFailed, emptyStyle
	default:
		return m.styles.StatusPending, emptyStyle
	}
}

func formatETA(seconds float64) string {
	if seconds <= 0 {
		return "0s"
	}
	if seconds < 60 {
		return fmt.Sprintf("%.0fs", seconds)
	}
	if seconds < 3600 {
		return fmt.Sprintf("%.0fm%02.0fs", seconds/60, float64(int(seconds)%60))
	}
	return fmt.Sprintf("%.0fh%02.0fm", seconds/3600, float64(int(seconds)%3600)/60)
}

func (m *downloadsModel) clampCursor() {
	if m.cursor >= len(m.items) {
		m.cursor = len(m.items) - 1
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
}

func (m *downloadsModel) setSize(w, h int) {
	m.width = w
	m.height = h
}
```

- [ ] **Step 2: Verify it compiles**

Run: `cd /home/williams/Documents/personal/bookdl && go build ./internal/tui/dashboard/`
Expected: No errors

- [ ] **Step 3: Commit**

```bash
git add internal/tui/dashboard/downloads.go
git commit -m "feat(tui): add downloads panel with split pane, live progress, speed/ETA"
```

---

### Task 9: Search Panel

**Files:**
- Create: `internal/tui/dashboard/search.go`

- [ ] **Step 1: Create search.go**

```go
package dashboard

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/billmal071/bookdl/internal/anna"
	"github.com/billmal071/bookdl/internal/config"
	"github.com/billmal071/bookdl/internal/db"
	"github.com/billmal071/bookdl/internal/search"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

type searchResultsMsg struct {
	books []*anna.Book
	err   error
}

type searchMoreMsg struct {
	books []*anna.Book
	err   error
}

// searchDownloadMsg tells root to download selected book(s).
type searchDownloadMsg struct {
	books []*anna.Book
}

// searchBookmarkMsg tells root to bookmark a book.
type searchBookmarkMsg struct {
	book *anna.Book
}

type sourceOption int

const (
	sourceAll sourceOption = iota
	sourceAnna
	sourceZLibrary
	sourceLiber3
)

var sourceNames = [4]string{"All", "Anna", "Z-Library", "Liber3"}
var sourceOptions = [4]search.Option{
	search.OptionAll, search.OptionAnna, search.OptionZLibrary, search.OptionLiber3,
}

func (s sourceOption) String() string { return sourceNames[s] }
func (s sourceOption) next() sourceOption { return (s + 1) % 4 }

type searchModel struct {
	input       textinput.Model
	spinner     spinner.Model
	results     []*anna.Book
	cursor      int
	styles      Styles
	width       int
	height      int
	err         error
	searching   bool
	inputFocused bool
	source      sourceOption
	query       string
	page        int
	noMore      bool
	checked     map[string]bool // multi-select by MD5
}

func newSearchModel(s Styles) searchModel {
	ti := textinput.New()
	ti.Placeholder = "Search for books..."
	ti.CharLimit = 200
	ti.Focus()

	sp := spinner.New()
	sp.Spinner = spinner.Points
	sp.Style = s.Spinner

	return searchModel{
		input:        ti,
		spinner:      sp,
		styles:       s,
		inputFocused: true,
		checked:      make(map[string]bool),
	}
}

func (m searchModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m searchModel) Update(msg tea.Msg) (searchModel, tea.Cmd) {
	switch msg := msg.(type) {
	case searchResultsMsg:
		m.searching = false
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		m.results = msg.books
		m.cursor = 0
		m.page = 1
		m.noMore = len(msg.books) == 0
		m.checked = make(map[string]bool)
		return m, nil

	case searchMoreMsg:
		m.searching = false
		if msg.err != nil || len(msg.books) == 0 {
			m.noMore = true
			return m, nil
		}
		m.results = append(m.results, msg.books...)
		return m, nil

	case spinner.TickMsg:
		if m.searching {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			return m, cmd
		}
		return m, nil

	case tea.KeyMsg:
		// Input mode
		if m.inputFocused {
			switch msg.String() {
			case "enter":
				q := strings.TrimSpace(m.input.Value())
				if q != "" {
					m.query = q
					m.searching = true
					m.err = nil
					m.inputFocused = false
					m.input.Blur()
					return m, tea.Batch(m.spinner.Tick, m.doSearch(q))
				}
				return m, nil
			case "esc":
				if len(m.results) > 0 {
					m.inputFocused = false
					m.input.Blur()
				}
				return m, nil
			default:
				var cmd tea.Cmd
				m.input, cmd = m.input.Update(msg)
				return m, cmd
			}
		}

		// Results mode
		switch msg.String() {
		case "/":
			m.inputFocused = true
			m.input.Focus()
			return m, textinput.Blink
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.results)-1 {
				m.cursor++
			}
		case "s":
			m.source = m.source.next()
		case " ":
			if m.cursor < len(m.results) {
				md5 := m.results[m.cursor].MD5Hash
				if m.checked[md5] {
					delete(m.checked, md5)
				} else {
					m.checked[md5] = true
				}
			}
		case "d":
			books := m.selectedBooks()
			if len(books) > 0 {
				return m, func() tea.Msg {
					return searchDownloadMsg{books: books}
				}
			}
		case "b":
			if m.cursor < len(m.results) {
				book := m.results[m.cursor]
				return m, func() tea.Msg {
					return searchBookmarkMsg{book: book}
				}
			}
		case "m":
			if !m.noMore && m.query != "" {
				m.searching = true
				m.page++
				return m, tea.Batch(m.spinner.Tick, m.doSearchPage(m.query, m.page))
			}
		}
	}
	return m, nil
}

func (m searchModel) doSearch(query string) tea.Cmd {
	src := m.source
	return func() tea.Msg {
		searcher := search.NewSearcher(sourceOptions[src])
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()

		books, err := searcher.Search(ctx, query, 25)
		if err != nil {
			return searchResultsMsg{err: err}
		}

		// Save to history
		db.AddSearchHistory(query, len(books), db.SearchFilters{})

		// Save to cache
		cfg := config.Get()
		if cfg.Cache.Enabled {
			filterMap := map[string]string{}
			cacheKey := db.GenerateCacheKey(query, filterMap)
			if resultsJSON, err := json.Marshal(books); err == nil {
				db.SaveCachedSearch(cacheKey, query, "{}", string(resultsJSON), len(books), cfg.Cache.TTL)
			}
		}

		return searchResultsMsg{books: books}
	}
}

func (m searchModel) doSearchPage(query string, page int) tea.Cmd {
	src := m.source
	return func() tea.Msg {
		searcher := search.NewSearcher(sourceOptions[src])
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		books, err := searcher.SearchPage(ctx, query, 25, page)
		return searchMoreMsg{books: books, err: err}
	}
}

func (m searchModel) selectedBooks() []*anna.Book {
	if len(m.checked) > 0 {
		var books []*anna.Book
		for _, book := range m.results {
			if m.checked[book.MD5Hash] {
				books = append(books, book)
			}
		}
		return books
	}
	// If nothing checked, use current item
	if m.cursor < len(m.results) {
		return []*anna.Book{m.results[m.cursor]}
	}
	return nil
}

func (m searchModel) View(l layout) string {
	var b strings.Builder
	b.Grow(2048)

	maxW := l.contentWidth - 6

	// Search input
	inputBorder := m.styles.SearchBorderBlurred
	if m.inputFocused {
		inputBorder = m.styles.SearchBorderFocused
	}
	inputView := inputBorder.Width(maxW).Render(m.input.View())
	b.WriteString(inputView)
	b.WriteString("\n")

	// Source selector
	b.WriteString("  Source: ")
	for i := sourceOption(0); i < 4; i++ {
		if i == m.source {
			b.WriteString(m.styles.ActiveTab.Render(i.String()))
		} else {
			b.WriteString(m.styles.InactiveTab.Render(i.String()))
		}
		if i < 3 {
			b.WriteString(m.styles.ItemMeta.Render("  ▸  "))
		}
	}
	b.WriteString("\n\n")

	// Loading
	if m.searching {
		b.WriteString(fmt.Sprintf("  %s Searching...", m.spinner.View()))
		return b.String()
	}

	// Error
	if m.err != nil {
		b.WriteString(m.styles.Error.Render(fmt.Sprintf("  Error: %v", m.err)))
		b.WriteString("\n")
		b.WriteString(m.styles.ItemMeta.Render("  Press / to search again"))
		return b.String()
	}

	// No results
	if m.query != "" && len(m.results) == 0 {
		b.WriteString(m.styles.ItemMeta.Render("  No results found."))
		return b.String()
	}

	if len(m.results) == 0 {
		b.WriteString(m.styles.ItemMeta.Render("  Type a search query and press Enter."))
		return b.String()
	}

	// Results
	checked := len(m.checked)
	header := fmt.Sprintf("  Results (%d)", len(m.results))
	if checked > 0 {
		header += m.styles.Success.Render(fmt.Sprintf("  [%d selected]", checked))
	}
	b.WriteString(m.styles.Label.Render(header))
	b.WriteString("\n\n")

	visible := (l.contentHeight - 7) / 2 // 2 lines per item, 7 lines of header/input/source
	if visible < 1 {
		visible = 1
	}

	start := 0
	if m.cursor >= visible {
		start = m.cursor - visible + 1
	}
	end := start + visible
	if end > len(m.results) {
		end = len(m.results)
	}

	for i := start; i < end; i++ {
		book := m.results[i]
		title := truncate(book.Title, maxW-6)

		checkbox := ""
		if len(m.checked) > 0 {
			if m.checked[book.MD5Hash] {
				checkbox = m.styles.Success.Render("[●] ")
			} else {
				checkbox = m.styles.ItemMeta.Render("[ ] ")
			}
		}

		if i == m.cursor {
			b.WriteString(m.styles.SelectedItem.Render(fmt.Sprintf("  ▸ %s%s", checkbox, title)))
		} else {
			b.WriteString(fmt.Sprintf("    %s%s", checkbox, m.styles.NormalItem.Render(title)))
		}
		b.WriteString("\n")

		// Metadata
		meta := joinMeta(book.Authors, book.Format, book.Size, book.Language)
		b.WriteString(m.styles.ItemMeta.Render(fmt.Sprintf("       %s", truncate(meta, maxW-7))))
		b.WriteString("\n")
	}

	return b.String()
}

func (m *searchModel) setSize(w, h int) {
	m.width = w
	m.height = h
	m.input.Width = w - 8
}

func (m searchModel) isFocusedOnInput() bool {
	return m.inputFocused
}

// prefillAndSearch sets a query and executes search (used by history re-run).
func (m *searchModel) prefillAndSearch(query string, filters db.SearchFilters) tea.Cmd {
	m.input.SetValue(query)
	m.query = query
	m.searching = true
	m.err = nil
	m.inputFocused = false
	m.input.Blur()
	return tea.Batch(m.spinner.Tick, m.doSearch(query))
}
```

- [ ] **Step 2: Verify it compiles**

Run: `cd /home/williams/Documents/personal/bookdl && go build ./internal/tui/dashboard/`
Expected: No errors

- [ ] **Step 3: Commit**

```bash
git add internal/tui/dashboard/search.go
git commit -m "feat(tui): add search panel with multi-source, pagination, multi-select"
```

---

### Task 10: Root Dashboard Model

**Files:**
- Create: `internal/tui/dashboard/model.go`

- [ ] **Step 1: Create model.go**

```go
package dashboard

import (
	"fmt"
	"os"
	"strings"

	"github.com/billmal071/bookdl/internal/anna"
	"github.com/billmal071/bookdl/internal/db"
	tea "github.com/charmbracelet/bubbletea"
	"golang.org/x/term"
)

// Model is the root dashboard model.
type Model struct {
	activeTab tab
	focus     focus
	layout    layout
	styles    Styles

	// Panel sub-models
	search    searchModel
	downloads downloadsModel
	queue     queueModel
	bookmarks bookmarksModel
	history   historyModel

	quitting bool
}

// New creates a new dashboard model.
func New() Model {
	s := NewStyles(DefaultTheme)
	return Model{
		activeTab: tabSearch,
		focus:     focusSearchInput,
		styles:    s,
		search:    newSearchModel(s),
		downloads: newDownloadsModel(s),
		queue:     newQueueModel(s),
		bookmarks: newBookmarksModel(s),
		history:   newHistoryModel(s),
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(
		m.search.Init(),
		m.downloads.Init(),
		m.queue.Init(),
		m.bookmarks.Init(),
		m.history.Init(),
	)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.layout = computeLayout(msg.Width, msg.Height)
		m.search.setSize(msg.Width, m.layout.contentHeight)
		m.downloads.setSize(msg.Width, m.layout.contentHeight)
		m.queue.setSize(msg.Width, m.layout.contentHeight)
		m.bookmarks.setSize(msg.Width, m.layout.contentHeight)
		m.history.setSize(msg.Width, m.layout.contentHeight)
		return m, nil

	case tea.KeyMsg:
		// Always handle ctrl+c
		if msg.String() == "ctrl+c" {
			m.quitting = true
			return m, tea.Quit
		}

		// Check if current panel is in text input mode
		if m.isInputFocused() {
			return m.updateActivePanel(msg)
		}

		// Global keys (not in text input)
		switch msg.String() {
		case "q":
			m.quitting = true
			return m, tea.Quit
		case "tab":
			m.activeTab = m.activeTab.next()
			m.focus = defaultFocusForTab(m.activeTab)
			return m, nil
		case "shift+tab":
			m.activeTab = m.activeTab.prev()
			m.focus = defaultFocusForTab(m.activeTab)
			return m, nil
		}

		// Delegate to active panel
		return m.updateActivePanel(msg)

	// Cross-panel messages
	case rerunSearchMsg:
		m.activeTab = tabSearch
		m.focus = focusSearchResults
		cmd := m.search.prefillAndSearch(msg.query, msg.filters)
		return m, cmd

	case startDownloadMsg:
		m.activeTab = tabDownloads
		m.focus = focusDownloadList
		return m, m.initiateDownload(msg.download)

	case startAllDownloadsMsg:
		m.activeTab = tabDownloads
		m.focus = focusDownloadList
		return m, nil // TODO: could start concurrent downloads

	case searchDownloadMsg:
		m.activeTab = tabDownloads
		m.focus = focusDownloadList
		return m, m.initiateBookDownloads(msg.books)

	case searchBookmarkMsg:
		return m, m.createBookmark(msg.book)

	case bookmarkDownloadMsg:
		m.activeTab = tabDownloads
		m.focus = focusDownloadList
		return m, nil

	case bookmarkDownloadAllMsg:
		m.activeTab = tabDownloads
		m.focus = focusDownloadList
		return m, nil
	}

	// Delegate all other messages to all panels (for ticks, loaded msgs, etc.)
	return m.updateAllPanels(msg)
}

func (m Model) updateActivePanel(msg tea.Msg) (Model, tea.Cmd) {
	var cmd tea.Cmd
	switch m.activeTab {
	case tabSearch:
		m.search, cmd = m.search.Update(msg)
	case tabDownloads:
		m.downloads, cmd = m.downloads.Update(msg)
	case tabQueue:
		m.queue, cmd = m.queue.Update(msg)
	case tabBookmarks:
		m.bookmarks, cmd = m.bookmarks.Update(msg)
	case tabHistory:
		m.history, cmd = m.history.Update(msg)
	}
	return m, cmd
}

func (m Model) updateAllPanels(msg tea.Msg) (Model, tea.Cmd) {
	var cmds []tea.Cmd
	var cmd tea.Cmd

	m.search, cmd = m.search.Update(msg)
	cmds = append(cmds, cmd)
	m.downloads, cmd = m.downloads.Update(msg)
	cmds = append(cmds, cmd)
	m.queue, cmd = m.queue.Update(msg)
	cmds = append(cmds, cmd)
	m.bookmarks, cmd = m.bookmarks.Update(msg)
	cmds = append(cmds, cmd)
	m.history, cmd = m.history.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

func (m Model) View() string {
	if m.quitting {
		return ""
	}

	var b strings.Builder
	b.Grow(4096)

	// Tab bar
	b.WriteString(renderTabBar(m.activeTab, m.styles, m.layout.width))
	b.WriteString("\n")

	// Active panel content
	switch m.activeTab {
	case tabSearch:
		b.WriteString(m.search.View(m.layout))
	case tabDownloads:
		b.WriteString(m.downloads.View(m.layout))
	case tabQueue:
		b.WriteString(m.queue.View(m.layout))
	case tabBookmarks:
		b.WriteString(m.bookmarks.View(m.layout))
	case tabHistory:
		b.WriteString(m.history.View(m.layout))
	}

	// Status bar
	b.WriteString("\n")
	b.WriteString(m.renderStatusBar())

	return b.String()
}

func (m Model) renderStatusBar() string {
	separator := m.styles.TabSeparator.Render(strings.Repeat("─", m.layout.width))

	var hints []string

	if m.isInputFocused() {
		hints = append(hints,
			m.styles.HelpKey.Render("esc")+" "+m.styles.HelpDesc.Render("cancel"),
			m.styles.HelpKey.Render("enter")+" "+m.styles.HelpDesc.Render("submit"),
		)
	} else {
		hints = append(hints,
			m.styles.HelpKey.Render("⇥")+" "+m.styles.HelpDesc.Render("switch tab"),
			m.styles.HelpKey.Render("↑↓")+" "+m.styles.HelpDesc.Render("navigate"),
		)

		switch m.activeTab {
		case tabSearch:
			hints = append(hints,
				m.styles.HelpKey.Render("d")+" "+m.styles.HelpDesc.Render("download"),
				m.styles.HelpKey.Render("b")+" "+m.styles.HelpDesc.Render("bookmark"),
				m.styles.HelpKey.Render("/")+" "+m.styles.HelpDesc.Render("search"),
			)
		case tabDownloads:
			hints = append(hints,
				m.styles.HelpKey.Render("p")+" "+m.styles.HelpDesc.Render("pause"),
				m.styles.HelpKey.Render("r")+" "+m.styles.HelpDesc.Render("resume"),
				m.styles.HelpKey.Render("x")+" "+m.styles.HelpDesc.Render("cancel"),
			)
		case tabQueue:
			hints = append(hints,
				m.styles.HelpKey.Render("d")+" "+m.styles.HelpDesc.Render("download"),
				m.styles.HelpKey.Render("t")+" "+m.styles.HelpDesc.Render("top"),
				m.styles.HelpKey.Render("x")+" "+m.styles.HelpDesc.Render("remove"),
			)
		case tabBookmarks:
			hints = append(hints,
				m.styles.HelpKey.Render("d")+" "+m.styles.HelpDesc.Render("download"),
				m.styles.HelpKey.Render("n")+" "+m.styles.HelpDesc.Render("note"),
				m.styles.HelpKey.Render("x")+" "+m.styles.HelpDesc.Render("remove"),
			)
		case tabHistory:
			hints = append(hints,
				m.styles.HelpKey.Render("enter")+" "+m.styles.HelpDesc.Render("re-run"),
				m.styles.HelpKey.Render("x")+" "+m.styles.HelpDesc.Render("delete"),
			)
		}

		hints = append(hints,
			m.styles.HelpKey.Render("q")+" "+m.styles.HelpDesc.Render("quit"),
		)
	}

	return separator + "\n  " + strings.Join(hints, "   ")
}

func (m Model) isInputFocused() bool {
	switch m.activeTab {
	case tabSearch:
		return m.search.isFocusedOnInput()
	case tabBookmarks:
		return m.bookmarks.isFocusedOnInput()
	}
	return false
}

func (m Model) initiateDownload(d *db.Download) tea.Cmd {
	return func() tea.Msg {
		db.UpdateStatus(d.ID, db.StatusDownloading, "")
		return downloadsLoadedMsg{} // trigger refresh
	}
}

func (m Model) initiateBookDownloads(books []*anna.Book) tea.Cmd {
	return func() tea.Msg {
		for _, book := range books {
			d := &db.Download{
				MD5Hash:   book.MD5Hash,
				Title:     book.Title,
				Authors:   book.Authors,
				Format:    book.Format,
				FileSize:  book.SizeBytes,
				SourceURL: book.PageURL,
				Source:    book.Source,
				Status:    db.StatusPending,
			}
			db.CreateDownload(d)
		}
		return downloadsLoadedMsg{} // trigger refresh
	}
}

func (m Model) createBookmark(book *anna.Book) tea.Cmd {
	return func() tea.Msg {
		if db.BookmarkExists(book.MD5Hash) {
			return bookmarksLoadedMsg{} // already bookmarked
		}
		bm := &db.Bookmark{
			MD5Hash:  book.MD5Hash,
			Title:    book.Title,
			Authors:  book.Authors,
			Year:     book.Year,
			Language: book.Language,
			Format:   book.Format,
			Size:     book.Size,
			PageURL:  book.PageURL,
		}
		db.CreateBookmark(bm)
		return bookmarksLoadedMsg{} // trigger refresh
	}
}

// Run starts the TUI dashboard.
func Run() error {
	// Check terminal
	if !term.IsTerminal(int(os.Stdin.Fd())) || os.Getenv("NO_TUI") != "" {
		return fmt.Errorf("TUI requires an interactive terminal. Use subcommands instead, or unset NO_TUI")
	}

	p := tea.NewProgram(New(), tea.WithAltScreen())
	_, err := p.Run()
	return err
}
```

- [ ] **Step 2: Add `golang.org/x/term` dependency (if not already present)**

Run: `cd /home/williams/Documents/personal/bookdl && grep "golang.org/x/term" go.mod`

If not found: `go get golang.org/x/term`

- [ ] **Step 3: Verify it compiles**

Run: `cd /home/williams/Documents/personal/bookdl && go build ./internal/tui/dashboard/`
Expected: No errors

- [ ] **Step 4: Commit**

```bash
git add internal/tui/dashboard/model.go
git commit -m "feat(tui): add root dashboard model with tab routing and cross-panel navigation"
```

---

### Task 11: Wire TUI into CLI

**Files:**
- Modify: `internal/cli/root.go`

- [ ] **Step 1: Add TUI launch to root command and `tui` subcommand**

In `internal/cli/root.go`, modify the `rootCmd` to add a `RunE` that launches the TUI when no subcommand is given, and add a `tui` subcommand:

```go
// Add this import
import "github.com/billmal071/bookdl/internal/tui/dashboard"

// Add RunE to rootCmd (add after the Long field, before PersistentPreRunE)
var rootCmd = &cobra.Command{
	Use:   "bookdl",
	Short: "Download books from Anna's Archive",
	Long: `bookdl is a CLI tool for searching and downloading books from Anna's Archive.
...existing long description...`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return dashboard.Run()
	},
	// ...existing PersistentPreRunE and PersistentPostRun...
}

// Add tui subcommand in init()
var tuiCmd = &cobra.Command{
	Use:   "tui",
	Short: "Launch interactive TUI dashboard",
	RunE: func(cmd *cobra.Command, args []string) error {
		return dashboard.Run()
	},
}
```

Add `rootCmd.AddCommand(tuiCmd)` in the `init()` function.

- [ ] **Step 2: Verify the full binary builds**

Run: `cd /home/williams/Documents/personal/bookdl && go build -o /dev/null ./cmd/bookdl/`
Expected: No errors

- [ ] **Step 3: Run existing tests to verify no regressions**

Run: `cd /home/williams/Documents/personal/bookdl && go test ./... 2>&1 | tail -20`
Expected: All tests pass

- [ ] **Step 4: Manual smoke test**

Run: `cd /home/williams/Documents/personal/bookdl && go run ./cmd/bookdl/`
Expected: Full-screen TUI appears with tab bar, search input focused, can switch tabs with Tab key, quit with q.

- [ ] **Step 5: Test existing subcommands still work**

Run: `cd /home/williams/Documents/personal/bookdl && go run ./cmd/bookdl/ version`
Expected: Prints version info (not TUI)

- [ ] **Step 6: Commit**

```bash
git add internal/cli/root.go
git commit -m "feat(tui): wire dashboard into CLI, launch on no-args and bookdl tui"
```

---

### Task 12: Build, Install, and Final Verification

**Files:** None (build/test only)

- [ ] **Step 1: Run go mod tidy**

Run: `cd /home/williams/Documents/personal/bookdl && go mod tidy`

- [ ] **Step 2: Run linter if available**

Run: `cd /home/williams/Documents/personal/bookdl && make lint 2>&1 || echo "lint not available"`

- [ ] **Step 3: Run full test suite**

Run: `cd /home/williams/Documents/personal/bookdl && make test`

- [ ] **Step 4: Build**

Run: `cd /home/williams/Documents/personal/bookdl && make build`
Expected: Binary at `./build/bookdl`

- [ ] **Step 5: Install**

Run: `cd /home/williams/Documents/personal/bookdl && make install`

- [ ] **Step 6: Verify installed binary launches TUI**

Run: `bookdl`
Expected: TUI dashboard launches

- [ ] **Step 7: Verify subcommands**

Run: `bookdl version && bookdl search --help`
Expected: Both work normally

- [ ] **Step 8: Commit any final changes (go.sum, etc.)**

```bash
git add -A
git commit -m "chore: tidy deps and finalize TUI dashboard integration"
```
