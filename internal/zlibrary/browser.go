package zlibrary

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/url"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/chromedp/chromedp"
	"github.com/billmal071/bookdl/internal/config"
)

// silentLogger discards all log output
var silentLogger = log.New(io.Discard, "", 0)

// browserPool manages a shared browser instance for reuse
type browserPool struct {
	mu sync.Mutex
	allocCtx context.Context
	allocCancel context.CancelFunc
	browserCtx context.Context
	cancelFunc context.CancelFunc
	inUse bool
}

var sharedBrowserPool = &browserPool{}

// getBrowserContext returns a reusable browser context
func (p *browserPool) getBrowserContext(parentCtx context.Context) (context.Context, context.CancelFunc, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Check if existing browser is still valid
	if p.browserCtx != nil {
		select {
		case <-p.browserCtx.Done():
			// Browser context was cancelled, need to recreate
			p.cleanup()
		default:
			// Browser is still valid, create a new tab context
			tabCtx, tabCancel := chromedp.NewContext(p.browserCtx,
				chromedp.WithLogf(silentLogger.Printf),
				chromedp.WithErrorf(silentLogger.Printf),
			)
			return tabCtx, tabCancel, nil
		}
	}

	// Create new browser instance
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", true),
		chromedp.Flag("disable-gpu", true),
		chromedp.Flag("no-sandbox", true),
		chromedp.Flag("disable-dev-shm-usage", true),
		chromedp.Flag("disable-background-networking", true),
		chromedp.Flag("disable-extensions", true),
		chromedp.Flag("disable-blink-features", "AutomationControlled"),
		chromedp.UserAgent("Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"),
		chromedp.WindowSize(1920, 1080),
	)

	p.allocCtx, p.allocCancel = chromedp.NewExecAllocator(context.Background(), opts...)
	p.browserCtx, p.cancelFunc = chromedp.NewContext(p.allocCtx,
		chromedp.WithLogf(silentLogger.Printf),
		chromedp.WithErrorf(silentLogger.Printf),
	)

	// Create a tab context for this request
	tabCtx, tabCancel := chromedp.NewContext(p.browserCtx,
		chromedp.WithLogf(silentLogger.Printf),
		chromedp.WithErrorf(silentLogger.Printf),
	)

	return tabCtx, tabCancel, nil
}

// cleanup releases browser resources
func (p *browserPool) cleanup() {
	if p.cancelFunc != nil {
		p.cancelFunc()
		p.cancelFunc = nil
	}
	if p.allocCancel != nil {
		p.allocCancel()
		p.allocCancel = nil
	}
	p.browserCtx = nil
	p.allocCtx = nil
}

// CloseBrowser closes the shared browser instance
func CloseBrowser() {
	sharedBrowserPool.mu.Lock()
	defer sharedBrowserPool.mu.Unlock()
	sharedBrowserPool.cleanup()
}

// BrowserClient uses a headless browser to access Z-Library
type BrowserClient struct {
	baseURL string
}

// NewBrowserClient creates a new browser client
func NewBrowserClient(baseURL string) *BrowserClient {
	if baseURL == "" {
		cfg := config.Get()
		baseURL = cfg.ZLibrary.BaseURL
		if baseURL == "" {
			baseURL = "z-library.sk"
		}
	}
	return &BrowserClient{baseURL: baseURL}
}

// Search searches for books using a headless browser
func (c *BrowserClient) Search(ctx context.Context, query string, limit int) ([]*Book, error) {
	return c.SearchPage(ctx, query, limit, 1)
}

