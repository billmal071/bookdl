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
	input        textinput.Model
	spinner      spinner.Model
	results      []*anna.Book
	cursor       int
	styles       Styles
	width        int
	height       int
	err          error
	searching    bool
	inputFocused bool
	source       sourceOption
	query        string
	page         int
	noMore       bool
	checked      map[string]bool // multi-select by MD5
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
func (m *searchModel) prefillAndSearch(query string, _ db.SearchFilters) tea.Cmd {
	m.input.SetValue(query)
	m.query = query
	m.searching = true
	m.err = nil
	m.inputFocused = false
	m.input.Blur()
	return tea.Batch(m.spinner.Tick, m.doSearch(query))
}
