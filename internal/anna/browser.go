package anna

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/chromedp/chromedp"
)

// silentLogger discards all log output
var silentLogger = log.New(io.Discard, "", 0)

// BrowserClient uses a headless browser to access Anna's Archive
// This is used as a fallback when Cloudflare blocks regular HTTP requests
type BrowserClient struct {
	baseURL string
}

// NewBrowserClient creates a new browser client
func NewBrowserClient(baseURL string) *BrowserClient {
	if baseURL == "" {
		baseURL = "annas-archive.li"
	}
	return &BrowserClient{baseURL: baseURL}
}

// Search searches for books using a headless browser
func (c *BrowserClient) Search(ctx context.Context, query string, limit int) ([]*Book, error) {
	return c.SearchPage(ctx, query, limit, 1)
}

// SearchPage searches for books with pagination using a headless browser
func (c *BrowserClient) SearchPage(ctx context.Context, query string, limit int, page int) ([]*Book, error) {
	// Create browser context with options
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", true),
		chromedp.Flag("disable-gpu", true),
		chromedp.Flag("no-sandbox", true),
		chromedp.Flag("disable-dev-shm-usage", true),
		chromedp.Flag("disable-background-networking", true),
		chromedp.Flag("disable-extensions", true),
		chromedp.UserAgent("Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"),
	)

	allocCtx, allocCancel := chromedp.NewExecAllocator(ctx, opts...)
	defer allocCancel()

	// Suppress chromedp debug/error logs
	browserCtx, browserCancel := chromedp.NewContext(allocCtx,
		chromedp.WithLogf(silentLogger.Printf),
		chromedp.WithErrorf(silentLogger.Printf),
	)
	defer browserCancel()

	// Set timeout
	browserCtx, timeoutCancel := context.WithTimeout(browserCtx, 60*time.Second)
	defer timeoutCancel()

	// Build search URL with pagination
	searchURL := fmt.Sprintf("https://%s/search?q=%s", c.baseURL, url.QueryEscape(query))
	if page > 1 {
		searchURL = fmt.Sprintf("%s&page=%d", searchURL, page)
	}

	var htmlContent string
	err := chromedp.Run(browserCtx,
		chromedp.Navigate(searchURL),
		// Wait for page to load (Cloudflare challenge should resolve)
		chromedp.Sleep(5*time.Second),
		// Wait for search results to appear
		chromedp.WaitVisible("a[href*='/md5/']", chromedp.ByQuery),
		chromedp.OuterHTML("html", &htmlContent),
	)
	if err != nil {
		// Try waiting longer if initial wait fails
		err = chromedp.Run(browserCtx,
			chromedp.Sleep(10*time.Second),
			chromedp.OuterHTML("html", &htmlContent),
		)
		if err != nil {
			return nil, fmt.Errorf("browser search failed: %w", err)
		}
	}

	return parseSearchResultsHTML(htmlContent, limit, c.baseURL)
}

// GetDownloadInfo retrieves download links using a headless browser
func (c *BrowserClient) GetDownloadInfo(ctx context.Context, md5Hash string) (*DownloadInfo, error) {
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", true),
		chromedp.Flag("disable-gpu", true),
		chromedp.Flag("no-sandbox", true),
		chromedp.Flag("disable-dev-shm-usage", true),
		chromedp.Flag("disable-background-networking", true),
		chromedp.Flag("disable-extensions", true),
		chromedp.UserAgent("Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"),
	)

	allocCtx, allocCancel := chromedp.NewExecAllocator(ctx, opts...)
	defer allocCancel()

	// Suppress chromedp debug/error logs
	browserCtx, browserCancel := chromedp.NewContext(allocCtx,
		chromedp.WithLogf(silentLogger.Printf),
		chromedp.WithErrorf(silentLogger.Printf),
	)
	defer browserCancel()

	browserCtx, timeoutCancel := context.WithTimeout(browserCtx, 60*time.Second)
	defer timeoutCancel()

	pageURL := fmt.Sprintf("https://%s/md5/%s", c.baseURL, md5Hash)

	var htmlContent string
	err := chromedp.Run(browserCtx,
		chromedp.Navigate(pageURL),
		chromedp.Sleep(5*time.Second),
		chromedp.OuterHTML("html", &htmlContent),
	)
	if err != nil {
		return nil, fmt.Errorf("browser page load failed: %w", err)
	}

	return parseDownloadPageHTML(htmlContent, c.baseURL)
}

