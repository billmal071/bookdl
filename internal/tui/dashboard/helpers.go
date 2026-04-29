package dashboard

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// truncate cuts a string to maxLen, appending "…" if truncated.
func truncate(s string, maxLen int) string {
	if maxLen <= 0 {
		return ""
	}
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	if maxLen <= 1 {
		return "…"
	}
	return string(runes[:maxLen-1]) + "…"
}

// renderProgressBar draws a progress bar: ████▒▒▒▒ 62%
func renderProgressBar(percent float64, width int, fillStyle, emptyStyle lipgloss.Style) string {
	if width <= 0 {
		return ""
	}
	if percent < 0 {
		percent = 0
	}
	if percent > 100 {
		percent = 100
	}
	filled := int(percent / 100 * float64(width))
	if filled > width {
		filled = width
	}
	empty := width - filled

	bar := fillStyle.Render(strings.Repeat("█", filled)) +
		emptyStyle.Render(strings.Repeat("▒", empty))

	return fmt.Sprintf("%s %3.0f%%", bar, percent)
}

// formatSize converts bytes to human-readable format.
func formatSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// formatSpeed converts bytes/sec to human-readable format.
func formatSpeed(bytesPerSec float64) string {
	if bytesPerSec <= 0 {
		return "0 B/s"
	}
	return formatSize(int64(bytesPerSec)) + "/s"
}

// statusIcon returns a text glyph and style for a download status.
func statusIcon(status string, s Styles) (string, lipgloss.Style) {
	switch status {
	case "downloading":
		return "⬇", s.StatusDownloading
	case "paused":
		return "‖", s.StatusPaused
	case "completed":
		return "✓", s.StatusCompleted
	case "failed":
		return "✗", s.StatusFailed
	default:
		return "○", s.StatusPending
	}
}

// joinMeta joins non-empty strings with " · " separator.
func joinMeta(parts ...string) string {
	var nonEmpty []string
	for _, p := range parts {
		if p != "" {
			nonEmpty = append(nonEmpty, p)
		}
	}
	return strings.Join(nonEmpty, "  ·  ")
}
