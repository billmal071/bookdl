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

	if len(downloads) == 0 {
		fmt.Println("No paused or failed downloads to resume.")
		return nil
	}

	fmt.Printf("Resuming %d download(s)...\n\n", len(downloads))

	var errors []error
	for _, d := range downloads {
		if err := resumeOne(ctx, d.ID); err != nil {
			errors = append(errors, fmt.Errorf("download #%d: %w", d.ID, err))
		}
	}

	if len(errors) > 0 {
		fmt.Printf("\n%d download(s) failed:\n", len(errors))
		for _, err := range errors {
			fmt.Printf("  - %s\n", err)
		}
	}

	return nil
}