// ResolveDownloadURL navigates to a slow_download page and extracts the actual download URL
func (c *BrowserClient) ResolveDownloadURL(ctx context.Context, slowDownloadURL string) (string, error) {
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", true),
		chromedp.Flag("disable-gpu", true),
		chromedp.Flag("no-sandbox", true),
		chromedp.Flag("disable-dev-shm-usage", true),
		chromedp.Flag("disable-background-networking", true),
		chromedp.Flag("disable-extensions", true),
		chromedp.UserAgent("Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"),
	)

	allocCtx, allocCancel := chromedp.NewExecAllocator(ctx, opts...)
	defer allocCancel()

	browserCtx, browserCancel := chromedp.NewContext(allocCtx,
		chromedp.WithLogf(silentLogger.Printf),
		chromedp.WithErrorf(silentLogger.Printf),
	)
	defer browserCancel()

	browserCtx, timeoutCancel := context.WithTimeout(browserCtx, 180*time.Second)
	defer timeoutCancel()

	var htmlContent string
	var downloadURL string

	// Navigate to slow_download page and wait for download link to appear
	err := chromedp.Run(browserCtx,
		chromedp.Navigate(slowDownloadURL),
		// Wait for anti-bot challenge to resolve (longer wait for Cloudflare)
		chromedp.Sleep(8*time.Second),
	)
	if err != nil {
		return "", fmt.Errorf("browser navigation failed: %w", err)
	}

	// Poll for the download link to appear (countdown timer varies, can be up to 60 seconds)
	// Poll every 3 seconds for up to 2 minutes
	for i := 0; i < 40; i++ {
		err = chromedp.Run(browserCtx,
			chromedp.OuterHTML("html", &htmlContent),
		)
		if err != nil {
			return "", fmt.Errorf("failed to get page content: %w", err)
		}

		downloadURL = extractDownloadURL(htmlContent, c.baseURL)
		if downloadURL != "" {
			break
		}

		// Check if there's a countdown timer - if so, we should wait
		if strings.Contains(htmlContent, "Please wait") ||
			strings.Contains(htmlContent, "countdown") ||
			strings.Contains(htmlContent, "seconds") {
			// Wait 3 seconds before checking again
			err = chromedp.Run(browserCtx, chromedp.Sleep(3*time.Second))
			if err != nil {
				return "", err
			}
			continue
		}

		// Check if page shows an error or no files available
		if strings.Contains(htmlContent, "No files available") ||
			strings.Contains(htmlContent, "File not found") ||
			strings.Contains(htmlContent, "error") {
			break
		}

		// Wait before checking again
		err = chromedp.Run(browserCtx, chromedp.Sleep(3*time.Second))
		if err != nil {
			return "", err
		}
	}

	if downloadURL == "" {
		return "", fmt.Errorf("could not find download URL after waiting")
	}

	return downloadURL, nil
}

