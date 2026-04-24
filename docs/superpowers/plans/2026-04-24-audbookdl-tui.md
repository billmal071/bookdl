# audbookdl TUI Implementation Plan (Plan 5 of 8)

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the full-screen terminal UI using the Charm stack (bubbletea, bubbles, lipgloss) with 4 tabs: Search, Downloads, Library, Player. Running `audbookdl` with no args launches this TUI.

**Architecture:** The TUI is a bubbletea program with a root `App` model that manages tab navigation. Each tab is a separate bubbletea model implementing a common `Tab` interface. The App delegates messages to the active tab. Styles are centralized in `styles.go`.

**Tech Stack:** bubbletea, bubbles (textinput, list, spinner, progress, help, viewport), lipgloss

---

### Task 1: Styles and Theme

**Files:**
- Create: `internal/tui/styles.go`

- [ ] **Step 1: Create `internal/tui/styles.go`**

```go
package tui

import "github.com/charmbracelet/lipgloss"

var (
	// Colors
	primaryColor   = lipgloss.Color("#7C3AED") // Purple
	secondaryColor = lipgloss.Color("#06B6D4") // Cyan
	successColor   = lipgloss.Color("#10B981") // Green
	warningColor   = lipgloss.Color("#F59E0B") // Amber
	errorColor     = lipgloss.Color("#EF4444") // Red
	mutedColor     = lipgloss.Color("#6B7280") // Gray
	textColor      = lipgloss.Color("#F9FAFB") // Light
	bgColor        = lipgloss.Color("#111827") // Dark

	// Tab bar
	activeTabStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(primaryColor).
			Padding(0, 2)

	inactiveTabStyle = lipgloss.NewStyle().
				Foreground(mutedColor).
				Padding(0, 2)

	tabBarStyle = lipgloss.NewStyle().
			BorderBottom(true).
			BorderStyle(lipgloss.NormalBorder()).
			BorderForeground(mutedColor)

	// Content
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(textColor)

	subtitleStyle = lipgloss.NewStyle().
			Foreground(mutedColor)

	sourceStyle = lipgloss.NewStyle().
			Foreground(secondaryColor)

	statusBarStyle = lipgloss.NewStyle().
			Foreground(mutedColor).
			BorderTop(true).
			BorderStyle(lipgloss.NormalBorder()).
			BorderForeground(mutedColor)

	// Status indicators
	completedStyle = lipgloss.NewStyle().Foreground(successColor)
	downloadingStyle = lipgloss.NewStyle().Foreground(secondaryColor)
	failedStyle = lipgloss.NewStyle().Foreground(errorColor)
	pausedStyle = lipgloss.NewStyle().Foreground(warningColor)
	pendingStyle = lipgloss.NewStyle().Foreground(mutedColor)

	// Help
	helpStyle = lipgloss.NewStyle().Foreground(mutedColor)
)
```

- [ ] **Step 2: Fetch charm dependencies**

```bash
cd ~/Documents/personal/audbookdl
/usr/local/go/bin/go get github.com/charmbracelet/bubbletea
/usr/local/go/bin/go get github.com/charmbracelet/bubbles
/usr/local/go/bin/go get github.com/charmbracelet/lipgloss
CGO_ENABLED=0 /usr/local/go/bin/go mod tidy
```

- [ ] **Step 3: Commit**

```bash
git add internal/tui/ go.mod go.sum
git commit -m "feat: add TUI styles and Charm stack dependencies"
```

---

### Task 2: App Shell with Tab Navigation

**Files:**
- Create: `internal/tui/app.go`

- [ ] **Step 1: Create `internal/tui/app.go`**

