package dashboard

import (
	"fmt"
	"strings"
	"time"

	"github.com/billmal071/bookdl/internal/db"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type downloadsLoadedMsg struct {
	downloads []*db.Download
	err       error
}

type downloadPauseMsg struct{ err error }
type downloadResumeMsg struct{ err error }
type downloadCancelMsg struct{ err error }

// tickDownloadsMsg triggers periodic refresh of download state.
type tickDownloadsMsg time.Time

type downloadsModel struct {
	items       []*db.Download
	cursor      int
	focusDetail bool // true when detail pane has focus
	styles      Styles
	width       int
	height      int
	err         error
	loaded      bool
	// For speed/ETA calculation
	prevSizes map[int64]int64
	prevTime  time.Time
	speeds    map[int64]float64 // bytes per second
}

func newDownloadsModel(s Styles) downloadsModel {
	return downloadsModel{
		styles:    s,
		prevSizes: make(map[int64]int64),
		speeds:    make(map[int64]float64),
		prevTime:  time.Now(),
	}
}

func (m downloadsModel) Init() tea.Cmd {
	return tea.Batch(loadDownloads, tickDownloads())
}

func loadDownloads() tea.Msg {
	downloads, err := db.ListDownloads("", true)
	return downloadsLoadedMsg{downloads: downloads, err: err}
}

func tickDownloads() tea.Cmd {
	return tea.Tick(500*time.Millisecond, func(t time.Time) tea.Msg {
		return tickDownloadsMsg(t)
	})
}

func (m downloadsModel) Update(msg tea.Msg) (downloadsModel, tea.Cmd) {
	switch msg := msg.(type) {
	case downloadsLoadedMsg:
		m.loaded = true
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		m.updateSpeeds(msg.downloads)
		m.items = msg.downloads
		m.clampCursor()
		return m, nil

	case tickDownloadsMsg:
		return m, tea.Batch(loadDownloads, tickDownloads())

	case downloadPauseMsg, downloadResumeMsg, downloadCancelMsg:
		return m, loadDownloads

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
		case "tab":
			m.focusDetail = !m.focusDetail
			return m, nil
		case "p":
			if m.cursor < len(m.items) {
				d := m.items[m.cursor]
				if d.Status == db.StatusDownloading {
					return m, func() tea.Msg {
						err := db.UpdateStatus(d.ID, db.StatusPaused, "")
						return downloadPauseMsg{err: err}
					}
				}
			}
		case "r":
			if m.cursor < len(m.items) {
				d := m.items[m.cursor]
				if d.Status == db.StatusPaused || d.Status == db.StatusFailed {
					return m, func() tea.Msg {
						err := db.UpdateStatus(d.ID, db.StatusPending, "")
						return downloadResumeMsg{err: err}
					}
				}
			}
		case "x":
			if m.cursor < len(m.items) {
				d := m.items[m.cursor]
				return m, func() tea.Msg {
					err := db.DeleteDownload(d.ID)
					return downloadCancelMsg{err: err}
				}
			}
		}
	}
	return m, nil
}

func (m *downloadsModel) updateSpeeds(downloads []*db.Download) {
	now := time.Now()
	elapsed := now.Sub(m.prevTime).Seconds()
	if elapsed <= 0 {
		elapsed = 0.5
	}

	for _, d := range downloads {
		prev, ok := m.prevSizes[d.ID]
		if ok && d.Status == db.StatusDownloading {
			diff := d.DownloadedSize - prev
			if diff > 0 {
				m.speeds[d.ID] = float64(diff) / elapsed
			}
		}
		m.prevSizes[d.ID] = d.DownloadedSize
	}
	m.prevTime = now
}

func (m downloadsModel) View(l layout) string {
	var b strings.Builder
	b.Grow(2048)

	if !m.loaded {
		b.WriteString(m.styles.Spinner.Render("  Loading downloads..."))
		return b.String()
	}

	if m.err != nil {
		b.WriteString(m.styles.Error.Render(fmt.Sprintf("  Error: %v", m.err)))
		return b.String()
	}

	if len(m.items) == 0 {
		b.WriteString(m.styles.ItemMeta.Render("  No downloads."))
		return b.String()
	}

	if l.splitView {
		return m.viewSplit(l)
	}
	return m.viewSingle(l)
}

