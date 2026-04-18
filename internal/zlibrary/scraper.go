package zlibrary

import (
	"context"
	"errors"

	"github.com/billmal071/bookdl/internal/config"
)

var (
	// ErrCloudflareBlocked indicates Cloudflare challenge detected
	ErrCloudflareBlocked = errors.New("cloudflare challenge detected")
	// ErrNoResults indicates no search results found
	ErrNoResults = errors.New("no results found")
)

// ScraperClient scrapes Z-Library website
type ScraperClient struct {
	baseURL string
	browser *BrowserClient
}

// NewScraperClient creates a new scraper client
func NewScraperClient(baseURL string) *ScraperClient {
	if baseURL == "" {
		cfg := config.Get()
		baseURL = cfg.ZLibrary.BaseURL
		if baseURL == "" {
			baseURL = "z-library.sk"
		}
	}
	return &ScraperClient{
		baseURL: baseURL,
		browser: NewBrowserClient(baseURL),
	}
}

// Search searches for books by scraping the website
func (c *ScraperClient) Search(ctx context.Context, query string, limit int) ([]*Book, error) {
	return c.SearchPage(ctx, query, limit, 1)
}

// SearchPage delegates directly to the browser client because Z-Library uses
// client-side rendering (Web Components with shadow DOM) for search results.
// The initial HTML returned by a plain HTTP request contains no book data.
func (c *ScraperClient) SearchPage(ctx context.Context, query string, limit int, page int) ([]*Book, error) {
	return c.browser.SearchPage(ctx, query, limit, page)
}

// GetDownloadInfo retrieves download links for a book.
// Z-Library requires login to show download links on book detail pages and
// uses Web Components (z-bookcard) that need a real browser to render.
// We delegate directly to the browser client which extracts download URLs
// from search results without authentication.
func (c *ScraperClient) GetDownloadInfo(ctx context.Context, md5Hash string) (*DownloadInfo, error) {
	return c.browser.GetDownloadInfo(ctx, md5Hash)
}
