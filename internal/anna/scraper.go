package anna

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/gocolly/colly/v2"
)

var (
	// ErrCloudflareBlocked indicates Cloudflare challenge detected
	ErrCloudflareBlocked = errors.New("cloudflare challenge detected")
	// ErrNoResults indicates no search results found
	ErrNoResults = errors.New("no results found")
)

// ScraperClient scrapes Anna's Archive website
type ScraperClient struct {
	baseURL string
	browser *BrowserClient
}

// NewScraperClient creates a new scraper client
func NewScraperClient(baseURL string) *ScraperClient {
	if baseURL == "" {
		baseURL = "annas-archive.li"
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

// SearchPage searches for books with pagination support
func (c *ScraperClient) SearchPage(ctx context.Context, query string, limit int, page int) ([]*Book, error) {
	var books []*Book
	var cloudflareDetected bool
	var scrapeErr error

	collector := colly.NewCollector(
		colly.AllowedDomains(c.baseURL),
		colly.UserAgent("Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"),
	)

	collector.SetRequestTimeout(30 * time.Second)

	// Detect Cloudflare challenge
	collector.OnResponse(func(r *colly.Response) {
		body := string(r.Body)
		if r.StatusCode == 403 || r.StatusCode == 503 ||
			strings.Contains(body, "cf-browser-verification") ||
			strings.Contains(body, "Just a moment...") ||
			strings.Contains(body, "_cf_chl") {
			cloudflareDetected = true
		}
	})

	// Track seen MD5s to avoid duplicates
	seenMD5 := make(map[string]bool)

	// Parse search results - look for title links with js-vim-focus class
	collector.OnHTML("a.js-vim-focus[href*='/md5/']", func(e *colly.HTMLElement) {
		if len(books) >= limit*2 { // Get extra for filtering
			return
		}

		book := parseBookElement(e, c.baseURL)
		if book != nil && book.MD5Hash != "" && !seenMD5[book.MD5Hash] {
			seenMD5[book.MD5Hash] = true
			books = append(books, book)
		}
	})

	collector.OnError(func(r *colly.Response, err error) {
		scrapeErr = err
	})

	// Build search URL with pagination
	searchURL := fmt.Sprintf("https://%s/search?q=%s", c.baseURL, url.QueryEscape(query))
	if page > 1 {
		searchURL = fmt.Sprintf("%s&page=%d", searchURL, page)
	}

	err := collector.Visit(searchURL)
	if err != nil {
		// Try browser fallback
		return c.browser.SearchPage(ctx, query, limit, page)
	}

	collector.Wait()

	if cloudflareDetected {
		// Fall back to headless browser
		return c.browser.SearchPage(ctx, query, limit, page)
	}

	if scrapeErr != nil {
		return nil, scrapeErr
	}

	if len(books) == 0 {
		return nil, ErrNoResults
	}

	// Limit results
	if len(books) > limit {
		books = books[:limit]
	}

	return books, nil
}

// GetDownloadInfo retrieves download links for a book
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

		// First priority: slow download links (these lead to IPFS downloads)
		// These are the best option for direct HTTP downloads
		e.ForEach("a[href*='/slow_download/']", func(_ int, el *colly.HTMLElement) {
			href := el.Attr("href")
			if href != "" && !strings.Contains(href, "?") {
				if !strings.HasPrefix(href, "http") {
					href = fmt.Sprintf("https://%s%s", c.baseURL, href)
				}
				if info.DirectURL == "" {
					info.DirectURL = href
				}
				info.MirrorURLs = append(info.MirrorURLs, href)
			}
		})

		// Also add slow_download links with query params as mirrors
		e.ForEach("a[href*='/slow_download/']", func(_ int, el *colly.HTMLElement) {
			href := el.Attr("href")
			if href != "" && strings.Contains(href, "?") {
				if !strings.HasPrefix(href, "http") {
					href = fmt.Sprintf("https://%s%s", c.baseURL, href)
				}
				info.MirrorURLs = append(info.MirrorURLs, href)
			}
		})

		// Second priority: fast download links (requires account, but add as mirror)
		e.ForEach("a[href*='/fast_download/']", func(_ int, el *colly.HTMLElement) {
			href := el.Attr("href")
			if href != "" && !strings.Contains(href, "javascript") {
				if !strings.HasPrefix(href, "http") {
					href = fmt.Sprintf("https://%s%s", c.baseURL, href)
				}
				info.MirrorURLs = append(info.MirrorURLs, href)
			}
		})

		// Third priority: LibGen links (add as fallback mirrors)
		e.ForEach("a[href*='libgen.li/file.php'], a[href*='library.lol']", func(_ int, el *colly.HTMLElement) {
			href := el.Attr("href")
			if href != "" {
				info.MirrorURLs = append(info.MirrorURLs, href)
			}
		})

		// If no direct URL found, use first mirror
		if info.DirectURL == "" && len(info.MirrorURLs) > 0 {
			info.DirectURL = info.MirrorURLs[0]
		}
	})

	pageURL := fmt.Sprintf("https://%s/md5/%s", c.baseURL, md5Hash)
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

// parseBookElement extracts book information from an HTML element
func parseBookElement(e *colly.HTMLElement, baseURL string) *Book {
	book := &Book{}

	// Extract MD5 hash from href
	href := e.Attr("href")
	md5Match := regexp.MustCompile(`/md5/([a-fA-F0-9]{32})`).FindStringSubmatch(href)
	if len(md5Match) < 2 {
		return nil
	}
	book.MD5Hash = strings.ToLower(md5Match[1])
	book.PageURL = fmt.Sprintf("https://%s/md5/%s", baseURL, book.MD5Hash)

	// The title is the text content of this anchor tag
	book.Title = strings.TrimSpace(e.Text)
	if book.Title == "" {
		return nil
	}

	// Limit title length
	if len(book.Title) > 200 {
		book.Title = book.Title[:197] + "..."
	}

	// Look for metadata in parent/sibling elements
	// The metadata is in a div with class "text-gray-800" that's a sibling
	parent := e.DOM.Parent()
	if parent != nil {
		// Find the metadata div (contains format, size, language info)
		metaText := ""
		parent.Find("div.text-gray-800, div.text-sm").Each(func(_ int, s *goquery.Selection) {
			metaText += " " + s.Text()
		})

		// Also check siblings
		parent.Parent().Find("div.text-gray-800").Each(func(_ int, s *goquery.Selection) {
			metaText += " " + s.Text()
		})

		metaText = strings.ToLower(metaText)

		// Format detection
		for _, format := range []string{"epub", "pdf", "mobi", "azw3", "djvu", "fb2", "cbr", "cbz"} {
			if strings.Contains(metaText, format) {
				book.Format = strings.ToUpper(format)
				break
			}
		}

		// Size detection (e.g., "5.2MB", "1.1 GB")
		if sizeMatch := regexp.MustCompile(`(\d+\.?\d*)\s*(KB|MB|GB)`).FindStringSubmatch(metaText); len(sizeMatch) > 0 {
			book.Size = sizeMatch[0]
		}

		// Language detection
		for _, lang := range []string{"english", "russian", "german", "french", "spanish", "chinese", "japanese", "portuguese", "italian"} {
			if strings.Contains(metaText, lang) {
				book.Language = strings.Title(lang)
				break
			}
		}
	}

	return book
}