// SearchPage searches for books with pagination using a headless browser
func (c *BrowserClient) SearchPage(ctx context.Context, query string, limit int, page int) ([]*Book, error) {
	// Get a browser context from the shared pool
	browserCtx, cancel, err := sharedBrowserPool.getBrowserContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get browser context: %w", err)
	}
	defer cancel()

	// Set longer timeout for Z-Library anti-bot protection
	browserCtx, timeoutCancel := context.WithTimeout(browserCtx, 180*time.Second)
	defer timeoutCancel()

	// Build search URL with pagination - use Z-Library's actual format
	searchURL := fmt.Sprintf("https://%s/s/%s", c.baseURL, url.QueryEscape(query))
	if page > 1 {
		searchURL = fmt.Sprintf("%s?page=%d", searchURL, page)
	}

	var htmlContent string
	var jsResult any

	// Navigate and wait for anti-bot challenge to complete
	err = chromedp.Run(browserCtx,
		chromedp.Navigate(searchURL),
		// Wait for the page to load
		chromedp.Sleep(5*time.Second),
		// Wait for body to be present
		chromedp.WaitVisible(`body`, chromedp.ByQuery),
		// Wait for the data to load - Z-Library uses client-side rendering
		chromedp.Sleep(15*time.Second),
		// Try to execute JavaScript to get the book data
		chromedp.Evaluate(`
			(function() {
				// Try to get book data from the z-booklist component
				const booklist = document.querySelector('z-booklist');
				if (!booklist) {
					return {error: 'z-booklist not found'};
				}
				if (!booklist.shadowRoot) {
					return {error: 'shadowRoot not found', hasBooklist: true};
				}
				const items = booklist.shadowRoot.querySelectorAll('[data-id]');
				if (items.length === 0) {
					return {error: 'no items found', hasShadowRoot: true};
				}
				return Array.from(items).map(item => ({
					id: item.getAttribute('data-id'),
					title: item.querySelector('.title')?.textContent || '',
					author: item.querySelector('.author')?.textContent || ''
				}));
			})()
		`, &jsResult),
		// Get the HTML content as well
		chromedp.OuterHTML("html", &htmlContent),
	)
	if err != nil {
		return nil, fmt.Errorf("browser navigation failed: %w", err)
	}

	// Just get the HTML content regardless of selectors
	// Z-Library might have different HTML structure
	err = chromedp.Run(browserCtx,
		chromedp.OuterHTML("html", &htmlContent),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get page content: %w", err)
	}

	// Debug: save HTML to file for inspection
	if len(htmlContent) > 0 {
		fmt.Printf("[DEBUG] Got %d bytes of HTML from Z-Library\n", len(htmlContent))
		if len(htmlContent) < 500 {
			fmt.Printf("[DEBUG] HTML preview: %s\n", htmlContent[:500])
		} else {
			fmt.Printf("[DEBUG] HTML preview: %s\n", htmlContent[:500])
		}
		// Save full HTML for debugging
		os.WriteFile("/tmp/zlibrary_debug.html", []byte(htmlContent), 0644)
	}

	// Debug: check JavaScript result
	if jsResult != nil {
		fmt.Printf("[DEBUG] JavaScript result: %v\n", jsResult)
	} else {
		fmt.Printf("[DEBUG] JavaScript result is nil\n")
	}

	// Debug: check for Cloudflare or anti-bot indicators
	if strings.Contains(htmlContent, "cf-browser-verification") ||
		strings.Contains(htmlContent, "Just a moment...") ||
		strings.Contains(htmlContent, "_cf_chl") {
		fmt.Printf("[DEBUG] Cloudflare detected in HTML\n")
	}
	if strings.Contains(htmlContent, "Please wait") ||
		strings.Contains(htmlContent, "Checking your browser") {
		fmt.Printf("[DEBUG] Anti-bot challenge detected\n")
	}

	return parseSearchResultsHTML(htmlContent, limit, c.baseURL)
}

// GetDownloadInfo retrieves download links using a headless browser
func (c *BrowserClient) GetDownloadInfo(ctx context.Context, md5Hash string) (*DownloadInfo, error) {
	// Get a browser context from the shared pool
	browserCtx, cancel, err := sharedBrowserPool.getBrowserContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get browser context: %w", err)
	}
	defer cancel()

	browserCtx, timeoutCancel := context.WithTimeout(browserCtx, 60*time.Second)
	defer timeoutCancel()

	pageURL := fmt.Sprintf("https://%s/book/%s", c.baseURL, md5Hash)

	var htmlContent string
	err = chromedp.Run(browserCtx,
		chromedp.Navigate(pageURL),
		chromedp.Sleep(5*time.Second),
		chromedp.OuterHTML("html", &htmlContent),
	)
	if err != nil {
		return nil, fmt.Errorf("browser page load failed: %w", err)
	}

	return parseDownloadPageHTML(htmlContent, c.baseURL)
}