func (m downloadsModel) viewSingle(l layout) string {
	var b strings.Builder
	b.Grow(2048)

	maxW := l.contentWidth - 6

	b.WriteString(m.styles.Label.Render(fmt.Sprintf("  Downloads (%d):", len(m.items))))
	b.WriteString("\n\n")

	visible := (l.contentHeight - 3) / 3 // 3 lines per item
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
		icon, iconStyle := statusIcon(string(d.Status), m.styles)
		title := truncate(d.Title, maxW-4)

		if i == m.cursor {
			b.WriteString(m.styles.SelectedItem.Render(fmt.Sprintf("  ▸ %s ", icon)))
			b.WriteString(m.styles.SelectedItem.Render(title))
		} else {
			b.WriteString(fmt.Sprintf("    %s ", iconStyle.Render(icon)))
			b.WriteString(m.styles.NormalItem.Render(title))
		}
		b.WriteString("\n")

		// Progress bar
		if d.FileSize > 0 {
			pct := float64(d.DownloadedSize) / float64(d.FileSize) * 100
			fillStyle, emptyStyle := m.progressStyles(d.Status)
			barWidth := maxW - 10
			if barWidth > 30 {
				barWidth = 30
			}
			if barWidth < 10 {
				barWidth = 10
			}
			bar := renderProgressBar(pct, barWidth, fillStyle, emptyStyle)
			b.WriteString(fmt.Sprintf("       %s\n", bar))
		} else {
			b.WriteString(fmt.Sprintf("       %s\n", iconStyle.Render(string(d.Status))))
		}
	}

	return b.String()
}

func (m downloadsModel) viewSplit(l layout) string {
	listW, detailW := l.splitPaneWidths()

	listContent := m.renderList(listW, l.contentHeight)
	detailContent := m.renderDetail(detailW, l.contentHeight)

	listBorder := m.styles.BlurredBorder
	detailBorder := m.styles.BlurredBorder
	if !m.focusDetail {
		listBorder = m.styles.FocusedBorder
	} else {
		detailBorder = m.styles.FocusedBorder
	}

	// Fill content to exact height
	listLines := strings.Split(listContent, "\n")
	innerH := l.contentHeight
	for len(listLines) < innerH {
		listLines = append(listLines, "")
	}
	listLines = listLines[:innerH]

	detailLines := strings.Split(detailContent, "\n")
	for len(detailLines) < innerH {
		detailLines = append(detailLines, "")
	}
	detailLines = detailLines[:innerH]

	leftPanel := listBorder.
		Width(listW - 2). // subtract border
		Render(strings.Join(listLines, "\n"))

	rightPanel := detailBorder.
		Width(detailW - 2).
		Render(strings.Join(detailLines, "\n"))

	return lipgloss.JoinHorizontal(lipgloss.Top, leftPanel, rightPanel)
}

func (m downloadsModel) renderList(width, height int) string {
	var b strings.Builder
	b.Grow(1024)

	maxW := width - 6

	b.WriteString(m.styles.Label.Render(fmt.Sprintf(" Downloads (%d):", len(m.items))))
	b.WriteString("\n\n")

	visible := (height - 3) / 3
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
		icon, iconStyle := statusIcon(string(d.Status), m.styles)
		title := truncate(d.Title, maxW-4)

		if i == m.cursor {
			b.WriteString(m.styles.SelectedItem.Render(fmt.Sprintf(" ▸ %s %s", icon, title)))
		} else {
			b.WriteString(fmt.Sprintf("   %s %s", iconStyle.Render(icon), m.styles.NormalItem.Render(title)))
		}
		b.WriteString("\n")

		// Mini progress bar
		if d.FileSize > 0 {
			pct := float64(d.DownloadedSize) / float64(d.FileSize) * 100
			fillStyle, emptyStyle := m.progressStyles(d.Status)
			barW := maxW - 10
			if barW > 20 {
				barW = 20
			}
			if barW < 8 {
				barW = 8
			}
			bar := renderProgressBar(pct, barW, fillStyle, emptyStyle)
			b.WriteString(fmt.Sprintf("     %s\n", bar))
		} else {
			b.WriteString(fmt.Sprintf("     %s\n", iconStyle.Render(string(d.Status))))
		}
	}

	return b.String()
}

