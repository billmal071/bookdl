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
