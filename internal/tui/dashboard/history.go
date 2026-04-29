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
	items   []*db.SearchHistory
	cursor  int
	styles  Styles
	width   int
	height  int
	err     error
	loaded  bool
	confirm bool // confirming clear all
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