```go
package tui

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Tab is the interface each tab model implements.
type Tab interface {
	tea.Model
	TabName() string
	ShortHelp() []key.Binding
}

// keyMap defines global keybindings.
type keyMap struct {
	Tab      key.Binding
	ShiftTab key.Binding
	Quit     key.Binding
	Help     key.Binding
}

var keys = keyMap{
	Tab:      key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "next tab")),
	ShiftTab: key.NewBinding(key.WithKeys("shift+tab"), key.WithHelp("shift+tab", "prev tab")),
	Quit:     key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")),
	Help:     key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "help")),
}

// App is the root TUI model.
type App struct {
	tabs      []Tab
	activeTab int
	width     int
	height    int
	help      help.Model
	showHelp  bool
	db        *sql.DB
}

// NewApp creates the TUI application.
func NewApp(database *sql.DB, baseDir string) *App {
	h := help.New()
	h.ShowAll = false

	return &App{
		tabs: []Tab{
			NewSearchTab(database),
			NewDownloadsTab(database),
			NewLibraryTab(database, baseDir),
			NewPlayerTab(),
		},
		help: h,
		db:   database,
	}
}

func (a *App) Init() tea.Cmd {
	var cmds []tea.Cmd
	for _, t := range a.tabs {
		cmds = append(cmds, t.Init())
	}
	return tea.Batch(cmds...)
}

func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		a.width = msg.Width
		a.height = msg.Height
		// Forward to all tabs
		var cmds []tea.Cmd
		for i, t := range a.tabs {
			// Subtract space for tab bar (2 lines) and status bar (2 lines)
			innerMsg := tea.WindowSizeMsg{Width: msg.Width, Height: msg.Height - 4}
			newTab, cmd := t.Update(innerMsg)
			a.tabs[i] = newTab.(Tab)
			cmds = append(cmds, cmd)
		}
		return a, tea.Batch(cmds...)

	case tea.KeyMsg:
		switch {
		case key.Matches(msg, keys.Quit):
			return a, tea.Quit
		case key.Matches(msg, keys.Tab):
			a.activeTab = (a.activeTab + 1) % len(a.tabs)
			return a, nil
		case key.Matches(msg, keys.ShiftTab):
			a.activeTab = (a.activeTab - 1 + len(a.tabs)) % len(a.tabs)
			return a, nil
		case key.Matches(msg, keys.Help):
			a.showHelp = !a.showHelp
			return a, nil
		}
	}

	// Forward to active tab
	newTab, cmd := a.tabs[a.activeTab].Update(msg)
	a.tabs[a.activeTab] = newTab.(Tab)
	return a, cmd
}

func (a *App) View() string {
	if a.width == 0 {
		return "Loading..."
	}

	var b strings.Builder

	// Tab bar
	var tabItems []string
	for i, t := range a.tabs {
		if i == a.activeTab {
			tabItems = append(tabItems, activeTabStyle.Render(t.TabName()))
		} else {
			tabItems = append(tabItems, inactiveTabStyle.Render(t.TabName()))
		}
	}
	tabBar := tabBarStyle.Width(a.width).Render(lipgloss.JoinHorizontal(lipgloss.Top, tabItems...))
	b.WriteString(tabBar)
	b.WriteString("\n")

	// Active tab content
	content := a.tabs[a.activeTab].View()
	b.WriteString(content)
	b.WriteString("\n")

	// Status bar
	helpText := "tab/shift+tab: switch tabs · q: quit · ?: help"
	if a.showHelp {
		bindings := a.tabs[a.activeTab].ShortHelp()
		var parts []string
		for _, binding := range bindings {
			parts = append(parts, fmt.Sprintf("%s: %s", binding.Help().Key, binding.Help().Desc))
		}
		if len(parts) > 0 {
			helpText = strings.Join(parts, " · ")
		}
	}
	statusBar := statusBarStyle.Width(a.width).Render(helpStyle.Render(helpText))
	b.WriteString(statusBar)

	return b.String()
}

// Run starts the TUI.
func Run(database *sql.DB, baseDir string) error {
	app := NewApp(database, baseDir)
	p := tea.NewProgram(app, tea.WithAltScreen())
	_, err := p.Run()
	return err
}
```

- [ ] **Step 2: Commit**

```bash
git add internal/tui/
git commit -m "feat: add TUI app shell with tab navigation"
```

---

### Task 3: Search Tab

**Files:**
- Create: `internal/tui/search.go`