// ResolveDownloadURL navigates to a download page and extracts the actual download URL
func (c *BrowserClient) ResolveDownloadURL(ctx context.Context, downloadPageURL string) (string, error) {
	// This method may not be needed for Z-Library as they typically provide direct download links
	// But included for compatibility with Anna's Archive structure

	browserCtx, cancel, err := sharedBrowserPool.getBrowserContext(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get browser context: %w", err)
	}
	defer cancel()

	browserCtx, timeoutCancel := context.WithTimeout(browserCtx, 90*time.Second)
	defer timeoutCancel()

	var htmlContent string
	var downloadURL string

	// Navigate to download page
	err = chromedp.Run(browserCtx,
		chromedp.Navigate(downloadPageURL),
		// Wait for anti-bot challenge
		chromedp.Sleep(8*time.Second),
	)
	if err != nil {
		return "", fmt.Errorf("browser navigation failed: %w", err)
	}

	// Try to extract direct download link
	err = chromedp.Run(browserCtx,
		chromedp.OuterHTML("html", &htmlContent),
	)
	if err != nil {
		return "", fmt.Errorf("failed to get page content: %w", err)
	}

	// Parse HTML to find download link
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(htmlContent))
	if err != nil {
		return "", fmt.Errorf("failed to parse HTML: %w", err)
	}

	// Look for download button or link
	doc.Find("a[href*='download'], .download-btn a, button[onclick*='download']").Each(func(_ int, s *goquery.Selection) {
		if downloadURL != "" {
			return
		}

		href, exists := s.Attr("href")
		if exists && href != "" && strings.HasPrefix(href, "http") {
			downloadURL = href
		}
	})

	if downloadURL == "" {
		return "", fmt.Errorf("no download URL found")
	}

	return downloadURL, nil
}

