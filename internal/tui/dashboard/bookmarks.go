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
	items       []*db.Bookmark
	cursor      int
	styles      Styles
	width       int
	height      int
	err         error
	loaded      bool
	editing     bool // editing a note
	noteInput   textinput.Model
	filter      string
	filtering   bool
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
