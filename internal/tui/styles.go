package tui

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
)

var (
	// Colors
	primaryColor   = lipgloss.Color("170") // Purple
	secondaryColor = lipgloss.Color("39")  // Cyan
	dimColor       = lipgloss.Color("240") // Gray
	successColor   = lipgloss.Color("82")  // Green
	errorColor     = lipgloss.Color("196") // Red
	warningColor   = lipgloss.Color("214") // Orange

	// Title style
	TitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(primaryColor).
			MarginBottom(1)

	// Selected item style
	SelectedStyle = lipgloss.NewStyle().
			Foreground(primaryColor).
			Bold(true)

	// Normal item style
	NormalStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("252"))

	// Dim style for metadata
	DimStyle = lipgloss.NewStyle().
			Foreground(dimColor)

	// Success style
	SuccessStyle = lipgloss.NewStyle().
			Foreground(successColor)

	// Error style
	ErrorStyle = lipgloss.NewStyle().
			Foreground(errorColor)

	// Warning style
	WarningStyle = lipgloss.NewStyle().
			Foreground(warningColor)

	// Box style for containers
	BoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(dimColor).
			Padding(1, 2)

	// Help style
	HelpStyle = lipgloss.NewStyle().
			Foreground(dimColor).
			MarginTop(1)

	// Progress bar styles
	ProgressStyle = lipgloss.NewStyle().
			Foreground(secondaryColor)

	ProgressCompleteStyle = lipgloss.NewStyle().
				Foreground(successColor)
)

// FormatSize formats bytes into human readable format
func FormatSize(bytes int64) string {
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
