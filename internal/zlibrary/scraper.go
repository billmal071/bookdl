package zlibrary

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/gocolly/colly/v2"
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
// Attempts a lightweight HTTP scrape first; falls back to browser on Cloudflare or empty results.
func (c *ScraperClient) GetDownloadInfo(ctx context.Context, md5Hash string) (*DownloadInfo, error) {
	var info *DownloadInfo
	var cloudflareDetected bool

	collector := colly.NewCollector(
		colly.AllowedDomains(c.baseURL),
		colly.UserAgent("Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"),
	)

	collector.SetRequestTimeout(30 * time.Second)

	collector.OnResponse(func(r *colly.Response) {
		body := string(r.Body)
		if strings.Contains(body, "cf-browser-verification") ||
			strings.Contains(body, "Just a moment...") {
			cloudflareDetected = true
		}
	})

	collector.OnHTML("body", func(e *colly.HTMLElement) {
		info = &DownloadInfo{}

		e.ForEach("a[href*='download'], .download-btn a", func(_ int, el *colly.HTMLElement) {
			href := el.Attr("href")
			if href != "" && !strings.Contains(href, "javascript") {
				if !strings.HasPrefix(href, "http") {
					href = fmt.Sprintf("https://%s%s", c.baseURL, href)
				}
				if info.DirectURL == "" {
					info.DirectURL = href
				}
				info.MirrorURLs = append(info.MirrorURLs, href)
			}
		})

		e.ForEach("a[href*='.pdf'], a[href*='.epub'], a[href*='.mobi']", func(_ int, el *colly.HTMLElement) {
			href := el.Attr("href")
			if href != "" && strings.HasPrefix(href, "http") {
				info.MirrorURLs = append(info.MirrorURLs, href)
			}
		})

		if info.DirectURL == "" && len(info.MirrorURLs) > 0 {
			info.DirectURL = info.MirrorURLs[0]
		}
	})

	pageURL := fmt.Sprintf("https://%s/book/%s", c.baseURL, md5Hash)
	err := collector.Visit(pageURL)
	if err != nil {
		return c.browser.GetDownloadInfo(ctx, md5Hash)
	}

	collector.Wait()

	if cloudflareDetected {
		return c.browser.GetDownloadInfo(ctx, md5Hash)
	}

	if info == nil || (info.DirectURL == "" && len(info.MirrorURLs) == 0) {
		return c.browser.GetDownloadInfo(ctx, md5Hash)
	}

	return info, nil
}
