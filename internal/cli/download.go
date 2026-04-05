package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/billmal071/bookdl/internal/anna"
	"github.com/billmal071/bookdl/internal/config"
	"github.com/billmal071/bookdl/internal/db"
	"github.com/billmal071/bookdl/internal/downloader"
	"github.com/billmal071/bookdl/internal/notify"
	"github.com/billmal071/bookdl/internal/liber3"
	"github.com/billmal071/bookdl/internal/zlibrary"
)

var downloadCmd = &cobra.Command{
	Use:   "download [book-id]",
	Short: "Download a book by its ID or MD5 hash",
	Long: `Download a book using its ID or MD5 hash.

The ID can be obtained from the search results. Use --source to specify
which source the ID belongs to (auto-detected if not specified).

Examples:
  bookdl download abc123def456789...                    # Anna's Archive (32-char MD5)
  bookdl download 115469056                             # auto-detects Z-Library/Liber3
  bookdl download 115469056 --source zlibrary           # explicit source
  bookdl download -o ~/Books abc123def456789...`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		outputDir, _ := cmd.Flags().GetString("output")
		source, _ := cmd.Flags().GetString("source")
		return runDownloadByHash(cmd.Context(), args[0], outputDir, nil, source)
	},
}

func init() {
	downloadCmd.Flags().StringP("output", "o", "", "output directory (default: ~/Downloads/books)")
	downloadCmd.Flags().String("source", "", "book source: anna, zlibrary, or liber3 (auto-detected if not specified)")
}

// detectSource guesses the source from the book ID format.
// 32-char hex strings are Anna's Archive MD5 hashes.
// Numeric IDs could be Z-Library or Liber3 — requires --source flag.
func detectSource(id string) (string, error) {
	if len(id) == 32 {
		isHex := true
		for _, c := range id {
			if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
				isHex = false
				break
			}
		}
		if isHex {
			return "anna", nil
		}
	}
	// Check if numeric
	isNumeric := true
	for _, c := range id {
		if c < '0' || c > '9' {
			isNumeric = false
			break
		}
	}
	if isNumeric {
		return "", fmt.Errorf("numeric ID detected — use --source to specify zlibrary or liber3")
	}
	return "anna", nil
}

// getDownloadInfoForSource fetches download info from the appropriate source client.
func getDownloadInfoForSource(ctx context.Context, bookID string, source string) (*anna.DownloadInfo, error) {
	switch source {
	case "zlibrary":
		zClient := zlibrary.NewClient()
		zInfo, err := zClient.GetDownloadInfo(ctx, bookID)
		if err != nil {
			return nil, err
		}
		return &anna.DownloadInfo{
			DirectURL:  zInfo.DirectURL,
			MirrorURLs: zInfo.MirrorURLs,
			Filename:   zInfo.Filename,
			FileSize:   zInfo.FileSize,
		}, nil
	case "liber3":
		lClient := liber3.NewClient()
		lInfo, err := lClient.GetDownloadInfo(ctx, bookID)
		if err != nil {
			return nil, err
		}
		return &anna.DownloadInfo{
			DirectURL:  lInfo.DirectURL,
			MirrorURLs: lInfo.MirrorURLs,
			Filename:   lInfo.Filename,
			FileSize:   lInfo.FileSize,
		}, nil
	default:
		client := anna.NewClient()
		return client.GetDownloadInfo(ctx, bookID)
	}
}