func (m downloadsModel) renderDetail(width, height int) string {
	if m.cursor >= len(m.items) {
		return ""
	}

	d := m.items[m.cursor]
	maxW := width - 4

	var b strings.Builder
	b.Grow(512)

	b.WriteString(m.styles.Label.Render(" Details"))
	b.WriteString("\n\n")

	// Metadata
	fields := []struct{ label, value string }{
		{"Title", truncate(d.Title, maxW-12)},
		{"Author", truncate(d.Authors, maxW-12)},
		{"Format", d.Format},
	}
	if d.FileSize > 0 {
		fields = append(fields, struct{ label, value string }{"Size", formatSize(d.FileSize)})
	}
	fields = append(fields, struct{ label, value string }{"Source", d.Source})

	icon, iconStyle := statusIcon(string(d.Status), m.styles)
	fields = append(fields, struct{ label, value string }{"Status", ""})

	for _, f := range fields {
		label := m.styles.Label.Render(fmt.Sprintf(" %-9s", f.label))
		if f.label == "Status" {
			b.WriteString(label + iconStyle.Render(fmt.Sprintf("%s %s", icon, d.Status)))
		} else {
			b.WriteString(label + m.styles.Value.Render(f.value))
		}
		b.WriteString("\n")
	}

	b.WriteString("\n")

	// Large progress bar
	if d.FileSize > 0 {
		pct := float64(d.DownloadedSize) / float64(d.FileSize) * 100
		fillStyle, emptyStyle := m.progressStyles(d.Status)
		barW := maxW - 8
		if barW > 30 {
			barW = 30
		}
		if barW < 10 {
			barW = 10
		}
		bar := renderProgressBar(pct, barW, fillStyle, emptyStyle)
		b.WriteString(fmt.Sprintf(" %s\n", bar))
		b.WriteString(m.styles.ItemMeta.Render(fmt.Sprintf(" %s / %s",
			formatSize(d.DownloadedSize), formatSize(d.FileSize))))
		b.WriteString("\n")

		// Speed and ETA
		speed := m.speeds[d.ID]
		if speed > 0 && d.Status == db.StatusDownloading {
			remaining := float64(d.FileSize-d.DownloadedSize) / speed
			eta := formatETA(remaining)
			b.WriteString(m.styles.Label.Render(" Speed  "))
			b.WriteString(m.styles.Value.Render(formatSpeed(speed)))
			b.WriteString(m.styles.Label.Render("     ETA  "))
			b.WriteString(m.styles.Value.Render(eta))
			b.WriteString("\n")
		}
	}

	// Error message
	if d.ErrorMessage != "" {
		b.WriteString("\n")
		b.WriteString(m.styles.Error.Render(fmt.Sprintf(" Error: %s", truncate(d.ErrorMessage, maxW-8))))
		b.WriteString("\n")
	}

	return b.String()
}

func (m downloadsModel) progressStyles(status db.DownloadStatus) (lipgloss.Style, lipgloss.Style) {
	emptyStyle := lipgloss.NewStyle().Foreground(m.styles.BlurredBorder.GetBorderBottomForeground())
	switch status {
	case db.StatusDownloading:
		return m.styles.StatusDownloading, emptyStyle
	case db.StatusPaused:
		return m.styles.StatusPaused, emptyStyle
	case db.StatusCompleted:
		return m.styles.StatusCompleted, emptyStyle
	case db.StatusFailed:
		return m.styles.StatusFailed, emptyStyle
	default:
		return m.styles.StatusPending, emptyStyle
	}
}

func formatETA(seconds float64) string {
	if seconds <= 0 {
		return "0s"
	}
	if seconds < 60 {
		return fmt.Sprintf("%.0fs", seconds)
	}
	if seconds < 3600 {
		return fmt.Sprintf("%.0fm%02.0fs", seconds/60, float64(int(seconds)%60))
	}
	return fmt.Sprintf("%.0fh%02.0fm", seconds/3600, float64(int(seconds)%3600)/60)
}

func (m *downloadsModel) clampCursor() {
	if m.cursor >= len(m.items) {
		m.cursor = len(m.items) - 1
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
}

func (m *downloadsModel) setSize(w, h int) {
	m.width = w
	m.height = h
}