- [ ] **Step 1: Create `internal/tui/search.go`**

The search tab has a text input for queries, a spinner for loading, and a list of results.

```go
package tui

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/billmal071/audbookdl/internal/db"
	"github.com/billmal071/audbookdl/internal/httpclient"
	"github.com/billmal071/audbookdl/internal/source"
)

// searchResultMsg carries search results back to the UI.
type searchResultMsg struct {
	books []*source.Audiobook
	err   error
}

// SearchTab is the search tab model.
type SearchTab struct {
	db       *sql.DB
	input    textinput.Model
	spinner  spinner.Model
	results  []*source.Audiobook
	cursor   int
	loading  bool
	err      error
	width    int
	height   int
}

func NewSearchTab(database *sql.DB) *SearchTab {
	ti := textinput.New()
	ti.Placeholder = "Search audiobooks..."
	ti.Focus()
	ti.CharLimit = 100
	ti.Width = 40

	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(primaryColor)

	return &SearchTab{
		db:      database,
		input:   ti,
		spinner: sp,
	}
}

func (s *SearchTab) TabName() string { return "Search" }

func (s *SearchTab) ShortHelp() []key.Binding {
	return []key.Binding{
		key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "search")),
		key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("up/k", "up")),
		key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("down/j", "down")),
	}
}

func (s *SearchTab) Init() tea.Cmd {
	return textinput.Blink
}

func (s *SearchTab) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		s.width = msg.Width
		s.height = msg.Height
		return s, nil

	case searchResultMsg:
		s.loading = false
		s.results = msg.books
		s.err = msg.err
		s.cursor = 0
		if msg.err == nil && s.db != nil {
			db.AddSearchHistory(s.db, s.input.Value(), "", len(msg.books))
		}
		return s, nil

	case spinner.TickMsg:
		if s.loading {
			var cmd tea.Cmd
			s.spinner, cmd = s.spinner.Update(msg)
			return s, cmd
		}
		return s, nil

	case tea.KeyMsg:
		if s.loading {
			return s, nil
		}

		switch msg.String() {
		case "enter":
			if s.input.Value() != "" {
				s.loading = true
				s.err = nil
				query := s.input.Value()
				return s, tea.Batch(s.spinner.Tick, doSearch(query))
			}
		case "up", "k":
			if s.cursor > 0 {
				s.cursor--
			}
		case "down", "j":
			if s.cursor < len(s.results)-1 {
				s.cursor++
			}
		default:
			var cmd tea.Cmd
			s.input, cmd = s.input.Update(msg)
			return s, cmd
		}
	}

	return s, nil
}

func (s *SearchTab) View() string {
	var view string

	// Search input
	view += s.input.View() + "\n\n"

	if s.loading {
		view += s.spinner.View() + " Searching...\n"
		return view
	}

	if s.err != nil {
		view += failedStyle.Render(fmt.Sprintf("Error: %v", s.err)) + "\n"
		return view
	}

	if len(s.results) == 0 {
		view += subtitleStyle.Render("Type a query and press Enter to search.") + "\n"
		return view
	}

	// Results list
	for i, b := range s.results {
		cursor := "  "
		style := lipgloss.NewStyle()
		if i == s.cursor {
			cursor = "> "
			style = style.Bold(true)
		}

		title := style.Render(b.Title)
		meta := subtitleStyle.Render(fmt.Sprintf("%s", b.Author))
		if b.Narrator != "" {
			meta += subtitleStyle.Render(fmt.Sprintf(" · %s", b.Narrator))
		}

		details := ""
		if b.Duration > 0 {
			details += b.DurationFormatted() + " · "
		}
		if b.ChapterCount > 0 {
			details += fmt.Sprintf("%d ch · ", b.ChapterCount)
		}
		details += sourceStyle.Render(b.Source)

		view += fmt.Sprintf("%s%s\n   %s\n   %s\n", cursor, title, meta, details)
	}

	view += fmt.Sprintf("\n%s", subtitleStyle.Render(fmt.Sprintf("%d results", len(s.results))))

	return view
}

func doSearch(query string) tea.Cmd {
	return func() tea.Msg {
		http := httpclient.New()
		searcher := buildDefaultSearcher(http)

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		books, err := searcher.Search(ctx, query, source.SearchOptions{Limit: 10})
		return searchResultMsg{books: books, err: err}
	}
}
```