// runDownloadByHash downloads a book by its MD5 hash or numeric ID
func runDownloadByHash(ctx context.Context, md5Hash string, outputDir string, bookInfo *anna.Book, source string) error {
	// Normalize hash
	md5Hash = strings.ToLower(strings.TrimSpace(md5Hash))

	if len(md5Hash) == 0 {
		return fmt.Errorf("invalid book ID: must not be empty")
	}

	// Detect source if not specified
	if source == "" {
		if bookInfo != nil && bookInfo.Source != "" {
			source = bookInfo.Source
		} else {
			var err error
			source, err = detectSource(md5Hash)
			if err != nil {
				return err
			}
		}
	}

	// Set output directory
	if outputDir == "" {
		outputDir = config.Get().Downloads.Path
	}

	// Ensure output directory exists
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Check if already downloaded
	existing, _ := db.GetDownloadByHash(md5Hash)
	if existing != nil {
		switch existing.Status {
		case db.StatusCompleted:
			fmt.Printf("Already downloaded: %s\n", existing.FilePath)
			return nil
		case db.StatusDownloading:
			fmt.Printf("Already downloading (ID: %d). Use 'bookdl list' to check status.\n", existing.ID)
			return nil
		case db.StatusPaused:
			fmt.Printf("Download paused (ID: %d). Use 'bookdl resume %d' to continue.\n", existing.ID, existing.ID)
			return nil
		case db.StatusFailed:
			fmt.Printf("Previous download failed. Restarting...\n")
			if err := db.ResetDownload(existing.ID); err != nil {
				return fmt.Errorf("failed to reset download: %w", err)
			}
		}
	}

	// Get book info if not provided
	if bookInfo == nil {
		fmt.Printf("Fetching book information...\n")
		switch source {
		case "zlibrary":
			// Z-Library doesn't support search-by-ID well, skip
		default:
			client := anna.NewClient()
			books, err := client.Search(ctx, md5Hash, 1)
			if err == nil && len(books) > 0 {
				bookInfo = books[0]
			}
		}
	}

	// Get download links
	fmt.Printf("Getting download links from %s...\n", source)
	dlInfo, err := getDownloadInfoForSource(ctx, md5Hash, source)
	if err != nil {
		return fmt.Errorf("failed to get download info: %w", err)
	}

	if dlInfo.DirectURL == "" && len(dlInfo.MirrorURLs) == 0 {
		return fmt.Errorf("no download links found")
	}

	// Determine filename
	filename := dlInfo.Filename
	if filename == "" && bookInfo != nil {
		// Create filename from book info
		safeName := sanitizeFilename(bookInfo.Title)
		ext := strings.ToLower(bookInfo.Format)
		if ext == "" {
			ext = "epub" // Default
		}
		filename = fmt.Sprintf("%s.%s", safeName, ext)
	}
	if filename == "" {
		filename = fmt.Sprintf("%s.epub", md5Hash)
	}

	// Apply file organization based on config
	filePath := OrganizedPath(outputDir, bookInfo, filename)

	// Ensure the organized directory exists
	if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	tempPath := filePath + ".part"

	// Create download record
	download := &db.Download{
		MD5Hash:   md5Hash,
		Title:     getTitle(bookInfo, md5Hash),
		Authors:   getAuthors(bookInfo),
		Format:    getFormat(bookInfo),
		SourceURL: buildSourceURL(source, md5Hash),
		FilePath:  filePath,
		TempPath:  tempPath,
		Status:    db.StatusPending,
	}

	// Get the primary download URL
	downloadURL := dlInfo.DirectURL
	if downloadURL == "" && len(dlInfo.MirrorURLs) > 0 {
		downloadURL = dlInfo.MirrorURLs[0]
	}

	download.DownloadURL = downloadURL

	// Save or update record
	if existing != nil && existing.Status == db.StatusPending {
		download.ID = existing.ID
	} else if existing == nil {
		if err := db.CreateDownload(download); err != nil {
			return fmt.Errorf("failed to create download record: %w", err)
		}
	}

	fmt.Printf("Downloading: %s\n", download.Title)
	fmt.Printf("Destination: %s\n", download.FilePath)
	fmt.Println()

	// Create download manager and start download
	mgr := downloader.NewManager()

	// Create context with configurable timeout
	timeout := config.Get().Downloads.Timeout
	if timeout == 0 {
		timeout = 30 * time.Minute
	}
	dlCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Collect all possible URLs to try
	urlsToTry := []string{downloadURL}
	for _, mirror := range dlInfo.MirrorURLs {
		if mirror != downloadURL {
			urlsToTry = append(urlsToTry, mirror)
		}
	}

	var lastErr error
	for i, tryURL := range urlsToTry {
		// For slow_download/fast_download URLs, resolve them via browser
		if strings.Contains(tryURL, "/slow_download/") || strings.Contains(tryURL, "/fast_download/") {
			if i > 0 {
				fmt.Printf("Trying mirror %d: resolving download link...\n", i+1)
			} else {
				fmt.Printf("Resolving download link...\n")
			}
			// Use dlCtx which respects the configured timeout
			resolvedURL, err := anna.NewBrowserClient(anna.GetBaseURL()).ResolveDownloadURL(dlCtx, tryURL)
			if err != nil {
				// Check if it's a timeout error
				if strings.Contains(err.Error(), "timeout") || strings.Contains(err.Error(), "context deadline exceeded") {
					fmt.Printf("Browser resolution timed out. Try increasing browser.max_countdown_wait in config.\n")
				}
				lastErr = fmt.Errorf("failed to resolve download link: %w", err)
				if i < len(urlsToTry)-1 {
					fmt.Printf("Trying next mirror...\n")
				}
				continue
			}
			tryURL = resolvedURL
		}

		download.DownloadURL = tryURL

		err := mgr.StartDownload(dlCtx, download)
		if err == nil {
			// Success! Mark as completed
			if err := db.MarkCompleted(download.ID, download.FilePath); err != nil {
				return fmt.Errorf("failed to mark download complete: %w", err)
			}

			// Verify checksum
			fmt.Println("Verifying checksum...")
			if err := downloader.VerifyAndMark(download); err != nil {
				fmt.Printf("⚠️  Warning: Checksum verification failed: %v\n", err)
				fmt.Printf("   File may be corrupted. Consider re-downloading.\n")
			} else {
				fmt.Println("✓ Checksum verified")
			}

			Successf("Downloaded: %s", download.FilePath)
			notify.DownloadComplete(download.Title)
			return nil
		}

		// Check if it's an HTML content error - try next mirror
		if err == downloader.ErrHTMLContent {
			fmt.Printf("Received HTML instead of file, trying next mirror...\n")
			lastErr = err
			continue
		}

		// For other errors, also try next mirror
		lastErr = err
		if i < len(urlsToTry)-1 {
			fmt.Printf("Download failed (%v), trying next mirror...\n", err)
		}
	}

	db.UpdateStatus(download.ID, db.StatusFailed, lastErr.Error())
	notify.DownloadFailed(download.Title, lastErr.Error())
	return fmt.Errorf("download failed after trying all mirrors: %w", lastErr)
}

