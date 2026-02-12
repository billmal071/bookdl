package cli

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"github.com/billmal071/bookdl/internal/db"
	"github.com/billmal071/bookdl/internal/downloader"
)

var pauseCmd = &cobra.Command{
	Use:   "pause [download-id|all]",
	Short: "Pause an active download",
	Long: `Pause an active download. The download can be resumed later.

Use 'all' to pause all active downloads.

Examples:
  bookdl pause 1      Pause download #1
  bookdl pause all    Pause all active downloads`,
	Args: cobra.ExactArgs(1),
	RunE: runPause,
}

func runPause(cmd *cobra.Command, args []string) error {
	arg := strings.ToLower(args[0])

	if arg == "all" {
		return pauseAll()
	}

	id, err := strconv.ParseInt(arg, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid download ID: %s", arg)
	}

	return pauseOne(id)
}

func pauseOne(id int64) error {
	download, err := db.GetDownload(id)
	if err != nil {
		return fmt.Errorf("download not found: %w", err)
	}

	if download.Status == db.StatusCompleted {
		fmt.Printf("Download #%d is already completed.\n", id)
		return nil
	}

	if download.Status == db.StatusPaused {
		fmt.Printf("Download #%d is already paused.\n", id)
		return nil
	}

	mgr := downloader.NewManager()
	if err := mgr.PauseDownload(id); err != nil {
		return fmt.Errorf("failed to pause: %w", err)
	}

	Successf("Paused: %s (ID: %d)", download.Title, id)
	fmt.Printf("Use 'bookdl resume %d' to continue.\n", id)
	return nil
}

func pauseAll() error {
	downloads, err := db.ListDownloads(db.StatusDownloading, false)
	if err != nil {
		return fmt.Errorf("failed to list downloads: %w", err)
	}

	if len(downloads) == 0 {
		fmt.Println("No active downloads to pause.")
		return nil
	}

	mgr := downloader.NewManager()
	paused := 0

	for _, d := range downloads {
		if err := mgr.PauseDownload(d.ID); err != nil {
			Errorf("Failed to pause #%d: %s", d.ID, err)
		} else {
			paused++
			fmt.Printf("⏸️  Paused: %s (ID: %d)\n", d.Title, d.ID)
		}
	}

	if paused > 0 {
		fmt.Printf("\nPaused %d download(s). Use 'bookdl resume all' to continue.\n", paused)
	}

	return nil
}
