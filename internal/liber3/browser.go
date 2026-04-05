package liber3

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/chromedp/chromedp"
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

// BrowserClient uses a headless browser to access Liber3
type BrowserClient struct {
	baseURL string
}

// NewBrowserClient creates a new browser client
func NewBrowserClient(baseURL string) *BrowserClient {
	if baseURL == "" {
		baseURL = "liber3.eth.limo"
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

	// Set timeout
	browserCtx, timeoutCancel := context.WithTimeout(browserCtx, 60*time.Second)
	defer timeoutCancel()

	// Build search URL with pagination - liber3 uses hash-based routing
	searchURL := fmt.Sprintf("https://%s/#/search?q=%s", c.baseURL, url.QueryEscape(query))
	if page > 1 {
		searchURL = fmt.Sprintf("%s&page=%d", searchURL, page)
	}

	var htmlContent string
	err = chromedp.Run(browserCtx,
		chromedp.Navigate(searchURL),
		// Wait for page to load and JavaScript to render results
		chromedp.Sleep(10*time.Second),
		chromedp.OuterHTML("html", &htmlContent),
	)
	if err != nil {
		return nil, fmt.Errorf("browser search failed: %w", err)
	}

	if len(htmlContent) == 0 {
		return nil, fmt.Errorf("no HTML content received from Liber3")
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

// parseSearchResultsHTML parses search results from HTML content
func parseSearchResultsHTML(html string, limit int, baseURL string) ([]*Book, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return nil, err
	}

	var books []*Book
	var seenMD5 = make(map[string]bool)

	// Liber3 uses a specific structure:
	// <div><div style="display: flex;">
	//   <div style="flex: 1 1 0%; display: flex; flex-direction: column;">
	//     <div class="title">Book Title</div>
	//     <div class="tags">Author / Language / format / size / IPFS</div>
	//   </div>
	// </div></div>

	// Find all title divs
	doc.Find(".title").Each(func(i int, s *goquery.Selection) {
		if len(books) >= limit {
			return
		}

		title := strings.TrimSpace(s.Text())
		if title == "" {
			return
		}

		// Get the parent div that contains the tags
		parent := s.Parent()
		tagsDiv := parent.Find(".tags")
		if tagsDiv.Length() == 0 {
			return
		}

		tagsText := strings.TrimSpace(tagsDiv.Text())

		// Parse tags: "Author / Language / format / size / IPFS"
		parts := strings.Split(tagsText, "/")
		if len(parts) < 3 {
			return
		}

		book := &Book{
			Title:  title,
			Source: "liber3",
		}

		// Extract author (first part)
		if len(parts) > 0 {
			book.Authors = strings.TrimSpace(parts[0])
		}

		// Extract language, format, size from remaining parts
		for _, part := range parts[1:] {
			part = strings.TrimSpace(part)
			part = strings.ToLower(part)

			// Language detection
			for _, lang := range []string{"english", "russian", "german", "french", "spanish", "chinese", "japanese", "portuguese", "italian"} {
				if strings.Contains(part, lang) {
					book.Language = strings.Title(lang)
					break
				}
			}

			// Format detection
			for _, format := range []string{"epub", "pdf", "mobi", "azw3", "djvu", "fb2", "cbr", "cbz"} {
				if strings.Contains(part, format) {
					book.Format = strings.ToUpper(format)
					break
				}
			}

			// Size detection
			if sizeMatch := regexp.MustCompile(`(\d+\.?\d*)\s*(KB|MB|GB)`).FindStringSubmatch(part); len(sizeMatch) > 0 {
				book.Size = sizeMatch[0]
			}
		}

		// Generate a unique ID for the book (use title as hash since no MD5 is available)
		book.MD5Hash = fmt.Sprintf("%x", len(title)+len(book.Authors))
		book.PageURL = fmt.Sprintf("https://%s/#/search?q=%s", baseURL, url.QueryEscape(title))

		if book.MD5Hash != "" && !seenMD5[book.MD5Hash] {
			seenMD5[book.MD5Hash] = true
			books = append(books, book)
		}
	})

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
	book.Source = "liber3" // Set source

	// Try to find MD5/id from various attributes
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
	titleSelectors := []string{"h3", "h4", ".title", ".book-title"}
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
