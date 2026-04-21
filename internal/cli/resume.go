package cli

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/billmal071/bookdl/internal/config"
	"github.com/billmal071/bookdl/internal/db"
	"github.com/billmal071/bookdl/internal/downloader"
	"github.com/billmal071/bookdl/internal/notify"
	"github.com/spf13/cobra"
)

var resumeCmd = &cobra.Command{
	Use:   "resume [download-id|all]",
	Short: "Resume a paused download",
	Long: `Resume a paused or failed download.

Use 'all' to resume all paused downloads.

Examples:
  bookdl resume 1      Resume download #1
  bookdl resume all    Resume all paused downloads`,
	Args: cobra.ExactArgs(1),
	RunE: runResume,
}

func runResume(cmd *cobra.Command, args []string) error {
	arg := strings.ToLower(args[0])

	if arg == "all" {
		return resumeAll(cmd.Context())
	}

	id, err := strconv.ParseInt(arg, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid download ID: %s", arg)
	}

	return resumeOne(cmd.Context(), id)
}

func resumeOne(ctx context.Context, id int64) error {
	download, err := db.GetDownload(id)
	if err != nil {
		return fmt.Errorf("download not found: %w", err)
	}

	if download.Status == db.StatusCompleted {
		fmt.Printf("Download #%d is already completed.\n", id)
		return nil
	}

	if download.Status == db.StatusDownloading {
		fmt.Printf("Download #%d is already in progress.\n", id)
		return nil
	}

	fmt.Printf("Resuming: %s\n", download.Title)

	mgr := downloader.NewManager()

	// Use configurable timeout
	timeout := config.Get().Downloads.Timeout
	if timeout == 0 {
		timeout = 30 * time.Minute
	}
	dlCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	err = mgr.StartDownload(dlCtx, download)
	if err == nil {
		if err := db.MarkCompleted(download.ID, download.FilePath); err != nil {
			return fmt.Errorf("failed to mark complete: %w", err)
		}
		Successf("Downloaded: %s", download.FilePath)
		return nil
	}

	// If the stored URL failed, try re-fetching fresh download links
	errStr := err.Error()
	isStaleURL := strings.Contains(errStr, "504") ||
		strings.Contains(errStr, "410") ||
		strings.Contains(errStr, "403") ||
		strings.Contains(errStr, "404") ||
		strings.Contains(errStr, "no such host") ||
		strings.Contains(errStr, "timeout") ||
		strings.Contains(errStr, "context deadline exceeded")

	if !isStaleURL {
		db.UpdateStatus(download.ID, db.StatusFailed, errStr)
		return fmt.Errorf("download failed: %w", err)
	}

	source := db.InferSource(download)
	if source == "" {
		db.UpdateStatus(download.ID, db.StatusFailed, errStr)
		return fmt.Errorf("download failed (unknown source, cannot refresh URL): %w", err)
	}

	fmt.Printf("Stored URL failed, fetching fresh download links from %s...\n", source)

	dlInfo, fetchErr := GetDownloadInfoForSource(dlCtx, download.MD5Hash, source)
	if fetchErr != nil {
		db.UpdateStatus(download.ID, db.StatusFailed, errStr)
		return fmt.Errorf("download failed (could not refresh URL: %v): %w", fetchErr, err)
	}

	// Collect all fresh URLs to try
	urlsToTry := []string{}
	if dlInfo.DirectURL != "" {
		urlsToTry = append(urlsToTry, dlInfo.DirectURL)
	}
	for _, mirror := range dlInfo.MirrorURLs {
		if mirror != dlInfo.DirectURL {
			urlsToTry = append(urlsToTry, mirror)
		}
	}

	for i, tryURL := range urlsToTry {
		if i > 0 {
			fmt.Printf("Trying mirror %d...\n", i+1)
		}
		download.DownloadURL = tryURL
		db.UpdateDownloadURL(download.ID, tryURL)

		if tryErr := mgr.StartDownload(dlCtx, download); tryErr == nil {
			if err := db.MarkCompleted(download.ID, download.FilePath); err != nil {
				return fmt.Errorf("failed to mark complete: %w", err)
			}
			Successf("Downloaded: %s", download.FilePath)
			return nil
		}
	}

	db.UpdateStatus(download.ID, db.StatusFailed, "all mirrors failed after URL refresh")
	return fmt.Errorf("download failed after trying all fresh mirrors")
}

func resumeAll(ctx context.Context) error {
	downloads, err := db.ListDownloads(db.StatusPaused, false)
	if err != nil {
		return fmt.Errorf("failed to list downloads: %w", err)
	}

	// Also get failed downloads
	failed, err := db.ListDownloads(db.StatusFailed, false)
	if err == nil {
		downloads = append(downloads, failed...)
	}

	// Also get pending downloads (from queue)
	pending, err := db.ListDownloads(db.StatusPending, false)
	if err == nil {
		downloads = append(downloads, pending...)
	}

	if len(downloads) == 0 {
		fmt.Println("No downloads to resume.")
		return nil
	}

	mgr := downloader.NewManager()
	maxConcurrent := mgr.GetMaxConcurrent()

	fmt.Printf("Resuming %d download(s) (max %d concurrent)...\n\n", len(downloads), maxConcurrent)

	// Track completed and failed
	completed := 0
	var errors []error

	// Use concurrent downloads
	results := mgr.StartConcurrent(ctx, downloads, func(id int64, status string, progress float64) {
		// Progress callback - could be used for TUI in future
		switch status {
		case "starting":
			// Find download title
			for _, d := range downloads {
				if d.ID == id {
					fmt.Printf("⬇️  Starting: %s\n", d.Title)
					break
				}
			}
		case "completed":
			fmt.Printf("✅ Completed: download #%d\n", id)
		case "failed":
			fmt.Printf("❌ Failed: download #%d\n", id)
		}
	})

	// Process results
	for _, result := range results {
		if result.Error != nil {
			db.UpdateStatus(result.Download.ID, db.StatusFailed, result.Error.Error())
			errors = append(errors, fmt.Errorf("download #%d (%s): %w",
				result.Download.ID, result.Download.Title, result.Error))
		} else {
			if err := db.MarkCompleted(result.Download.ID, result.Download.FilePath); err != nil {
				errors = append(errors, fmt.Errorf("failed to mark #%d complete: %w", result.Download.ID, err))
			} else {
				completed++
			}
		}
	}

	fmt.Println()
	fmt.Printf("Summary: %d completed, %d failed\n", completed, len(errors))

	if len(errors) > 0 {
		fmt.Printf("\nFailed downloads:\n")
		for _, err := range errors {
			fmt.Printf("  - %s\n", err)
		}
	}

	// Send queue completion notification
	notify.QueueComplete(completed, len(errors))

	return nil
}
