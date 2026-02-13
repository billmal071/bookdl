package tui

import (
	"fmt"
	"io"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/billmal071/bookdl/internal/db"
)

// HistoryItem wraps a SearchHistory for the list component
type HistoryItem struct {
	History *db.SearchHistory
}

func (h HistoryItem) Title() string { return h.History.Query }

func (h HistoryItem) Description() string {
	var parts []string
	parts = append(parts, fmt.Sprintf("%d results", h.History.ResultCount))

	var filterParts []string
	if h.History.Filters.Format != "" {
		filterParts = append(filterParts, "format="+h.History.Filters.Format)
	}
	if h.History.Filters.Language != "" {
		filterParts = append(filterParts, "language="+h.History.Filters.Language)
	}
	if h.History.Filters.Year != "" {
		filterParts = append(filterParts, "year="+h.History.Filters.Year)
	}
	if h.History.Filters.MaxSize != "" {
		filterParts = append(filterParts, "max-size="+h.History.Filters.MaxSize)
	}
	if len(filterParts) > 0 {
		parts = append(parts, strings.Join(filterParts, ", "))
	}

	parts = append(parts, h.History.CreatedAt.Format("2006-01-02 15:04"))

	return DimStyle.Render(strings.Join(parts, " | "))
}

func (h HistoryItem) FilterValue() string { return h.History.Query }

// HistoryDelegate handles rendering of history items
type HistoryDelegate struct{}

func (d HistoryDelegate) Height() int                             { return 2 }
func (d HistoryDelegate) Spacing() int                            { return 1 }
func (d HistoryDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }

func (d HistoryDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	history, ok := item.(HistoryItem)
	if !ok {
		return
	}

	// Truncate query if too long
	query := history.History.Query
	if len(query) > 70 {
		query = query[:67] + "..."
	}

	var str string
	if index == m.Index() {
		str = SelectedStyle.Render(fmt.Sprintf("  ➤ %d. %s", index+1, query))
	} else {
		str = NormalStyle.Render(fmt.Sprintf("    %d. %s", index+1, query))
	}
	str += "\n" + DimStyle.Render(fmt.Sprintf("      %s", history.Description()))

	fmt.Fprint(w, str)
}

// HistorySelectorModel is the Bubble Tea model for history selection
type HistorySelectorModel struct {
	list     list.Model
	selected *db.SearchHistory
	quitting bool
	err      error
}

// NewHistorySelector creates a new history selector TUI
func NewHistorySelector(history []*db.SearchHistory) HistorySelectorModel {
	items := make([]list.Item, len(history))
	for i, h := range history {
		items[i] = HistoryItem{History: h}
	}

	delegate := HistoryDelegate{}
	l := list.New(items, delegate, 80, 20)
	l.Title = "Search History"
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(true)
	l.SetShowHelp(false)
	l.Styles.Title = TitleStyle

	return HistorySelectorModel{
		list: l,
	}
}

func (m HistorySelectorModel) Init() tea.Cmd {
	return nil
}

func (m HistorySelectorModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c", "esc":
			m.quitting = true
			return m, tea.Quit
		case "enter":
			if item, ok := m.list.SelectedItem().(HistoryItem); ok {
				m.selected = item.History
			}
			return m, tea.Quit
		}
	case tea.WindowSizeMsg:
		m.list.SetWidth(msg.Width)
		return m, nil
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m HistorySelectorModel) View() string {
	if m.err != nil {
		return ErrorStyle.Render(fmt.Sprintf("\n  Error: %s\n", m.err.Error()))
	}

	if m.selected != nil {
		return SuccessStyle.Render(fmt.Sprintf("\n  ✓ Selected: %s\n", m.selected.Query))
	}

	if m.quitting {
		return DimStyle.Render("\n  Cancelled.\n")
	}

	help := HelpStyle.Render("  ↑/↓: navigate • enter: select • /: filter • q: cancel")

	var view strings.Builder
	view.WriteString("\n")
	view.WriteString(m.list.View())
	view.WriteString("\n")
	view.WriteString(help)

	return view.String()
}

// Selected returns the selected history item
func (m HistorySelectorModel) Selected() *db.SearchHistory {
	return m.selected
}

// RunHistorySelector displays the TUI and returns the selected search history
func RunHistorySelector(history []*db.SearchHistory) (*db.SearchHistory, error) {
	if len(history) == 0 {
		return nil, fmt.Errorf("no search history available")
	}

	model := NewHistorySelector(history)
	p := tea.NewProgram(model)

	finalModel, err := p.Run()
	if err != nil {
		return nil, err
	}

	selector := finalModel.(HistorySelectorModel)
	if selector.err != nil {
		return nil, selector.err
	}

	return selector.Selected(), nil
}
