package dashboard

import "github.com/charmbracelet/lipgloss"

type Theme struct {
	Downloading lipgloss.AdaptiveColor
	Paused      lipgloss.AdaptiveColor
	Completed   lipgloss.AdaptiveColor
	Failed      lipgloss.AdaptiveColor
	Pending     lipgloss.AdaptiveColor
	Primary   lipgloss.AdaptiveColor
	Secondary lipgloss.AdaptiveColor
	Text      lipgloss.AdaptiveColor
	Subtext   lipgloss.AdaptiveColor
	Border    lipgloss.AdaptiveColor
	Highlight lipgloss.AdaptiveColor
}

var DefaultTheme = Theme{
	Downloading: lipgloss.AdaptiveColor{Light: "#e65100", Dark: "#ffb86c"},
	Paused:      lipgloss.AdaptiveColor{Light: "#f9a825", Dark: "#f1fa8c"},
	Completed:   lipgloss.AdaptiveColor{Light: "#2e7d32", Dark: "#50fa7b"},
	Failed:      lipgloss.AdaptiveColor{Light: "#c62828", Dark: "#ff5555"},
	Pending:     lipgloss.AdaptiveColor{Light: "#757575", Dark: "#6272a4"},
	Primary:   lipgloss.AdaptiveColor{Light: "#7c4dff", Dark: "#bd93f9"},
	Secondary: lipgloss.AdaptiveColor{Light: "#1565c0", Dark: "#8be9fd"},
	Text:      lipgloss.AdaptiveColor{Light: "#1a1a2e", Dark: "#f8f8f2"},
	Subtext:   lipgloss.AdaptiveColor{Light: "#757575", Dark: "#6272a4"},
	Border:    lipgloss.AdaptiveColor{Light: "#bdbdbd", Dark: "#44475a"},
	Highlight: lipgloss.AdaptiveColor{Light: "#7c4dff", Dark: "#bd93f9"},
}
