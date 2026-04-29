package dashboard

import (
	"strings"
)

type tab int

const (
	tabSearch tab = iota
	tabDownloads
	tabQueue
	tabBookmarks
	tabHistory
	tabCount
)

var tabNames = [tabCount]string{
	"Search",
	"Downloads",
	"Queue",
	"Bookmarks",
	"History",
}

func (t tab) String() string {
	if t >= 0 && t < tabCount {
		return tabNames[t]
	}
	return "?"
}

func (t tab) next() tab {
	return (t + 1) % tabCount
}

func (t tab) prev() tab {
	return (t - 1 + tabCount) % tabCount
}

func renderTabBar(active tab, s Styles, width int) string {
	var b strings.Builder

	b.WriteString(s.AppName.Render("bookdl"))
	b.WriteString("    ")

	for i := tab(0); i < tabCount; i++ {
		name := i.String()
		if i == active {
			b.WriteString(s.ActiveTab.Render(name))
			b.WriteString(s.ActiveTab.Render(" ━━"))
		} else {
			b.WriteString(s.InactiveTab.Render(name))
		}
		if i < tabCount-1 {
			b.WriteString("    ")
		}
	}

	line := b.String()
	separator := s.TabSeparator.Render(strings.Repeat("─", width))

	return line + "\n" + separator
}
