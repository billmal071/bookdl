package cli

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/spf13/cobra"
	"github.com/billmal071/bookdl/internal/db"
	"github.com/billmal071/bookdl/internal/downloader"
)

var restartCmd = &cobra.Command{
	Use:   "restart [download-id]",
	Short: "Restart a download from scratch",
	Long: `Restart a download from the beginning, discarding any partial progress.

This is useful when a download is corrupted or you want to start fresh.

Examples:
  bookdl restart 1    Restart download #1 from scratch`,
	Args: cobra.ExactArgs(1),
	RunE: runRestart,
}

func runRestart(cmd *cobra.Command, args []string) error {
	id, err := strconv.ParseInt(args[0], 10, 64)
	if err != nil {
		return fmt.Errorf("invalid download ID: %s", args[0])
	}

	download, err := db.GetDownload(id)
	if err != nil {
		return fmt.Errorf("download not found: %w", err)
	}

	fmt.Printf("Restarting: %s\n", download.Title)

	// Reset download state
	if err := db.ResetDownload(id); err != nil {
		return fmt.Errorf("failed to reset download: %w", err)
	}

	// Re-fetch download record
	download, err = db.GetDownload(id)
	if err != nil {
		return fmt.Errorf("failed to get download: %w", err)
	}

	// Start fresh download
	mgr := downloader.NewManager()

	dlCtx, cancel := context.WithTimeout(cmd.Context(), 30*time.Minute)
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
