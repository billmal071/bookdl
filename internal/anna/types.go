package anna

import "context"

// Book represents a book from Anna's Archive
type Book struct {
	MD5Hash   string `json:"md5"`
	Title     string `json:"title"`
	Authors   string `json:"authors"`
	Publisher string `json:"publisher"`
	Year      string `json:"year"`
	Language  string `json:"language"`
	Format    string `json:"format"`
	Size      string `json:"size"`
	SizeBytes int64  `json:"size_bytes"`
	PageURL   string `json:"page_url"`
}

// SearchResult contains search results with metadata
type SearchResult struct {
	Books      []*Book `json:"books"`
	TotalCount int     `json:"total_count"`
	Query      string  `json:"query"`
}

// DownloadInfo contains information needed to download a book
type DownloadInfo struct {
	DirectURL  string `json:"direct_url"`
	MirrorURLs []string `json:"mirror_urls"`
	Filename   string `json:"filename"`
	FileSize   int64  `json:"file_size"`
}

// Client defines the interface for Anna's Archive access
type Client interface {
	// Search searches for books matching the query
	Search(ctx context.Context, query string, limit int) ([]*Book, error)

	// SearchPage searches for books with pagination support
	SearchPage(ctx context.Context, query string, limit int, page int) ([]*Book, error)

	// GetDownloadInfo retrieves download URLs for a book
	GetDownloadInfo(ctx context.Context, md5Hash string) (*DownloadInfo, error)
}
