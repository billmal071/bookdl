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
