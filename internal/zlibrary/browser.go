package zlibrary

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/billmal071/bookdl/internal/config"
	"github.com/chromedp/chromedp"
)

// silentLogger discards all log output
var silentLogger = log.New(io.Discard, "", 0)

// browserPool manages a shared browser instance for reuse
type browserPool struct {
	mu          sync.Mutex
	allocCtx    context.Context
	allocCancel context.CancelFunc
	browserCtx  context.Context
	cancelFunc  context.CancelFunc
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

// jsBookData represents book data extracted via JavaScript from Z-Library's
// <z-bookcard> custom elements. All metadata lives in element attributes and
// slotted children (not in shadow DOM internals).
type jsBookData struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Author   string `json:"author"`
	Year     string `json:"year"`
	Language string `json:"language"`
	Format   string `json:"format"`
	Size     string `json:"size"`
	PageURL  string `json:"pageUrl"`
	Download string `json:"download"`
}

// extractBooksJS is the JavaScript executed inside the browser to pull book data
// from Z-Library's <z-bookcard> custom elements. The data is stored in element
// attributes (id, href, language, year, extension, filesize) and slotted divs
// (slot="title", slot="author").
const extractBooksJS = `
(function() {
	var cards = document.querySelectorAll('z-bookcard');
	if (cards.length === 0) return [];

	return Array.from(cards).map(function(card) {
		var titleEl = card.querySelector('[slot="title"]');
		var authorEl = card.querySelector('[slot="author"]');
		return {
			id: card.getAttribute('id') || '',
			title: titleEl ? titleEl.textContent.trim() : '',
			author: authorEl ? authorEl.textContent.trim() : '',
			year: card.getAttribute('year') || '',
			language: card.getAttribute('language') || '',
			format: card.getAttribute('extension') || '',
			size: card.getAttribute('filesize') || '',
			pageUrl: card.getAttribute('href') || '',
			download: card.getAttribute('download') || ''
		};
	}).filter(function(b) { return b.id && b.title; });
})()
`

// SearchPage searches for books with pagination using a headless browser.
// Z-Library renders search results via <z-bookcard> custom elements whose
// metadata is stored in HTML attributes and slotted children. We extract
// the data with JavaScript after the page has rendered.
func (c *BrowserClient) SearchPage(ctx context.Context, query string, limit int, page int) ([]*Book, error) {
	browserCtx, cancel, err := sharedBrowserPool.getBrowserContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get browser context: %w", err)
	}
	defer cancel()

	browserCtx, timeoutCancel := context.WithTimeout(browserCtx, 90*time.Second)
	defer timeoutCancel()

	// Use PathEscape (not QueryEscape) because the query is in the URL path segment,
	// not in a query parameter. QueryEscape encodes spaces as '+' which Z-Library
	// does not interpret correctly in /s/<query> paths.
	searchURL := fmt.Sprintf("https://%s/s/%s", c.baseURL, url.PathEscape(query))
	if page > 1 {
		searchURL = fmt.Sprintf("%s?page=%d", searchURL, page)
	}

	// Navigate and wait for initial page load
	err = chromedp.Run(browserCtx,
		chromedp.Navigate(searchURL),
		chromedp.Sleep(3*time.Second),
		chromedp.WaitVisible(`body`, chromedp.ByQuery),
	)
	if err != nil {
		return nil, fmt.Errorf("browser navigation failed: %w", err)
	}

	// Poll for z-bookcard elements to appear in the DOM.
	var jsResultRaw any
	pollInterval := 2 * time.Second
	maxAttempts := 15

	for attempt := 0; attempt < maxAttempts; attempt++ {
		select {
		case <-browserCtx.Done():
			return nil, fmt.Errorf("browser timeout waiting for results")
		default:
		}

		err = chromedp.Run(browserCtx,
			chromedp.Evaluate(extractBooksJS, &jsResultRaw),
		)
		if err != nil {
			return nil, fmt.Errorf("failed to evaluate JavaScript: %w", err)
		}

		if arr, ok := jsResultRaw.([]any); ok && len(arr) > 0 {
			break
		}

		// Check for Cloudflare challenge
		var pageTitle string
		_ = chromedp.Run(browserCtx, chromedp.Title(&pageTitle))
		if strings.Contains(pageTitle, "Just a moment") {
			err = chromedp.Run(browserCtx, chromedp.Sleep(5*time.Second))
			if err != nil {
				return nil, fmt.Errorf("polling interrupted: %w", err)
			}
			continue
		}

		err = chromedp.Run(browserCtx, chromedp.Sleep(pollInterval))
		if err != nil {
			return nil, fmt.Errorf("polling interrupted: %w", err)
		}
	}

	books, err := jsResultToBooks(jsResultRaw, limit, c.baseURL)
	if err != nil {
		// Fallback: parse outer HTML in case Z-Library changed its rendering
		var htmlContent string
		_ = chromedp.Run(browserCtx, chromedp.OuterHTML("html", &htmlContent))
		if htmlContent != "" {
			return parseSearchResultsHTML(htmlContent, limit, c.baseURL)
		}
		return nil, err
	}

	return books, nil
}