NOTE: `buildDefaultSearcher` needs to be accessible. Add this helper to search.go or make it a shared function. Since search.go in cli package has `buildSearcher`, we need a version in the tui package. Add this to the bottom of search.go:

```go
func buildDefaultSearcher(http *httpclient.Client) *search.Searcher {
	return buildSearcher(http, "")
}
```

Actually, since the tui package can't import the cli package (that would be circular), we need to duplicate the searcher builder in the tui package or extract it. The simplest approach: put the builder function directly in the tui search tab file.

Add to the bottom of `internal/tui/search.go`:

```go
import (
	"github.com/billmal071/audbookdl/internal/archive"
	"github.com/billmal071/audbookdl/internal/config"
	"github.com/billmal071/audbookdl/internal/librivox"
	"github.com/billmal071/audbookdl/internal/loyalbooks"
	"github.com/billmal071/audbookdl/internal/openlibrary"
	"github.com/billmal071/audbookdl/internal/search"
)

func buildDefaultSearcher(http *httpclient.Client) *search.Searcher {
	cfg := config.Get()
	var sources []source.Source
	for _, s := range cfg.Search.Sources {
		switch s {
		case "librivox":
			sources = append(sources, librivox.NewClient("", http))
		case "archive":
			sources = append(sources, archive.NewClient("", http))
		case "loyalbooks":
			sources = append(sources, loyalbooks.NewClient("", http))
		case "openlibrary":
			sources = append(sources, openlibrary.NewClient("", "", http))
		}
	}
	return search.New(sources...)
}
```

