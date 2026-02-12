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
)

var downloadCmd = &cobra.Command{
	Use:   "download [md5-hash]",
	Short: "Download a book by MD5 hash",
	Long: `Download a book from Anna's Archive using its MD5 hash.

The MD5 hash can be obtained from the search results.

Examples:
  bookdl download abc123def456789...
  bookdl download -o ~/Books abc123def456789...`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		outputDir, _ := cmd.Flags().GetString("output")
		return runDownloadByHash(cmd.Context(), args[0], outputDir, nil)
	},
}

func init() {
	downloadCmd.Flags().StringP("output", "o", "", "output directory (default: ~/Downloads/books)")
}

// runDownloadByHash downloads a book by its MD5 hash
func runDownloadByHash(ctx context.Context, md5Hash string, outputDir string, bookInfo *anna.Book) error {
	// Normalize hash
	md5Hash = strings.ToLower(strings.TrimSpace(md5Hash))

	// Validate hash format
	if len(md5Hash) != 32 {
		return fmt.Errorf("invalid MD5 hash: must be 32 characters")
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
	client := anna.NewClient()

	if bookInfo == nil {
		Printf("Fetching book information...\n")
		books, err := client.Search(ctx, md5Hash, 1)
		if err == nil && len(books) > 0 {
			bookInfo = books[0]
		}
	}

	// Get download links
	Printf("Getting download links...\n")
	dlInfo, err := client.GetDownloadInfo(ctx, md5Hash)
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
		SourceURL: fmt.Sprintf("https://%s/md5/%s", anna.GetBaseURL(), md5Hash),
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

	// Create context with timeout
	dlCtx, cancel := context.WithTimeout(ctx, 30*time.Minute)
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
				Printf("Trying mirror %d: resolving download link...\n", i+1)
			}
			resolvedURL, err := anna.NewBrowserClient(anna.GetBaseURL()).ResolveDownloadURL(ctx, tryURL)
			if err != nil {
				lastErr = err
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
			Successf("Downloaded: %s", download.FilePath)
			return nil
		}

		// Check if it's an HTML content error - try next mirror
		if err == downloader.ErrHTMLContent {
			Printf("Received HTML instead of file, trying next mirror...\n")
			lastErr = err
			continue
		}

		// For other errors, also try next mirror
		lastErr = err
		if i < len(urlsToTry)-1 {
			Printf("Download failed (%v), trying next mirror...\n", err)
		}
	}

	db.UpdateStatus(download.ID, db.StatusFailed, lastErr.Error())
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