// GetDownloadInfo retrieves download links using a headless browser.
// Z-Library requires login to show download links on book detail pages,
// so we search by book ID to find the z-bookcard element which exposes
// the download attribute without authentication.
func (c *BrowserClient) GetDownloadInfo(ctx context.Context, md5Hash string) (*DownloadInfo, error) {
	// Try search-based extraction first (works without login)
	info, err := c.getDownloadInfoViaSearch(ctx, md5Hash)
	if err == nil && info != nil && info.DirectURL != "" {
		return info, nil
	}

	// Fallback: try the book detail page (may work if user has session cookies)
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

// getDownloadInfoViaSearch extracts the download URL for a specific book ID
// by searching on Z-Library and finding the matching z-bookcard element.
// Z-Library's search doesn't work with numeric IDs directly, so we navigate
// to the search page and use JavaScript to find the z-bookcard with the
// matching ID attribute.
func (c *BrowserClient) getDownloadInfoViaSearch(ctx context.Context, md5Hash string) (*DownloadInfo, error) {
	browserCtx, cancel, err := sharedBrowserPool.getBrowserContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get browser context: %w", err)
	}
	defer cancel()

	browserCtx, timeoutCancel := context.WithTimeout(browserCtx, 60*time.Second)
	defer timeoutCancel()

	// Navigate to the search page with the book ID as query
	searchURL := fmt.Sprintf("https://%s/s/%s", c.baseURL, url.PathEscape(md5Hash))

	err = chromedp.Run(browserCtx,
		chromedp.Navigate(searchURL),
		chromedp.Sleep(3*time.Second),
		chromedp.WaitVisible(`body`, chromedp.ByQuery),
	)
	if err != nil {
		return nil, fmt.Errorf("browser navigation failed: %w", err)
	}

	// Wait for z-bookcard elements and find the one with matching ID
	const maxPoll = 8
	var downloadURL string
	targetID := md5Hash

	for i := 0; i < maxPoll; i++ {
		err = chromedp.Run(browserCtx, chromedp.Evaluate(fmt.Sprintf(`
			(function() {
				var cards = document.querySelectorAll('z-bookcard');
				// First try exact ID match
				for (var i = 0; i < cards.length; i++) {
					if (cards[i].getAttribute('id') === '%s') {
						var dl = cards[i].getAttribute('download');
						if (dl && dl.trim() !== '') return dl.trim();
					}
				}
				// If no exact match, return first card with a download attribute
				for (var i = 0; i < cards.length; i++) {
					var dl = cards[i].getAttribute('download');
					if (dl && dl.trim() !== '') return dl.trim();
				}
				return '';
			})()
		`, targetID), &downloadURL))
		if err != nil {
			return nil, fmt.Errorf("failed to extract download URL: %w", err)
		}
		if downloadURL != "" {
			break
		}
		if err := chromedp.Run(browserCtx, chromedp.Sleep(2*time.Second)); err != nil {
			return nil, err
		}
	}

	if downloadURL == "" {
		return nil, fmt.Errorf("no download URL found in search results")
	}

	// Make absolute if relative
	if !strings.HasPrefix(downloadURL, "http") {
		downloadURL = fmt.Sprintf("https://%s%s", c.baseURL, downloadURL)
	}

	return &DownloadInfo{
		DirectURL:  downloadURL,
		MirrorURLs: []string{downloadURL},
	}, nil
}

