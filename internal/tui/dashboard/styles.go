package dashboard

import "github.com/charmbracelet/lipgloss"

type Styles struct {
	AppName       lipgloss.Style
	ActiveTab     lipgloss.Style
	InactiveTab   lipgloss.Style
	TabSeparator  lipgloss.Style
	SelectedItem  lipgloss.Style
	NormalItem    lipgloss.Style
	ItemMeta      lipgloss.Style
	Cursor        lipgloss.Style
	StatusDownloading lipgloss.Style
	StatusPaused      lipgloss.Style
	StatusCompleted   lipgloss.Style
	StatusFailed      lipgloss.Style
	StatusPending     lipgloss.Style
	FocusedBorder lipgloss.Style
	BlurredBorder lipgloss.Style
	Label lipgloss.Style
	Value lipgloss.Style
	HelpKey  lipgloss.Style
	HelpDesc lipgloss.Style
	SearchBorderFocused lipgloss.Style
	SearchBorderBlurred lipgloss.Style
	Error   lipgloss.Style
	Warning lipgloss.Style
	Success lipgloss.Style
	Spinner lipgloss.Style
}

func NewStyles(t Theme) Styles {
	return Styles{
		AppName:     lipgloss.NewStyle().Bold(true).Foreground(t.Secondary),
		ActiveTab:   lipgloss.NewStyle().Bold(true).Foreground(t.Primary),
		InactiveTab: lipgloss.NewStyle().Foreground(t.Subtext),
		TabSeparator: lipgloss.NewStyle().Foreground(t.Border),
		SelectedItem: lipgloss.NewStyle().Bold(true).Foreground(t.Primary),
		NormalItem:   lipgloss.NewStyle().Foreground(t.Text),
		ItemMeta:     lipgloss.NewStyle().Foreground(t.Subtext),
		Cursor:       lipgloss.NewStyle().Bold(true).Foreground(t.Primary),
		StatusDownloading: lipgloss.NewStyle().Foreground(t.Downloading),
		StatusPaused:      lipgloss.NewStyle().Foreground(t.Paused),
		StatusCompleted:   lipgloss.NewStyle().Foreground(t.Completed),
		StatusFailed:      lipgloss.NewStyle().Foreground(t.Failed),
		StatusPending:     lipgloss.NewStyle().Foreground(t.Pending),
		FocusedBorder: lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(t.Primary),
		BlurredBorder: lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(t.Border),
		Label: lipgloss.NewStyle().Foreground(t.Secondary).Bold(true),
		Value: lipgloss.NewStyle().Foreground(t.Text),
		HelpKey:  lipgloss.NewStyle().Bold(true).Foreground(t.Primary),
		HelpDesc: lipgloss.NewStyle().Foreground(t.Subtext),
		SearchBorderFocused: lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(t.Primary).Padding(0, 1),
		SearchBorderBlurred: lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(t.Border).Padding(0, 1),
		Error:   lipgloss.NewStyle().Foreground(t.Failed),
		Warning: lipgloss.NewStyle().Foreground(t.Paused),
		Success: lipgloss.NewStyle().Foreground(t.Completed),
		Spinner: lipgloss.NewStyle().Foreground(t.Secondary),
	}
}
