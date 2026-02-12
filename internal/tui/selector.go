package tui

import (
	"fmt"
	"io"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/williams/bookdl/internal/anna"
)

// LoadMoreFunc is a callback to load more search results
type LoadMoreFunc func() ([]*anna.Book, error)

// loadMoreMsg is sent when more results are loaded
type loadMoreMsg struct {
	books []*anna.Book
	err   error
}

// loadingMsg indicates loading is in progress
type loadingMsg struct{}

// BookItem wraps a Book for the list component
type BookItem struct {
	Book *anna.Book
}

func (b BookItem) Title() string { return b.Book.Title }

func (b BookItem) Description() string {
	var parts []string

	if b.Book.Authors != "" {
		parts = append(parts, b.Book.Authors)
	}
	if b.Book.Format != "" {
		parts = append(parts, b.Book.Format)
	}
	if b.Book.Size != "" {
		parts = append(parts, b.Book.Size)
	}
	if b.Book.Language != "" {
		parts = append(parts, b.Book.Language)
	}

	if len(parts) == 0 {
		return DimStyle.Render("No metadata available")
	}
	return DimStyle.Render(strings.Join(parts, " | "))
}

func (b BookItem) FilterValue() string { return b.Book.Title }

// BookDelegate handles rendering of book items
type BookDelegate struct{}

func (d BookDelegate) Height() int                             { return 3 }
func (d BookDelegate) Spacing() int                            { return 0 }
func (d BookDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }

func (d BookDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	book, ok := item.(BookItem)
	if !ok {
		return
	}

	// Truncate title if too long
	title := book.Book.Title
	if len(title) > 60 {
		title = title[:57] + "..."
	}

	var str string
	if index == m.Index() {
		str = SelectedStyle.Render(fmt.Sprintf("  ➤ %d. %s", index+1, title))
		str += "\n" + DimStyle.Render(fmt.Sprintf("      %s", book.Description()))
		str += "\n" + DimStyle.Render(fmt.Sprintf("      MD5: %s", book.Book.MD5Hash[:16]+"..."))
	} else {
		str = NormalStyle.Render(fmt.Sprintf("    %d. %s", index+1, title))
		str += "\n" + DimStyle.Render(fmt.Sprintf("      %s", book.Description()))
		str += "\n" + DimStyle.Render(fmt.Sprintf("      MD5: %s", book.Book.MD5Hash[:16]+"..."))
	}

	fmt.Fprint(w, str)
}

// SelectorModel is the Bubble Tea model for book selection
type SelectorModel struct {
	list       list.Model
	selected   *anna.Book
	quitting   bool
	err        error
	loadMore   LoadMoreFunc
	loading    bool
	seenMD5s   map[string]bool
	noMoreResults bool
}

// NewSelector creates a new book selector TUI
func NewSelector(books []*anna.Book, title string) SelectorModel {
	return NewSelectorWithLoadMore(books, title, nil)
}

// NewSelectorWithLoadMore creates a new book selector TUI with load more support
func NewSelectorWithLoadMore(books []*anna.Book, title string, loadMore LoadMoreFunc) SelectorModel {
	items := make([]list.Item, len(books))
	seenMD5s := make(map[string]bool)
	for i, book := range books {
		items[i] = BookItem{Book: book}
		seenMD5s[book.MD5Hash] = true
	}

	delegate := BookDelegate{}
	l := list.New(items, delegate, 70, 4+len(books)*3)
	l.Title = title
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false)
	l.SetShowHelp(true)
	l.Styles.Title = TitleStyle

	return SelectorModel{
		list:     l,
		loadMore: loadMore,
		seenMD5s: seenMD5s,
	}
}

func (m SelectorModel) Init() tea.Cmd {
	return nil
}

func (m SelectorModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		// Don't handle keys while loading
		if m.loading {
			return m, nil
		}
		switch msg.String() {
		case "q", "ctrl+c", "esc":
			m.quitting = true
			return m, tea.Quit
		case "enter":
			if item, ok := m.list.SelectedItem().(BookItem); ok {
				m.selected = item.Book
			}
			return m, tea.Quit
		case "m", "M":
			// Load more results
			if m.loadMore != nil && !m.noMoreResults {
				m.loading = true
				return m, m.doLoadMore()
			}
		}
	case loadMoreMsg:
		m.loading = false
		if msg.err != nil {
			m.noMoreResults = true
			return m, nil
		}
		if len(msg.books) == 0 {
			m.noMoreResults = true
			return m, nil
		}
		// Add new books to the list (avoiding duplicates)
		newItems := make([]list.Item, 0, len(msg.books))
		for _, book := range msg.books {
			if !m.seenMD5s[book.MD5Hash] {
				m.seenMD5s[book.MD5Hash] = true
				newItems = append(newItems, BookItem{Book: book})
			}
		}
		if len(newItems) == 0 {
			m.noMoreResults = true
			return m, nil
		}
		// Append new items to the list
		currentItems := m.list.Items()
		allItems := append(currentItems, newItems...)
		m.list.SetItems(allItems)
		// Adjust height for new items
		m.list.SetHeight(4 + len(allItems)*3)
		return m, nil
	case tea.WindowSizeMsg:
		m.list.SetWidth(msg.Width)
		return m, nil
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

// doLoadMore returns a command that loads more results
func (m SelectorModel) doLoadMore() tea.Cmd {
	return func() tea.Msg {
		books, err := m.loadMore()
		return loadMoreMsg{books: books, err: err}
	}
}

func (m SelectorModel) View() string {
	if m.err != nil {
		return ErrorStyle.Render(fmt.Sprintf("\n  Error: %s\n", m.err.Error()))
	}

	if m.selected != nil {
		return SuccessStyle.Render(fmt.Sprintf("\n  ✓ Selected: %s\n", m.selected.Title))
	}

	if m.quitting {
		return DimStyle.Render("\n  Cancelled.\n")
	}

	// Show loading indicator
	if m.loading {
		return "\n" + m.list.View() + "\n" + WarningStyle.Render("  Loading more results...")
	}

	// Build help text
	helpParts := []string{"↑/↓: navigate", "enter: select"}
	if m.loadMore != nil && !m.noMoreResults {
		helpParts = append(helpParts, "m: more results")
	}
	helpParts = append(helpParts, "q/esc: cancel")
	help := HelpStyle.Render("  " + strings.Join(helpParts, " • "))

	return "\n" + m.list.View() + "\n" + help
}

// Selected returns the selected book
func (m SelectorModel) Selected() *anna.Book {
	return m.selected
}

// RunSelector displays the TUI and returns the selected book
func RunSelector(books []*anna.Book) (*anna.Book, error) {
	return RunSelectorWithLoadMore(books, nil)
}

// RunSelectorWithLoadMore displays the TUI with load more support and returns the selected book
func RunSelectorWithLoadMore(books []*anna.Book, loadMore LoadMoreFunc) (*anna.Book, error) {
	if len(books) == 0 {
		return nil, fmt.Errorf("no books to select from")
	}

	model := NewSelectorWithLoadMore(books, "Select a book to download", loadMore)
	p := tea.NewProgram(model)

	finalModel, err := p.Run()
	if err != nil {
		return nil, err
	}

	selector := finalModel.(SelectorModel)
	if selector.err != nil {
		return nil, selector.err
	}

	return selector.Selected(), nil
}