// ResolveDownloadURL navigates to a Z-Library download URL (e.g. /dl/...)
// using the browser and captures the actual file download URL via JS interception.
func (c *BrowserClient) ResolveDownloadURL(ctx context.Context, downloadPageURL string) (string, error) {
	browserCtx, cancel, err := sharedBrowserPool.getBrowserContext(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get browser context: %w", err)
	}
	defer cancel()

	browserCtx, timeoutCancel := context.WithTimeout(browserCtx, 90*time.Second)
	defer timeoutCancel()

	// Navigate to the download URL. Z-Library /dl/ endpoints typically redirect
	// to the actual file or show a download page with a link.
	err = chromedp.Run(browserCtx,
		chromedp.Navigate(downloadPageURL),
		chromedp.Sleep(5*time.Second),
	)
	if err != nil {
		return "", fmt.Errorf("browser navigation failed: %w", err)
	}

	// Check where we ended up — the browser may have redirected to the file
	var currentURL string
	err = chromedp.Run(browserCtx, chromedp.Location(&currentURL))
	if err != nil {
		return "", fmt.Errorf("failed to get current URL: %w", err)
	}

	// If we redirected to a direct file URL, use it
	lowerURL := strings.ToLower(currentURL)
	for _, ext := range []string{".pdf", ".epub", ".mobi", ".azw3", ".djvu", ".fb2"} {
		if strings.Contains(lowerURL, ext) {
			return currentURL, nil
		}
	}

	// Otherwise, try to extract a download link from the page
	var downloadURL string
	err = chromedp.Run(browserCtx, chromedp.Evaluate(`
		(function() {
			// Look for direct download links
			var selectors = [
				'a[href*="/dl/"]',
				'a[href*="download"]',
				'.download-btn a',
				'a.btn-download',
				'a[href$=".pdf"]',
				'a[href$=".epub"]',
				'a[href$=".mobi"]'
			];
			for (var i = 0; i < selectors.length; i++) {
				var el = document.querySelector(selectors[i]);
				if (el && el.href && !el.href.includes('javascript')) {
					return el.href;
				}
			}
			return '';
		})()
	`, &downloadURL))
	if err != nil {
		return "", fmt.Errorf("failed to extract download URL: %w", err)
	}

	if downloadURL == "" {
		// Final attempt: wait longer and check again (some pages have countdowns)
		err = chromedp.Run(browserCtx,
			chromedp.Sleep(10*time.Second),
		)
		if err != nil {
			return "", fmt.Errorf("polling interrupted: %w", err)
		}

		var htmlContent string
		err = chromedp.Run(browserCtx, chromedp.OuterHTML("html", &htmlContent))
		if err != nil {
			return "", fmt.Errorf("failed to get page content: %w", err)
		}

		doc, err := goquery.NewDocumentFromReader(strings.NewReader(htmlContent))
		if err != nil {
			return "", fmt.Errorf("failed to parse HTML: %w", err)
		}

		doc.Find("a[href]").Each(func(_ int, s *goquery.Selection) {
			if downloadURL != "" {
				return
			}
			href, exists := s.Attr("href")
			if !exists || href == "" {
				return
			}
			hrefLower := strings.ToLower(href)
			for _, ext := range []string{".pdf", ".epub", ".mobi", ".azw3", ".djvu"} {
				if strings.Contains(hrefLower, ext) && strings.HasPrefix(href, "http") {
					downloadURL = href
					return
				}
			}
		})
	}

	if downloadURL == "" {
		return "", fmt.Errorf("no download URL found on page")
	}

	return downloadURL, nil
}

// bookIDRe matches Z-Library book URLs like /book/12345.
var bookIDRe = regexp.MustCompile(`/book/(\d+)`)

// jsResultToBooks converts the raw JavaScript evaluation result into Book structs.
func jsResultToBooks(raw any, limit int, baseURL string) ([]*Book, error) {
	jsonBytes, err := json.Marshal(raw)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal JS result: %w", err)
	}

	var jsBooks []jsBookData
	if err := json.Unmarshal(jsonBytes, &jsBooks); err != nil {
		return nil, fmt.Errorf("failed to parse JS book data: %w", err)
	}

	if len(jsBooks) == 0 {
		return nil, ErrNoResults
	}

	seen := make(map[string]bool)
	var books []*Book

	for _, jb := range jsBooks {
		if len(books) >= limit {
			break
		}

		id := strings.TrimSpace(jb.ID)
		if id == "" {
			continue
		}
		if seen[id] {
			continue
		}
		seen[id] = true

		pageURL := strings.TrimSpace(jb.PageURL)
		if pageURL != "" && !strings.HasPrefix(pageURL, "http") {
			pageURL = fmt.Sprintf("https://%s%s", baseURL, pageURL)
		}
		if pageURL == "" {
			pageURL = fmt.Sprintf("https://%s/book/%s", baseURL, id)
		}

		title := strings.TrimSpace(jb.Title)
		if title == "" {
			continue
		}
		if runes := []rune(title); len(runes) > 200 {
			title = string(runes[:197]) + "..."
		}

		dlPath := strings.TrimSpace(jb.Download)
		var downloadURL string
		if dlPath != "" {
			downloadURL = fmt.Sprintf("https://%s%s", baseURL, dlPath)
		}

		books = append(books, &Book{
			MD5Hash:     id,
			Title:       title,
			Authors:     strings.TrimSpace(jb.Author),
			Year:        strings.TrimSpace(jb.Year),
			Language:    strings.TrimSpace(jb.Language),
			Format:      strings.ToUpper(strings.TrimSpace(jb.Format)),
			Size:        strings.TrimSpace(jb.Size),
			PageURL:     pageURL,
			DownloadURL: downloadURL,
			Source:      "zlibrary",
		})
	}

	if len(books) == 0 {
		return nil, ErrNoResults
	}

	return books, nil
}

