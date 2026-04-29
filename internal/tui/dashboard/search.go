package dashboard

import (
	"github.com/billmal071/bookdl/internal/anna"
	"github.com/billmal071/bookdl/internal/db"
	tea "github.com/charmbracelet/bubbletea"
)

// searchDownloadMsg tells root to download selected book(s).
type searchDownloadMsg struct {
	books []*anna.Book
}

// searchBookmarkMsg tells root to bookmark a book.
type searchBookmarkMsg struct {
	book *anna.Book
}

type searchModel struct {
	styles       Styles
	width        int
	height       int
	inputFocused bool
}

func newSearchModel(s Styles) searchModel {
	return searchModel{
		styles:       s,
		inputFocused: true,
	}
}

func (m searchModel) Init() tea.Cmd {
	return nil
}

func (m searchModel) Update(msg tea.Msg) (searchModel, tea.Cmd) {
	return m, nil
}

func (m searchModel) View(l layout) string {
	return m.styles.ItemMeta.Render("  Search panel (not yet implemented)")
}

func (m *searchModel) setSize(w, h int) {
	m.width = w
	m.height = h
}

func (m searchModel) isFocusedOnInput() bool {
	return m.inputFocused
}

// prefillAndSearch sets a query and executes search (used by history re-run).
func (m *searchModel) prefillAndSearch(query string, filters db.SearchFilters) tea.Cmd {
	_ = query
	_ = filters
	return nil
}
