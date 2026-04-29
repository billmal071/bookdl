package dashboard

type layout struct {
	width         int
	height        int
	contentWidth  int
	contentHeight int
	splitView     bool
}

const (
	tabBarHeight    = 2
	statusBarHeight = 1
	panelBorderH    = 2

	splitViewMinWidth = 120
)

func computeLayout(termWidth, termHeight int) layout {
	l := layout{
		width:  termWidth,
		height: termHeight,
	}
	l.contentWidth = termWidth
	l.contentHeight = termHeight - tabBarHeight - statusBarHeight - panelBorderH
	if l.contentHeight < 1 {
		l.contentHeight = 1
	}
	l.splitView = termWidth >= splitViewMinWidth
	return l
}

func (l layout) splitPaneWidths() (int, int) {
	if !l.splitView {
		return l.contentWidth, 0
	}
	available := l.contentWidth - 1
	listWeight, detailWeight := 2, 3
	totalWeight := listWeight + detailWeight
	listW := (available * listWeight) / totalWeight
	detailW := available - listW
	return listW, detailW
}