// parseSearchResultsHTML is a fallback parser for when Z-Library renders without shadow DOM.
func parseSearchResultsHTML(html string, limit int, baseURL string) ([]*Book, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return nil, err
	}

	var books []*Book
	seen := make(map[string]bool)

	selectors := []string{
		"[data-id]",
		".book-item",
		".book-card",
		".resItem",
		"article",
	}

	for _, selector := range selectors {
		doc.Find(selector).Each(func(_ int, s *goquery.Selection) {
			if len(books) >= limit {
				return
			}
			book := parseBookElementZ(s, baseURL)
			if book != nil && book.MD5Hash != "" && !seen[book.MD5Hash] {
				seen[book.MD5Hash] = true
				books = append(books, book)
			}
		})
		if len(books) > 0 {
			break
		}
	}

	if len(books) == 0 {
		bookIDRe := bookIDRe
		doc.Find("a[href*='/book/']").Each(func(_ int, s *goquery.Selection) {
			if len(books) >= limit {
				return
			}
			href, exists := s.Attr("href")
			if !exists {
				return
			}
			matches := bookIDRe.FindStringSubmatch(href)
			if len(matches) < 2 {
				return
			}
			id := matches[1]
			if seen[id] {
				return
			}
			title := strings.TrimSpace(s.Text())
			if title == "" || len(title) < 3 {
				return
			}
			seen[id] = true
			pageURL := href
			if !strings.HasPrefix(pageURL, "http") {
				pageURL = fmt.Sprintf("https://%s%s", baseURL, href)
			}
			books = append(books, &Book{
				MD5Hash: id,
				Title:   title,
				PageURL: pageURL,
				Source:  "zlibrary",
			})
		})
	}

	if len(books) == 0 {
		return nil, ErrNoResults
	}

	return books, nil
}

// parseBookElementZ extracts book information from a goquery selection.
func parseBookElementZ(s *goquery.Selection, baseURL string) *Book {
	book := &Book{Source: "zlibrary"}

	if id, exists := s.Attr("data-id"); exists && id != "" {
		book.MD5Hash = id
		book.PageURL = fmt.Sprintf("https://%s/book/%s", baseURL, id)
	} else if href, exists := s.Find("a").Attr("href"); exists {
		if matches := bookIDRe.FindStringSubmatch(href); len(matches) > 1 {
			book.MD5Hash = matches[1]
			book.PageURL = fmt.Sprintf("https://%s/book/%s", baseURL, book.MD5Hash)
		}
	}

	for _, sel := range []string{"h3", "h4", ".title", ".book-title", "a"} {
		if title := s.Find(sel).First().Text(); title != "" {
			book.Title = strings.TrimSpace(title)
			break
		}
	}
	if book.Title == "" || book.MD5Hash == "" {
		return nil
	}
	if len(book.Title) > 200 {
		book.Title = book.Title[:197] + "..."
	}

	if author := s.Find(".author, .book-author").Text(); author != "" {
		book.Authors = strings.TrimSpace(author)
	}

	return book
}

// parseDownloadPageHTML parses download links from the book page HTML.
func parseDownloadPageHTML(html string, baseURL string) (*DownloadInfo, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return nil, err
	}

	info := &DownloadInfo{}

	doc.Find("a[href*='download'], .download-btn a, .btn-download").Each(func(_ int, s *goquery.Selection) {
		href, exists := s.Attr("href")
		if exists && href != "" && !strings.Contains(href, "javascript") {
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

	doc.Find("a[href*='.pdf'], a[href*='.epub'], a[href*='.mobi']").Each(func(_ int, s *goquery.Selection) {
		href, exists := s.Attr("href")
		if exists && href != "" && strings.HasPrefix(href, "http") {
			info.MirrorURLs = append(info.MirrorURLs, href)
		}
	})

	if info.DirectURL == "" && len(info.MirrorURLs) > 0 {
		info.DirectURL = info.MirrorURLs[0]
	}

	if info.DirectURL == "" && len(info.MirrorURLs) == 0 {
		return nil, fmt.Errorf("no download links found")
	}

	return info, nil
}