// extractDownloadURL parses HTML and finds the best download URL
func extractDownloadURL(html string, baseURL string) string {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return ""
	}

	var downloadURL string
	var fallbackURL string

	// IPFS gateway patterns (comprehensive list)
	ipfsGateways := []string{
		"ipfs.io", "dweb.link", "cloudflare-ipfs", "gateway.pinata", "w3s.link",
		"ipfs.eth", "cf-ipfs", "gateway.ipfs", "ipfs.fleek", "ipfs.infura",
		"nftstorage.link", "4everland.io", "ipfs-gateway", "hardbin.com",
	}

	// Other trusted download sources
	trustedSources := []string{
		"libgen.li", "libgen.is", "libgen.rs", "libgen.st", "library.lol",
		"z-lib", "zlibrary", "b-ok", "bookfi", "sci-hub",
		"annas-archive", "anna-archive",
	}

	// File extensions we're interested in
	fileExtensions := []string{".pdf", ".epub", ".mobi", ".azw3", ".djvu", ".fb2", ".cbr", ".cbz"}

	// Look for all links and categorize them
	doc.Find("a").Each(func(_ int, s *goquery.Selection) {
		if downloadURL != "" {
			return
		}
		href, exists := s.Attr("href")
		if !exists || href == "" || strings.Contains(href, "javascript") {
			return
		}

		hrefLower := strings.ToLower(href)

		// Skip internal Anna's Archive navigation links
		if strings.Contains(hrefLower, "/slow_download/") ||
			strings.Contains(hrefLower, "/fast_download/") ||
			strings.Contains(hrefLower, "/account/") ||
			strings.Contains(hrefLower, "/md5/") {
			return
		}

		// Priority 1: IPFS gateways (actual file downloads)
		for _, gateway := range ipfsGateways {
			if strings.Contains(hrefLower, gateway) {
				downloadURL = href
				return
			}
		}

		// Priority 2: Direct file links with known extensions
		for _, ext := range fileExtensions {
			if strings.HasSuffix(hrefLower, ext) && strings.HasPrefix(href, "http") {
				downloadURL = href
				return
			}
		}

		// Priority 3: Trusted download sources (file.php, get endpoints)
		for _, source := range trustedSources {
			if strings.Contains(hrefLower, source) {
				if strings.Contains(hrefLower, "/file.php") ||
					strings.Contains(hrefLower, "/get/") ||
					strings.Contains(hrefLower, "/main/") ||
					strings.Contains(hrefLower, "/download/") {
					if fallbackURL == "" {
						fallbackURL = href
					}
				}
			}
		}
	})

	// If no direct download found, look for download buttons by text
	if downloadURL == "" {
		doc.Find("a").Each(func(_ int, s *goquery.Selection) {
			if downloadURL != "" {
				return
			}
			text := strings.ToLower(s.Text())
			href, exists := s.Attr("href")
			if !exists || href == "" || strings.Contains(href, "javascript") {
				return
			}

			hrefLower := strings.ToLower(href)

			// Skip internal links
			if strings.Contains(hrefLower, "/slow_download/") ||
				strings.Contains(hrefLower, "/fast_download/") ||
				strings.Contains(hrefLower, "/account/") {
				return
			}

			// Look for explicit download links/buttons
			if (strings.Contains(text, "download") ||
				strings.Contains(text, "get file") ||
				strings.Contains(text, "mirror")) &&
				strings.HasPrefix(href, "http") {
				downloadURL = href
			}
		})
	}

	// Use fallback if no direct download found
	if downloadURL == "" && fallbackURL != "" {
		downloadURL = fallbackURL
	}

	// Make URL absolute if needed
	if downloadURL != "" && !strings.HasPrefix(downloadURL, "http") {
		if strings.HasPrefix(downloadURL, "/") {
			downloadURL = fmt.Sprintf("https://%s%s", baseURL, downloadURL)
		}
	}

	return downloadURL
}

