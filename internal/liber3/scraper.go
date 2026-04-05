package liber3

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

// ScraperClient scrapes Liber3 website
type ScraperClient struct {
	baseURL string
	browser *BrowserClient
}

// NewScraperClient creates a new scraper client
func NewScraperClient(baseURL string) *ScraperClient {
	if baseURL == "" {
		baseURL = "liber3.eth.limo"
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

	// Parse search results - look for book items
	// Liber3 might use different selectors
	collector.OnHTML(".book-item, .resItem, .book-card, [class*='book']", func(e *colly.HTMLElement) {
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

	// Build search URL with pagination - liber3 uses hash-based routing
	searchURL := fmt.Sprintf("https://%s/#/search?q=%s", c.baseURL, url.QueryEscape(query))
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
		return c.browser.SearchPage(ctx, query, limit, page)
	}

	if scrapeErr != nil {
		return nil, scrapeErr
	}

	if len(books) == 0 {
		// Fall back to browser for SPA content
		return c.browser.SearchPage(ctx, query, limit, page)
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

		// Look for download buttons and links
		e.ForEach("a[href*='download'], .download-btn a, button[onclick*='download']", func(_ int, el *colly.HTMLElement) {
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

		// Also look for direct file links
		e.ForEach("a[href*='.pdf'], a[href*='.epub'], a[href*='.mobi']", func(_ int, el *colly.HTMLElement) {
			href := el.Attr("href")
			if href != "" && strings.HasPrefix(href, "http") {
				info.MirrorURLs = append(info.MirrorURLs, href)
			}
		})

		// If no direct URL found, use first mirror
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

// parseBookElement extracts book information from an HTML element
func parseBookElement(e *colly.HTMLElement, baseURL string) *Book {
	book := &Book{}
	book.Source = "liber3" // Set source

	// Extract MD5 hash - Liber3 might use different URL patterns
	href := e.Attr("href")
	md5Match := regexp.MustCompile(`/book/(\d+)`).FindStringSubmatch(href)
	if len(md5Match) < 2 {
		// Try other patterns
		md5Match = regexp.MustCompile(`/md5/([a-fA-F0-9]{32})`).FindStringSubmatch(href)
		if len(md5Match) < 2 {
			// Try numeric ID
			hdMatch := regexp.MustCompile(`\d{5,}`).FindString(href)
			if hdMatch != "" {
				book.MD5Hash = hdMatch
			} else {
				book.PageURL = fmt.Sprintf("https://%s%s", baseURL, href)
			}
		}
	} else {
		book.MD5Hash = md5Match[1]
		book.PageURL = fmt.Sprintf("https://%s/book/%s", baseURL, book.MD5Hash)
	}

	// Extract title - look for h3, h4, or .title elements
	titleSelectors := []string{"h3", "h4", ".title", ".book-title"}
	for _, selector := range titleSelectors {
		if title := e.ChildText(selector); title != "" {
			book.Title = strings.TrimSpace(title)
			break
		}
	}

	if book.Title == "" {
		book.Title = strings.TrimSpace(e.Text)
	}

	if book.Title == "" {
		return nil
	}

	// Limit title length
	if len(book.Title) > 200 {
		book.Title = book.Title[:197] + "..."
	}

	// Extract metadata from sibling elements
	metaText := ""

	// Look for metadata in parent sibling elements
	e.DOM.Parent().Find(".book-meta, .meta-info, .text-muted").Each(func(_ int, s *goquery.Selection) {
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

	// Author detection - look in .author or after by
	if author := e.ChildText(".author, .book-author"); author != "" {
		book.Authors = strings.TrimSpace(author)
	} else {
		// Try to extract from meta text "by Author Name"
		if authorMatch := regexp.MustCompile(`\bby\s+([A-Z][a-z]+(?:\s+[A-Z][a-z]+)*)`); len(authorMatch.FindStringSubmatch(metaText)) > 1 {
			book.Authors = authorMatch.FindStringSubmatch(metaText)[1]
		}
	}

	if book.MD5Hash != "" || book.PageURL != "" {
		return book
	}
	return nil
}
