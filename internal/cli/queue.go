package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/billmal071/bookdl/internal/db"
)

var queueCmd = &cobra.Command{
	Use:   "queue",
	Short: "Manage download queue",
	Long: `Manage the download queue.

The queue shows pending downloads that haven't started yet.
Use subcommands to list, clear, or reorder the queue.

Examples:
  bookdl queue              List queued downloads
  bookdl queue list         List queued downloads
  bookdl queue clear        Clear all pending downloads
  bookdl queue remove 1 2 3 Remove specific items from queue`,
	RunE: runQueueList,
}

var queueListCmd = &cobra.Command{
	Use:   "list",
	Short: "List queued downloads",
	Long:  "List all downloads in the queue (pending status)",
	RunE:  runQueueList,
}

var queueClearCmd = &cobra.Command{
	Use:   "clear",
	Short: "Clear the download queue",
	Long:  "Remove all pending downloads from the queue",
	RunE:  runQueueClear,
}

var queueRemoveCmd = &cobra.Command{
	Use:   "remove [ids...]",
	Short: "Remove items from queue",
	Long: `Remove specific items from the download queue by their ID.

Examples:
  bookdl queue remove 1       Remove item #1
  bookdl queue remove 1 2 3   Remove items #1, #2, and #3`,
	Args: cobra.MinimumNArgs(1),
	RunE: runQueueRemove,
}

var queuePriorityCmd = &cobra.Command{
	Use:   "priority [id] [top|bottom|value]",
	Short: "Change priority of a queue item",
	Long: `Change the priority of a specific queue item.

Examples:
  bookdl queue priority 1 top       Move item #1 to top of queue
  bookdl queue priority 1 bottom    Move item #1 to bottom of queue
  bookdl queue priority 1 10        Set item #1 priority to 10`,
	Args: cobra.ExactArgs(2),
	RunE: runQueuePriority,
}

func init() {
	queueCmd.AddCommand(queueListCmd)
	queueCmd.AddCommand(queueClearCmd)
	queueCmd.AddCommand(queueRemoveCmd)
	queueCmd.AddCommand(queuePriorityCmd)
}

func runQueueList(cmd *cobra.Command, args []string) error {
	downloads, err := db.ListDownloads(db.StatusPending, true)
	if err != nil {
		return fmt.Errorf("failed to list queue: %w", err)
	}

	if len(downloads) == 0 {
		fmt.Println("Queue is empty.")
		return nil
	}

	fmt.Printf("Download Queue (%d):\n\n", len(downloads))

	for i, d := range downloads {
		// Title (truncate if too long)
		title := d.Title
		if len(title) > 50 {
			title = title[:47] + "..."
		}

		// Show priority indicator if non-zero
		priorityStr := ""
		if d.Priority != 0 {
			priorityStr = fmt.Sprintf(" [Priority: %d]", d.Priority)
		}

		fmt.Printf("  %d. [%d]%s %s\n", i+1, d.ID, priorityStr, title)

		var details []string
		if d.Format != "" {
			details = append(details, d.Format)
		}
		if d.FileSize > 0 {
			details = append(details, formatBytes(d.FileSize))
		}
		if d.Authors != "" {
			authors := d.Authors
			if len(authors) > 30 {
				authors = authors[:27] + "..."
			}
			details = append(details, authors)
		}
		if len(details) > 0 {
			fmt.Printf("     %s\n", strings.Join(details, " | "))
		}
	}

	fmt.Println()
	fmt.Println("Run 'bookdl resume all' to start downloading.")
	fmt.Println("Use 'bookdl queue priority <id> top|bottom' to reorder.")
	return nil
}

func runQueueClear(cmd *cobra.Command, args []string) error {
	downloads, err := db.ListDownloads(db.StatusPending, true)
	if err != nil {
		return fmt.Errorf("failed to list queue: %w", err)
	}

	if len(downloads) == 0 {
		fmt.Println("Queue is already empty.")
		return nil
	}

	// Delete all pending downloads
	count := 0
	for _, d := range downloads {
		if err := db.DeleteDownload(d.ID); err != nil {
			Errorf("failed to remove %s: %v", d.Title, err)
		} else {
			count++
		}
	}

	Successf("Cleared %d item(s) from the queue.", count)
	return nil
}

func runQueueRemove(cmd *cobra.Command, args []string) error {
	removed := 0
	for _, idStr := range args {
		var id int64
		if _, err := fmt.Sscanf(idStr, "%d", &id); err != nil {
			Errorf("invalid ID: %s", idStr)
			continue
		}

		// Get the download to verify it's pending
		download, err := db.GetDownload(id)
		if err != nil {
			Errorf("download #%d not found", id)
			continue
		}

		if download.Status != db.StatusPending {
			Errorf("download #%d is not in queue (status: %s)", id, download.Status)
			continue
		}

		if err := db.DeleteDownload(id); err != nil {
			Errorf("failed to remove #%d: %v", id, err)
		} else {
			removed++
			Printf("Removed: %s\n", download.Title)
		}
	}

	if removed > 0 {
		Successf("Removed %d item(s) from the queue.", removed)
	}
	return nil
}

func runQueuePriority(cmd *cobra.Command, args []string) error {
	// Parse download ID
	var id int64
	if _, err := fmt.Sscanf(args[0], "%d", &id); err != nil {
		return fmt.Errorf("invalid ID: %s", args[0])
	}

	// Get the download to verify it's pending
	download, err := db.GetDownload(id)
	if err != nil {
		return fmt.Errorf("download #%d not found", id)
	}

	if download.Status != db.StatusPending {
		return fmt.Errorf("download #%d is not in queue (status: %s)", id, download.Status)
	}

	action := strings.ToLower(args[1])

	switch action {
	case "top":
		if err := db.SetPriorityTop(id); err != nil {
			return fmt.Errorf("failed to set priority: %w", err)
		}
		Successf("Moved %s to top of queue.", download.Title)
	case "bottom":
		if err := db.SetPriorityBottom(id); err != nil {
			return fmt.Errorf("failed to set priority: %w", err)
		}
		Successf("Moved %s to bottom of queue.", download.Title)
	default:
		// Try to parse as numeric priority
		var priority int
		if _, err := fmt.Sscanf(action, "%d", &priority); err != nil {
			return fmt.Errorf("invalid priority value: %s (use 'top', 'bottom', or a number)", action)
		}
		if err := db.UpdatePriority(id, priority); err != nil {
			return fmt.Errorf("failed to set priority: %w", err)
		}
		Successf("Set priority of %s to %d.", download.Title, priority)
	}

	return nil
}