// parseSearchResultsHTML parses search results from HTML content
func parseSearchResultsHTML(html string, limit int, baseURL string) ([]*Book, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return nil, err
	}

	var books []*Book
	var seenMD5 = make(map[string]bool)

	fmt.Printf("[DEBUG] Starting to parse Z-Library HTML for books...\n")

	// Look for book items - use very generic selectors first
	// Z-Library might use different HTML structure
	selectors := []string{
		".book-item",
		".book-card",
		".resItem",
		"[class*='book']",
		".item",
		".result",
		"article",
		".search-result",
	}

	for _, selector := range selectors {
		matches := doc.Find(selector)
		fmt.Printf("[DEBUG] Selector '%s' found %d elements\n", selector, matches.Length())
		matches.Each(func(i int, s *goquery.Selection) {
			if len(books) >= limit {
				return
			}

			book := parseBookElementZ(s, baseURL)
			if book != nil && book.MD5Hash != "" && !seenMD5[book.MD5Hash] {
				seenMD5[book.MD5Hash] = true
				books = append(books, book)
				fmt.Printf("[DEBUG] Added book: %s (MD5: %s)\n", book.Title, book.MD5Hash)
			}
		})

		if len(books) > 0 {
			break // Found results with this selector
		}
	}

	// If still no results, try to find any links that look like book pages
	if len(books) == 0 {
		// Look for any links that might be book pages
		doc.Find("a").Each(func(i int, s *goquery.Selection) {
			if len(books) >= limit {
				return
			}

			href, exists := s.Attr("href")
			if !exists || href == "" {
				return
			}

			// Skip if it's clearly not a book link
			if strings.Contains(href, "javascript") ||
				strings.Contains(href, "#") ||
				strings.Contains(href, "mailto") ||
				strings.Contains(href, "tel:") {
				return
			}

			// Extract ID from href
			var md5Hash string
			if matches := regexp.MustCompile(`/book/(\d+)`).FindStringSubmatch(href); len(matches) > 1 {
				md5Hash = matches[1]
			} else if matches := regexp.MustCompile(`/md5/([a-fA-F0-9]{32})`).FindStringSubmatch(href); len(matches) > 1 {
				md5Hash = matches[1]
			} else if matches := regexp.MustCompile(`\d{5,}`).FindString(href); len(matches) > 0 {
				md5Hash = matches
			}

			if md5Hash != "" && !seenMD5[md5Hash] {
				seenMD5[md5Hash] = true
				book := &Book{
					MD5Hash:  md5Hash,
					PageURL:  fmt.Sprintf("https://%s%s", baseURL, href),
					Title:     strings.TrimSpace(s.Text()),
					Source:    "zlibrary",
				}
				if book.Title != "" && len(book.Title) > 3 {
					books = append(books, book)
				}
			}
		})
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

// parseBookElementZ extracts book information from a goquery selection (browser-html parsed)
func parseBookElementZ(s *goquery.Selection, baseURL string) *Book {
	book := &Book{}
	book.Source = "zlibrary" // Set source

	// Try to find MD5/id from various attributes
	// Look in data-id, id, or href attributes
	if id, exists := s.Attr("data-id"); exists && id != "" {
		book.MD5Hash = id
		book.PageURL = fmt.Sprintf("https://%s/book/%s", baseURL, id)
	} else if href, exists := s.Find("a").Attr("href"); exists && href != "" {
		// Extract ID from href
		if matches := regexp.MustCompile(`/book/(\d+)`).FindStringSubmatch(href); len(matches) > 1 {
			book.MD5Hash = matches[1]
			book.PageURL = fmt.Sprintf("https://%s/book/%s", baseURL, book.MD5Hash)
		}
	}

	// Extract title
	titleSelectors := []string{"h3", "h4", ".title", ".book-title", "a"}
	for _, selector := range titleSelectors {
		if title := s.Find(selector).First().Text(); title != "" {
			book.Title = strings.TrimSpace(title)
			break
		}
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

	// Look for metadata in the same container
	s.Parent().Find(".book-meta, .meta-info, .text-muted, div").Each(func(_ int, sibling *goquery.Selection) {
		metaText += " " + sibling.Text()
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

	// Author detection
	if author := s.Find(".author, .book-author").Text(); author != "" {
		book.Authors = strings.TrimSpace(author)
	}

	if book.MD5Hash != "" || book.PageURL != "" {
		return book
	}
	return nil
}

// parseDownloadPageHTML parses download links from the book page HTML
func parseDownloadPageHTML(html string, baseURL string) (*DownloadInfo, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return nil, err
	}

	info := &DownloadInfo{}

	// Look for download buttons and links
	doc.Find("a[href*='download'], .download-btn a, .btn-download").Each(func(_ int, s *goquery.Selection) {
		href, exists := s.Attr("href")
		if exists && href != "" && !strings.Contains(href, "javascript") {
			// Make absolute URL if needed
			if !strings.HasPrefix(href, "http") {
				if strings.HasPrefix(href, "/") {
					href = fmt.Sprintf("https://%s%s", baseURL, href)
				}
			}

			if info.DirectURL == "" {
				info.DirectURL = href
			}
			info.MirrorURLs = append(info.MirrorURLs, href)
		}
	})

	// Also look for direct file links
	doc.Find("a[href*='.pdf'], a[href*='.epub'], a[href*='.mobi']").Each(func(_ int, s *goquery.Selection) {
		href, exists := s.Attr("href")
		if exists && href != "" && strings.HasPrefix(href, "http") {
			info.MirrorURLs = append(info.MirrorURLs, href)
		}
	})

	// If no direct URL found, use first mirror
	if info.DirectURL == "" && len(info.MirrorURLs) > 0 {
		info.DirectURL = info.MirrorURLs[0]
	}

	if info.DirectURL == "" && len(info.MirrorURLs) == 0 {
		return nil, fmt.Errorf("no download links found")
	}

	return info, nil
}
