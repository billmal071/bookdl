package tui

import (
	"fmt"
	"io"
	"os/exec"
	"runtime"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/billmal071/bookdl/internal/anna"
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
type BookDelegate struct {
	selectedMD5s map[string]bool // For multi-select mode
}

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

	// Check if this item is selected (multi-select mode)
	isChecked := d.selectedMD5s != nil && d.selectedMD5s[book.Book.MD5Hash]
	checkbox := "[ ]"
	if isChecked {
		checkbox = "[✓]"
	}

	var str string
	if index == m.Index() {
		if d.selectedMD5s != nil {
			// Multi-select mode
			if isChecked {
				str = SuccessStyle.Render(fmt.Sprintf("➤ %s %d. %s", checkbox, index+1, title))
			} else {
				str = SelectedStyle.Render(fmt.Sprintf("➤ %s %d. %s", checkbox, index+1, title))
			}
		} else {
			str = SelectedStyle.Render(fmt.Sprintf("  ➤ %d. %s", index+1, title))
		}
		str += "\n" + DimStyle.Render(fmt.Sprintf("      %s", book.Description()))
		str += "\n" + DimStyle.Render(fmt.Sprintf("      MD5: %s", book.Book.MD5Hash[:16]+"..."))
	} else {
		if d.selectedMD5s != nil {
			// Multi-select mode
			if isChecked {
				str = SuccessStyle.Render(fmt.Sprintf("  %s %d. %s", checkbox, index+1, title))
			} else {
				str = NormalStyle.Render(fmt.Sprintf("  %s %d. %s", checkbox, index+1, title))
			}
		} else {
			str = NormalStyle.Render(fmt.Sprintf("    %d. %s", index+1, title))
		}
		str += "\n" + DimStyle.Render(fmt.Sprintf("      %s", book.Description()))
		str += "\n" + DimStyle.Render(fmt.Sprintf("      MD5: %s", book.Book.MD5Hash[:16]+"..."))
	}

	fmt.Fprint(w, str)
}

// SelectorModel is the Bubble Tea model for book selection
type SelectorModel struct {
	list          list.Model
	selected      *anna.Book
	multiSelected []*anna.Book
	quitting      bool
	err           error
	loadMore      LoadMoreFunc
	loading       bool
	seenMD5s      map[string]bool
	noMoreResults bool
	showDetails   bool
	browserMsg    string
	multiSelect   bool
	checkedMD5s   map[string]bool
}

// NewSelector creates a new book selector TUI
func NewSelector(books []*anna.Book, title string) SelectorModel {
	return NewSelectorWithLoadMore(books, title, nil)
}

// NewSelectorWithLoadMore creates a new book selector TUI with load more support
func NewSelectorWithLoadMore(books []*anna.Book, title string, loadMore LoadMoreFunc) SelectorModel {
	return newSelector(books, title, loadMore, false)
}

// NewMultiSelector creates a new book selector TUI with multi-select support
func NewMultiSelector(books []*anna.Book, title string, loadMore LoadMoreFunc) SelectorModel {
	return newSelector(books, title, loadMore, true)
}