(Merge these imports into the file's import block.)

- [ ] **Step 2: Commit**

```bash
git add internal/tui/
git commit -m "feat: add search tab with text input, spinner, and result list"
```

---

### Task 4: Downloads Tab

**Files:**
- Create: `internal/tui/downloads.go`

- [ ] **Step 1: Create `internal/tui/downloads.go`**

Shows download status grouped by state (active, queued, completed).

```go
package tui

import (
	"database/sql"
	"fmt"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/billmal071/audbookdl/internal/db"
)

type refreshDownloadsMsg struct {
	downloads []*db.AudiobookDownload
	err       error
}

type DownloadsTab struct {
	db        *sql.DB
	downloads []*db.AudiobookDownload
	cursor    int
	err       error
	width     int
	height    int
}

func NewDownloadsTab(database *sql.DB) *DownloadsTab {
	return &DownloadsTab{db: database}
}

func (d *DownloadsTab) TabName() string { return "Downloads" }

func (d *DownloadsTab) ShortHelp() []key.Binding {
	return []key.Binding{
		key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "refresh")),
		key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("up/k", "up")),
		key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("down/j", "down")),
	}
}

func (d *DownloadsTab) Init() tea.Cmd {
	return d.refresh()
}

func (d *DownloadsTab) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		d.width = msg.Width
		d.height = msg.Height

	case refreshDownloadsMsg:
		d.downloads = msg.downloads
		d.err = msg.err

	case tea.KeyMsg:
		switch msg.String() {
		case "r":
			return d, d.refresh()
		case "up", "k":
			if d.cursor > 0 { d.cursor-- }
		case "down", "j":
			if d.cursor < len(d.downloads)-1 { d.cursor++ }
		}
	}
	return d, nil
}

func (d *DownloadsTab) View() string {
	if d.err != nil {
		return failedStyle.Render(fmt.Sprintf("Error: %v", d.err)) + "\n"
	}

	if len(d.downloads) == 0 {
		return subtitleStyle.Render("No downloads yet. Search for audiobooks to get started.") + "\n"
	}

	var view string
	for i, dl := range d.downloads {
		cursor := "  "
		if i == d.cursor { cursor = "> " }

		progress := float64(0)
		if dl.TotalSize > 0 {
			progress = float64(dl.DownloadedSize) / float64(dl.TotalSize) * 100
		}

		statusStr := renderStatus(dl.Status)
		view += fmt.Sprintf("%s%s %s — %s  %.0f%%\n",
			cursor, statusStr, dl.Title, dl.Author, progress)
	}

	view += "\n" + subtitleStyle.Render("r: refresh")
	return view
}

func (d *DownloadsTab) refresh() tea.Cmd {
	return func() tea.Msg {
		downloads, err := db.ListDownloads(d.db)
		return refreshDownloadsMsg{downloads: downloads, err: err}
	}
}

func renderStatus(status db.DownloadStatus) string {
	switch status {
	case db.StatusCompleted:
		return completedStyle.Render("[done]")
	case db.StatusDownloading:
		return downloadingStyle.Render("[....]")
	case db.StatusPaused:
		return pausedStyle.Render("[stop]")
	case db.StatusFailed:
		return failedStyle.Render("[fail]")
	default:
		return pendingStyle.Render("[wait]")
	}
}
```

- [ ] **Step 2: Commit**

```bash
git add internal/tui/
git commit -m "feat: add downloads tab with status display"
```

---

### Task 5: Library Tab

**Files:**
- Create: `internal/tui/library.go`

- [ ] **Step 1: Create `internal/tui/library.go`**

Shows downloaded audiobooks grouped by author, plus bookmarks.

```go
package tui

import (
	"database/sql"
	"fmt"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/billmal071/audbookdl/internal/db"
)

type refreshLibraryMsg struct {
	downloads []*db.AudiobookDownload
	bookmarks []*db.Bookmark
	err       error
}

type LibraryTab struct {
	db        *sql.DB
	baseDir   string
	downloads []*db.AudiobookDownload
	bookmarks []*db.Bookmark
	cursor    int
	err       error
	width     int
	height    int
}

func NewLibraryTab(database *sql.DB, baseDir string) *LibraryTab {
	return &LibraryTab{db: database, baseDir: baseDir}
}

func (l *LibraryTab) TabName() string { return "Library" }

func (l *LibraryTab) ShortHelp() []key.Binding {
	return []key.Binding{
		key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "refresh")),
		key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("up/k", "up")),
		key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("down/j", "down")),
	}
}

func (l *LibraryTab) Init() tea.Cmd {
	return l.refresh()
}

func (l *LibraryTab) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		l.width = msg.Width
		l.height = msg.Height

	case refreshLibraryMsg:
		l.downloads = msg.downloads
		l.bookmarks = msg.bookmarks
		l.err = msg.err

	case tea.KeyMsg:
		switch msg.String() {
		case "r":
			return l, l.refresh()
		case "up", "k":
			if l.cursor > 0 { l.cursor-- }
		case "down", "j":
			totalItems := len(l.downloads) + len(l.bookmarks)
			if l.cursor < totalItems-1 { l.cursor++ }
		}
	}
	return l, nil
}

func (l *LibraryTab) View() string {
	if l.err != nil {
		return failedStyle.Render(fmt.Sprintf("Error: %v", l.err)) + "\n"
	}

	var view string

	// Completed downloads grouped by author
	if len(l.downloads) > 0 {
		view += titleStyle.Render("Downloaded") + "\n\n"
		byAuthor := make(map[string][]*db.AudiobookDownload)
		var authorOrder []string
		for _, dl := range l.downloads {
			if dl.Status != db.StatusCompleted { continue }
			if _, ok := byAuthor[dl.Author]; !ok {
				authorOrder = append(authorOrder, dl.Author)
			}
			byAuthor[dl.Author] = append(byAuthor[dl.Author], dl)
		}

		idx := 0
		for _, author := range authorOrder {
			view += subtitleStyle.Render(author) + "\n"
			for _, dl := range byAuthor[author] {
				cursor := "  "
				if idx == l.cursor { cursor = "> " }
				view += fmt.Sprintf("%s  %s\n", cursor, dl.Title)
				idx++
			}
		}
	}

	// Bookmarks
	if len(l.bookmarks) > 0 {
		view += "\n" + titleStyle.Render("Bookmarks") + "\n\n"
		bmStart := len(l.downloads)
		for i, bm := range l.bookmarks {
			cursor := "  "
			if bmStart+i == l.cursor { cursor = "> " }
			view += fmt.Sprintf("%s%s — %s (%s)\n", cursor, bm.Title, bm.Author, sourceStyle.Render(bm.Source))
		}
	}

	if len(l.downloads) == 0 && len(l.bookmarks) == 0 {
		view = subtitleStyle.Render("Your library is empty. Download some audiobooks!") + "\n"
	}

	return view
}

func (l *LibraryTab) refresh() tea.Cmd {
	return func() tea.Msg {
		downloads, err := db.ListDownloads(l.db)
		if err != nil {
			return refreshLibraryMsg{err: err}
		}
		bookmarks, err := db.ListBookmarks(l.db)
		return refreshLibraryMsg{downloads: downloads, bookmarks: bookmarks, err: err}
	}
}
```

- [ ] **Step 2: Commit**

```bash
git add internal/tui/
git commit -m "feat: add library tab with author-grouped downloads and bookmarks"
```

---

### Task 6: Player Tab (Stub)

**Files:**
- Create: `internal/tui/playerui.go`

- [ ] **Step 1: Create `internal/tui/playerui.go`**

Placeholder that will be wired to the audio engine in Plan 6.

```go
package tui

import (
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
)

// PlayerTab is the audio player tab (stub — wired to engine in Plan 6).
type PlayerTab struct {
	width  int
	height int
}

func NewPlayerTab() *PlayerTab {
	return &PlayerTab{}
}

func (p *PlayerTab) TabName() string { return "Player" }

func (p *PlayerTab) ShortHelp() []key.Binding {
	return []key.Binding{
		key.NewBinding(key.WithKeys("space"), key.WithHelp("space", "play/pause")),
		key.NewBinding(key.WithKeys("n"), key.WithHelp("n", "next chapter")),
		key.NewBinding(key.WithKeys("p"), key.WithHelp("p", "prev chapter")),
	}
}

func (p *PlayerTab) Init() tea.Cmd { return nil }

func (p *PlayerTab) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		p.width = msg.Width
		p.height = msg.Height
	}
	return p, nil
}

func (p *PlayerTab) View() string {
	return "\n" + titleStyle.Render("Player") + "\n\n" +
		subtitleStyle.Render("No audiobook loaded.") + "\n\n" +
		subtitleStyle.Render("Select an audiobook from the Library tab to start playing.") + "\n"
}
```

- [ ] **Step 2: Commit**

```bash
git add internal/tui/
git commit -m "feat: add player tab stub (wired to audio engine in Plan 6)"
```

---

### Task 7: Wire TUI to Root Command

**Files:**
- Modify: `internal/cli/root.go`

- [ ] **Step 1: Update root.go to launch TUI when no args**

Add a `RunE` to `rootCmd` that launches the TUI when no subcommand is given:

```go
// In rootCmd definition, add RunE:
RunE: func(cmd *cobra.Command, args []string) error {
    cfg := config.Get()
    return tui.Run(db.DB(), cfg.Download.Directory)
},
```

Add import: `"github.com/billmal071/audbookdl/internal/tui"`

- [ ] **Step 2: Build and verify**

```bash
CGO_ENABLED=0 /usr/local/go/bin/go build -o ./build/audbookdl ./cmd/audbookdl
./build/audbookdl --help
```

- [ ] **Step 3: Run all tests**

```bash
CGO_ENABLED=0 /usr/local/go/bin/go test ./...
CGO_ENABLED=0 /usr/local/go/bin/go vet ./...
```

- [ ] **Step 4: Commit**

```bash
git add internal/cli/root.go internal/tui/
git commit -m "feat: wire TUI to root command — launch with no args"
```
