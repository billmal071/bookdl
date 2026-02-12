package cli

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/billmal071/bookdl/internal/db"
	"github.com/billmal071/bookdl/internal/downloader"
	"github.com/billmal071/bookdl/internal/notify"
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

	dlCtx, cancel := context.WithTimeout(ctx, 30*time.Minute)
	defer cancel()

	if err := mgr.StartDownload(dlCtx, download); err != nil {
		db.UpdateStatus(download.ID, db.StatusFailed, err.Error())
		return fmt.Errorf("download failed: %w", err)
	}

	if err := db.MarkCompleted(download.ID, download.FilePath); err != nil {
		return fmt.Errorf("failed to mark complete: %w", err)
	}

	Successf("Downloaded: %s", download.FilePath)
	return nil
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