// newSelector is the internal constructor for both single and multi-select modes
func newSelector(books []*anna.Book, title string, loadMore LoadMoreFunc, multiSelect bool) SelectorModel {
	items := make([]list.Item, len(books))
	seenMD5s := make(map[string]bool)
	for i, book := range books {
		items[i] = BookItem{Book: book}
		seenMD5s[book.MD5Hash] = true
	}

	var checkedMD5s map[string]bool
	delegate := BookDelegate{}
	if multiSelect {
		checkedMD5s = make(map[string]bool)
		delegate.selectedMD5s = checkedMD5s
	}

	// Use a fixed reasonable height that allows scrolling
	// The list component handles scrolling internally
	l := list.New(items, delegate, 80, 20)
	l.Title = title
	l.SetShowStatusBar(true)
	l.SetFilteringEnabled(false)
	l.SetShowHelp(false) // We show our own help
	l.Styles.Title = TitleStyle

	return SelectorModel{
		list:        l,
		loadMore:    loadMore,
		seenMD5s:    seenMD5s,
		multiSelect: multiSelect,
		checkedMD5s: checkedMD5s,
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
			if m.multiSelect {
				// In multi-select mode, confirm selection
				m.multiSelected = m.getCheckedBooks()
				if len(m.multiSelected) == 0 {
					// If nothing checked, select current item
					if item, ok := m.list.SelectedItem().(BookItem); ok {
						m.multiSelected = []*anna.Book{item.Book}
					}
				}
			} else {
				// Single select mode
				if item, ok := m.list.SelectedItem().(BookItem); ok {
					m.selected = item.Book
				}
			}
			return m, tea.Quit
		case " ":
			// Toggle selection in multi-select mode
			if m.multiSelect {
				if item, ok := m.list.SelectedItem().(BookItem); ok {
					md5 := item.Book.MD5Hash
					if m.checkedMD5s[md5] {
						delete(m.checkedMD5s, md5)
					} else {
						m.checkedMD5s[md5] = true
					}
					// Update delegate's reference
					m.updateDelegate()
				}
				return m, nil
			}
		case "a", "A":
			// Select all in multi-select mode
			if m.multiSelect {
				for _, item := range m.list.Items() {
					if book, ok := item.(BookItem); ok {
						m.checkedMD5s[book.Book.MD5Hash] = true
					}
				}
				m.updateDelegate()
				return m, nil
			}
		case "n", "N":
			// Deselect all in multi-select mode
			if m.multiSelect {
				m.checkedMD5s = make(map[string]bool)
				m.updateDelegate()
				return m, nil
			}
		case "m", "M":
			// Load more results
			if m.loadMore != nil && !m.noMoreResults {
				m.loading = true
				return m, m.doLoadMore()
			}
		case "i", "I":
			// Toggle details view
			m.showDetails = !m.showDetails
			m.browserMsg = ""
			return m, nil
		case "o", "O":
			// Open book page in browser
			if item, ok := m.list.SelectedItem().(BookItem); ok {
				if item.Book.PageURL != "" {
					if err := openBrowser(item.Book.PageURL); err != nil {
						m.browserMsg = ErrorStyle.Render("Failed to open browser")
					} else {
						m.browserMsg = SuccessStyle.Render("Opened in browser")
					}
				} else {
					m.browserMsg = WarningStyle.Render("No URL available")
				}
			}
			return m, nil
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
		// Don't change height - let the list handle scrolling
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

// getCheckedBooks returns the list of checked books in multi-select mode
func (m SelectorModel) getCheckedBooks() []*anna.Book {
	var books []*anna.Book
	for _, item := range m.list.Items() {
		if book, ok := item.(BookItem); ok {
			if m.checkedMD5s[book.Book.MD5Hash] {
				books = append(books, book.Book)
			}
		}
	}
	return books
}

// updateDelegate updates the list delegate with current selection state
func (m *SelectorModel) updateDelegate() {
	delegate := BookDelegate{selectedMD5s: m.checkedMD5s}
	m.list.SetDelegate(delegate)
}

// openBrowser opens a URL in the default browser
func openBrowser(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "linux":
		cmd = exec.Command("xdg-open", url)
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		return fmt.Errorf("unsupported platform")
	}
	return cmd.Start()
}

// renderDetailsView renders the book details panel
func (m SelectorModel) renderDetailsView() string {
	item, ok := m.list.SelectedItem().(BookItem)
	if !ok {
		return ""
	}
	book := item.Book

	var sb strings.Builder
	sb.WriteString(TitleStyle.Render("📖 Book Details") + "\n\n")

	// Title
	sb.WriteString(LabelStyle.Render("Title:    "))
	sb.WriteString(ValueStyle.Render(book.Title) + "\n")

	// Authors
	if book.Authors != "" {
		sb.WriteString(LabelStyle.Render("Authors:  "))
		sb.WriteString(ValueStyle.Render(book.Authors) + "\n")
	}

	// Publisher
	if book.Publisher != "" {
		sb.WriteString(LabelStyle.Render("Publisher:"))
		sb.WriteString(ValueStyle.Render(" " + book.Publisher) + "\n")
	}

	// Year
	if book.Year != "" {
		sb.WriteString(LabelStyle.Render("Year:     "))
		sb.WriteString(ValueStyle.Render(book.Year) + "\n")
	}

	// Language
	if book.Language != "" {
		sb.WriteString(LabelStyle.Render("Language: "))
		sb.WriteString(ValueStyle.Render(book.Language) + "\n")
	}

	// Format
	if book.Format != "" {
		sb.WriteString(LabelStyle.Render("Format:   "))
		sb.WriteString(ValueStyle.Render(book.Format) + "\n")
	}

	// Size
	if book.Size != "" {
		sb.WriteString(LabelStyle.Render("Size:     "))
		sb.WriteString(ValueStyle.Render(book.Size) + "\n")
	}

	// MD5 Hash
	sb.WriteString(LabelStyle.Render("MD5:      "))
	sb.WriteString(ValueStyle.Render(book.MD5Hash) + "\n")

	// Page URL
	if book.PageURL != "" {
		sb.WriteString(LabelStyle.Render("URL:      "))
		// Truncate URL if too long
		url := book.PageURL
		if len(url) > 50 {
			url = url[:47] + "..."
		}
		sb.WriteString(DimStyle.Render(url) + "\n")
	}

	return DetailsBoxStyle.Render(sb.String())
}

func (m SelectorModel) View() string {
	if m.err != nil {
		return ErrorStyle.Render(fmt.Sprintf("\n  Error: %s\n", m.err.Error()))
	}

	// Show completion messages
	if m.multiSelect && len(m.multiSelected) > 0 {
		var titles []string
		for _, book := range m.multiSelected {
			titles = append(titles, book.Title)
		}
		return SuccessStyle.Render(fmt.Sprintf("\n  ✓ Selected %d book(s):\n    - %s\n",
			len(m.multiSelected), strings.Join(titles, "\n    - ")))
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

	// Build help text based on mode
	var helpParts []string
	if m.multiSelect {
		helpParts = []string{"↑/↓: navigate", "space: toggle", "a: all", "n: none", "enter: confirm", "i: details"}
	} else {
		helpParts = []string{"↑/↓: navigate", "enter: select", "i: details"}
	}
	if m.showDetails {
		helpParts = append(helpParts, "o: open in browser")
	}
	if m.loadMore != nil && !m.noMoreResults {
		helpParts = append(helpParts, "m: more")
	}
	helpParts = append(helpParts, "q: cancel")
	help := HelpStyle.Render("  " + strings.Join(helpParts, " • "))

	// Build the view
	var view strings.Builder
	view.WriteString("\n")
	view.WriteString(m.list.View())

	// Show selection count in multi-select mode
	if m.multiSelect && len(m.checkedMD5s) > 0 {
		view.WriteString("\n")
		view.WriteString(SuccessStyle.Render(fmt.Sprintf("  %d book(s) selected", len(m.checkedMD5s))))
	}

	// Show details panel if enabled
	if m.showDetails {
		view.WriteString("\n")
		view.WriteString(m.renderDetailsView())
	}

	// Show browser message if any
	if m.browserMsg != "" {
		view.WriteString("\n  " + m.browserMsg)
	}

	view.WriteString("\n")
	view.WriteString(help)

	return view.String()
}

// Selected returns the selected book
func (m SelectorModel) Selected() *anna.Book {
	return m.selected
}

// MultiSelected returns the multi-selected books
func (m SelectorModel) MultiSelected() []*anna.Book {
	return m.multiSelected
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

// RunMultiSelector displays the TUI with multi-select support and returns selected books
func RunMultiSelector(books []*anna.Book, loadMore LoadMoreFunc) ([]*anna.Book, error) {
	if len(books) == 0 {
		return nil, fmt.Errorf("no books to select from")
	}

	model := NewMultiSelector(books, "Select books to queue (space to toggle)", loadMore)
	p := tea.NewProgram(model)

	finalModel, err := p.Run()
	if err != nil {
		return nil, err
	}

	selector := finalModel.(SelectorModel)
	if selector.err != nil {
		return nil, selector.err
	}

	return selector.MultiSelected(), nil
}