// sanitizeFilename removes invalid characters from filename
func sanitizeFilename(name string) string {
	// Remove or replace invalid characters
	invalid := []string{"/", "\\", ":", "*", "?", "\"", "<", ">", "|"}
	for _, char := range invalid {
		name = strings.ReplaceAll(name, char, "_")
	}

	// Trim whitespace and limit length
	name = strings.TrimSpace(name)
	if len(name) > 100 {
		name = name[:100]
	}

	return name
}

func getTitle(book *anna.Book, fallback string) string {
	if book != nil && book.Title != "" {
		return book.Title
	}
	return fallback
}

func getAuthors(book *anna.Book) string {
	if book != nil {
		return book.Authors
	}
	return ""
}

func getFormat(book *anna.Book) string {
	if book != nil && book.Format != "" {
		return book.Format
	}
	return "EPUB"
}

func buildSourceURL(source, bookID string) string {
	switch source {
	case "zlibrary":
		return fmt.Sprintf("https://%s/book/%s", zlibrary.GetBaseURL(), bookID)
	case "liber3":
		return fmt.Sprintf("https://%s/book/%s", liber3.GetBaseURL(), bookID)
	default:
		return fmt.Sprintf("https://%s/md5/%s", anna.GetBaseURL(), bookID)
	}
}
