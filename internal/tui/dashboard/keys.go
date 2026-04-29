package dashboard

type focus int

const (
	focusSearchInput focus = iota
	focusSearchResults
	focusDownloadList
	focusDownloadDetail
	focusQueueList
	focusBookmarkList
	focusBookmarkNote
	focusHistoryList
	focusConfirmDialog
)

func (f focus) isTextInput() bool {
	switch f {
	case focusSearchInput, focusBookmarkNote:
		return true
	}
	return false
}

func (f focus) panelTab() tab {
	switch f {
	case focusSearchInput, focusSearchResults:
		return tabSearch
	case focusDownloadList, focusDownloadDetail:
		return tabDownloads
	case focusQueueList:
		return tabQueue
	case focusBookmarkList, focusBookmarkNote:
		return tabBookmarks
	case focusHistoryList:
		return tabHistory
	default:
		return tabSearch
	}
}

func defaultFocusForTab(t tab) focus {
	switch t {
	case tabSearch:
		return focusSearchInput
	case tabDownloads:
		return focusDownloadList
	case tabQueue:
		return focusQueueList
	case tabBookmarks:
		return focusBookmarkList
	case tabHistory:
		return focusHistoryList
	default:
		return focusSearchInput
	}
}
