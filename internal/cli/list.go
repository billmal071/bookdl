package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/billmal071/bookdl/internal/db"
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List downloads",
	Long: `List all downloads and their status.

By default, completed downloads are hidden. Use -a/--all to show them.

Examples:
  bookdl list                  List active downloads
  bookdl list -a               List all downloads
  bookdl list -s paused        List paused downloads
  bookdl list -s failed        List failed downloads`,
	RunE: runList,
}

func init() {
	listCmd.Flags().StringP("status", "s", "", "filter by status (pending, downloading, paused, completed, failed)")
	listCmd.Flags().BoolP("all", "a", false, "show all downloads including completed")
}

func runList(cmd *cobra.Command, args []string) error {
	statusFilter, _ := cmd.Flags().GetString("status")
	showAll, _ := cmd.Flags().GetBool("all")

	var status db.DownloadStatus
	if statusFilter != "" {
		status = db.DownloadStatus(strings.ToLower(statusFilter))
	}

	downloads, err := db.ListDownloads(status, showAll)
	if err != nil {
		return fmt.Errorf("failed to list downloads: %w", err)
	}

	if len(downloads) == 0 {
		if statusFilter != "" {
			fmt.Printf("No downloads with status '%s'.\n", statusFilter)
		} else {
			fmt.Println("No active downloads.")
		}
		return nil
	}

	fmt.Printf("Downloads (%d):\n\n", len(downloads))

	for _, d := range downloads {
		printDownload(d)
	}

	return nil
}

func printDownload(d *db.Download) {
	// Status indicator
	var statusIcon string
	switch d.Status {
	case db.StatusPending:
		statusIcon = "⏳"
	case db.StatusDownloading:
		statusIcon = "⬇️ "
	case db.StatusPaused:
		statusIcon = "⏸️ "
	case db.StatusCompleted:
		statusIcon = "✅"
	case db.StatusFailed:
		statusIcon = "❌"
	default:
		statusIcon = "  "
	}

	// Title (truncate if too long)
	title := d.Title
	if len(title) > 50 {
		title = title[:47] + "..."
	}

	fmt.Printf("%s [%d] %s\n", statusIcon, d.ID, title)

	// Progress
	if d.FileSize > 0 {
		progress := float64(d.DownloadedSize) / float64(d.FileSize) * 100
		fmt.Printf("   Progress: %.1f%% (%s / %s)\n",
			progress,
			formatBytes(d.DownloadedSize),
			formatBytes(d.FileSize))
	}

	// Status details
	fmt.Printf("   Status: %s", d.Status)
	if d.ErrorMessage != "" {
		fmt.Printf(" - %s", d.ErrorMessage)
	}
	fmt.Println()

	// File info
	if d.FilePath != "" {
		fmt.Printf("   File: %s\n", d.FilePath)
	}

	// MD5
	fmt.Printf("   MD5: %s\n", d.MD5Hash)

	fmt.Println()
}

func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}
