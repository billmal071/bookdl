package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/billmal071/bookdl/internal/db"
	"github.com/billmal071/bookdl/internal/downloader"
)

var verifyCmd = &cobra.Command{
	Use:   "verify [download-id]",
	Short: "Verify checksum of downloaded files",
	Long: `Verify the MD5 checksum of downloaded files.

Examples:
  bookdl verify 1          # Verify specific download
  bookdl verify --all      # Verify all completed downloads
  bookdl verify --failed   # Re-verify failed downloads`,
	RunE: runVerify,
}

func init() {
	verifyCmd.Flags().Bool("all", false, "verify all completed downloads")
	verifyCmd.Flags().Bool("failed", false, "re-verify downloads that failed verification")
	verifyCmd.Flags().Bool("fix", false, "automatically re-download corrupted files")
}

func runVerify(cmd *cobra.Command, args []string) error {
	verifyAll, _ := cmd.Flags().GetBool("all")
	verifyFailed, _ := cmd.Flags().GetBool("failed")
	autoFix, _ := cmd.Flags().GetBool("fix")

	var downloads []*db.Download
	var err error

	if verifyAll {
		// Verify all completed downloads
		downloads, err = db.ListDownloads(db.StatusCompleted, false)
		if err != nil {
			return fmt.Errorf("failed to list downloads: %w", err)
		}
	} else if verifyFailed {
		// Re-verify downloads that failed verification
		allDownloads, err := db.ListDownloads(db.StatusCompleted, false)
		if err != nil {
			return fmt.Errorf("failed to list downloads: %w", err)
		}
		for _, d := range allDownloads {
			if !d.Verified {
				downloads = append(downloads, d)
			}
		}
	} else if len(args) == 0 {
		return fmt.Errorf("provide a download ID or use --all flag")
	} else {
		// Verify specific download
		var id int64
		if _, err := fmt.Sscanf(args[0], "%d", &id); err != nil {
			return fmt.Errorf("invalid download ID: %s", args[0])
		}

		download, err := db.GetDownload(id)
		if err != nil {
			return fmt.Errorf("download not found: %w", err)
		}

		if download.Status != db.StatusCompleted {
			return fmt.Errorf("download is not completed (status: %s)", download.Status)
		}

		downloads = []*db.Download{download}
	}

	if len(downloads) == 0 {
		fmt.Println("No downloads to verify")
		return nil
	}

	fmt.Printf("Verifying %d download(s)...\n\n", len(downloads))

	verified := 0
	failed := 0
	missing := 0

	for _, download := range downloads {
		// Check if file exists
		if _, err := os.Stat(download.FilePath); os.IsNotExist(err) {
			fmt.Printf("❌ [%d] %s\n", download.ID, download.Title)
			fmt.Printf("    File not found: %s\n\n", download.FilePath)
			missing++
			continue
		}

		fmt.Printf("🔍 [%d] %s\n", download.ID, download.Title)
		fmt.Printf("    Verifying: %s\n", download.FilePath)

		err := downloader.VerifyAndMark(download)
		if err != nil {
			fmt.Printf("    ❌ Verification failed: %v\n", err)
			failed++

			if autoFix {
				fmt.Printf("    🔄 Re-downloading...\n")
				// Reset and re-download
				if err := db.ResetDownload(download.ID); err != nil {
					fmt.Printf("    ⚠️  Failed to reset download: %v\n", err)
				} else {
					// Trigger re-download
					if err := runDownloadByHash(cmd.Context(), download.MD5Hash, "", nil); err != nil {
						fmt.Printf("    ⚠️  Re-download failed: %v\n", err)
					} else {
						fmt.Printf("    ✓ Re-download completed\n")
					}
				}
			}
			fmt.Println()
		} else {
			fmt.Printf("    ✓ Checksum verified\n\n")
			verified++
		}
	}

	// Summary
	fmt.Println("─────────────────────────────────")
	fmt.Printf("Verified: %d\n", verified)
	if failed > 0 {
		fmt.Printf("Failed: %d\n", failed)
	}
	if missing > 0 {
		fmt.Printf("Missing: %d\n", missing)
	}

	if failed > 0 && !autoFix {
		fmt.Println("\nTip: Use --fix flag to automatically re-download corrupted files")
	}

	return nil
}