// parseSearchResultsHTML parses search results from HTML content
func parseSearchResultsHTML(html string, limit int, baseURL string) ([]*Book, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return nil, err
	}

	var books []*Book
	md5Regex := regexp.MustCompile(`/md5/([a-fA-F0-9]{32})`)
	seenMD5 := make(map[string]bool)

	// Use js-vim-focus class to match only title links, not cover images
	doc.Find("a.js-vim-focus[href*='/md5/']").Each(func(i int, s *goquery.Selection) {
		if len(books) >= limit {
			return
		}

		href, _ := s.Attr("href")
		matches := md5Regex.FindStringSubmatch(href)
		if len(matches) < 2 {
			return
		}

		book := &Book{
			MD5Hash: strings.ToLower(matches[1]),
			PageURL: fmt.Sprintf("https://%s/md5/%s", baseURL, matches[1]),
		}

		// Extract title
		if title := s.Find("h3").First().Text(); title != "" {
			book.Title = strings.TrimSpace(title)
		} else {
			book.Title = strings.TrimSpace(s.Text())
		}

		// Clean up title - limit length and remove emojis
		if len(book.Title) > 200 {
			book.Title = book.Title[:200] + "..."
		}

		// Extract metadata from parent/sibling elements (format, size, language are in gray text divs)
		metaText := ""
		parent := s.Parent()
		if parent.Length() > 0 {
			// Look in parent and siblings for metadata
			parent.Find("div.text-gray-800, div.text-sm, div.truncate").Each(func(_ int, el *goquery.Selection) {
				metaText += " " + el.Text()
			})
			// Also check grandparent
			grandparent := parent.Parent()
			if grandparent.Length() > 0 {
				grandparent.Find("div.text-gray-800, div.text-xs").Each(func(_ int, el *goquery.Selection) {
					metaText += " " + el.Text()
				})
			}
		}
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

		if book.Title != "" && book.MD5Hash != "" && !seenMD5[book.MD5Hash] {
			seenMD5[book.MD5Hash] = true
			books = append(books, book)
		}
	})

	if len(books) == 0 {
		return nil, ErrNoResults
	}

	return books, nil
}

// parseDownloadPageHTML parses download links from the book page HTML
func parseDownloadPageHTML(html string, baseURL string) (*DownloadInfo, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return nil, err
	}

	info := &DownloadInfo{}

	// First priority: Direct external download links (LibGen file.php, library.lol/main, etc.)
	doc.Find("a[href*='libgen.li/file.php'], a[href*='library.lol/main'], a[href*='libgen.is/get'], a[href*='libgen.rs/get']").Each(func(_ int, s *goquery.Selection) {
		href, exists := s.Attr("href")
		if exists && href != "" {
			if info.DirectURL == "" {
				info.DirectURL = href
			}
			info.MirrorURLs = append(info.MirrorURLs, href)
		}
	})

	// Find other download links
	doc.Find("a").Each(func(_ int, s *goquery.Selection) {
		href, exists := s.Attr("href")
		if !exists || href == "" {
			return
		}

		hrefLower := strings.ToLower(href)

		// Skip navigation/account links and ads
		if strings.Contains(hrefLower, "/account/") ||
			strings.Contains(hrefLower, "ads.php") ||
			strings.Contains(hrefLower, "member_codes") {
			return
		}

		// Check for actual download links
		isFastDownload := strings.Contains(hrefLower, "/fast_download/")
		isSlowDownload := strings.Contains(hrefLower, "/slow_download/")
		isLibgen := strings.Contains(hrefLower, "libgen") && strings.Contains(hrefLower, "/file.php")
		isLibraryLol := strings.Contains(hrefLower, "library.lol")

		if (isFastDownload || isSlowDownload || isLibgen || isLibraryLol) && !strings.Contains(href, "javascript") {
			// Make absolute URL if needed
			if !strings.HasPrefix(href, "http") {
				if strings.HasPrefix(href, "/") {
					href = fmt.Sprintf("https://%s%s", baseURL, href)
				}
			}

			// Skip if already in mirrors
			for _, u := range info.MirrorURLs {
				if u == href {
					return
				}
			}

			// Prefer direct external links
			if (isLibgen || isLibraryLol) && info.DirectURL == "" {
				info.DirectURL = href
			}
			info.MirrorURLs = append(info.MirrorURLs, href)
		}
	})

	// If no direct URL found, use first mirror
	if info.DirectURL == "" && len(info.MirrorURLs) > 0 {
		info.DirectURL = info.MirrorURLs[0]
	}

	// Try to extract filename from page
	doc.Find("h1, h2, .title").Each(func(_ int, s *goquery.Selection) {
		if info.Filename == "" {
			info.Filename = strings.TrimSpace(s.Text())
		}
	})

	if info.DirectURL == "" && len(info.MirrorURLs) == 0 {
		return nil, fmt.Errorf("no download links found")
	}

	return info, nil
}
