package cli

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/billmal071/bookdl/internal/anna"
	"github.com/billmal071/bookdl/internal/db"
)

var bookmarkCmd = &cobra.Command{
	Use:     "bookmark [md5]",
	Aliases: []string{"bm"},
	Short:   "Bookmark a book for later",
	Long: `Save a book to your bookmarks for later download.

Use without arguments to list all bookmarks.
Use with an MD5 hash to add a new bookmark.

Examples:
  bookdl bookmark                    List all bookmarks
  bookdl bookmark abc123def456...    Add book to bookmarks
  bookdl bookmark -d abc123...       Remove from bookmarks
  bookdl bookmark --download         Download all bookmarks`,
	RunE: runBookmark,
}

var bookmarksCmd = &cobra.Command{
	Use:   "bookmarks",
	Short: "List all bookmarks",
	Long: `List all saved bookmarks.

Examples:
  bookdl bookmarks              List all bookmarks
  bookdl bookmarks --download   Download all bookmarks`,
	RunE: runBookmarkList,
}

func init() {
	bookmarkCmd.Flags().BoolP("delete", "d", false, "remove bookmark")
	bookmarkCmd.Flags().Bool("download", false, "download all bookmarks")
	bookmarkCmd.Flags().StringP("note", "n", "", "add a note to the bookmark")

	bookmarksCmd.Flags().Bool("download", false, "download all bookmarks")
}

func runBookmark(cmd *cobra.Command, args []string) error {
	deleteMode, _ := cmd.Flags().GetBool("delete")
	downloadAll, _ := cmd.Flags().GetBool("download")
	note, _ := cmd.Flags().GetString("note")

	// Download all bookmarks
	if downloadAll {
		return downloadBookmarks(cmd.Context())
	}

	// List bookmarks if no args
	if len(args) == 0 {
		return runBookmarkList(cmd, args)
	}

	md5Hash := strings.ToLower(args[0])

	// Delete mode
	if deleteMode {
		return removeBookmark(md5Hash)
	}

	// Add bookmark
	return addBookmark(cmd.Context(), md5Hash, note)
}

func runBookmarkList(cmd *cobra.Command, args []string) error {
	downloadAll, _ := cmd.Flags().GetBool("download")

	if downloadAll {
		return downloadBookmarks(cmd.Context())
	}

	bookmarks, err := db.ListBookmarks()
	if err != nil {
		return fmt.Errorf("failed to list bookmarks: %w", err)
	}

	if len(bookmarks) == 0 {
		fmt.Println("No bookmarks saved.")
		fmt.Println("\nTo bookmark a book:")
		fmt.Println("  bookdl bookmark <md5-hash>")
		return nil
	}

	fmt.Printf("Bookmarks (%d):\n\n", len(bookmarks))

	for i, b := range bookmarks {
		// Title (truncate if too long)
		title := b.Title
		if len(title) > 50 {
			title = title[:47] + "..."
		}

		fmt.Printf("  %d. %s\n", i+1, title)

		var details []string
		if b.Authors != "" {
			authors := b.Authors
			if len(authors) > 30 {
				authors = authors[:27] + "..."
			}
			details = append(details, authors)
		}
		if b.Format != "" {
			details = append(details, b.Format)
		}
		if b.Size != "" {
			details = append(details, b.Size)
		}
		if len(details) > 0 {
			fmt.Printf("     %s\n", strings.Join(details, " | "))
		}

		fmt.Printf("     MD5: %s\n", b.MD5Hash)

		if b.Notes != "" {
			fmt.Printf("     Note: %s\n", b.Notes)
		}

		fmt.Printf("     Added: %s\n", b.CreatedAt.Format("2006-01-02"))
		fmt.Println()
	}

	fmt.Println("To download all bookmarks: bookdl bookmarks --download")
	return nil
}

func addBookmark(ctx context.Context, md5Hash string, note string) error {
	// Check if already bookmarked
	if db.BookmarkExists(md5Hash) {
		fmt.Println("Book is already bookmarked.")
		return nil
	}

	// Fetch book info
	fmt.Println("Fetching book info...")

	client := anna.NewClient()
	searchCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	// Try to get book info from the page
	info, err := client.GetDownloadInfo(searchCtx, md5Hash)
	if err != nil {
		// Create a minimal bookmark with just the hash
		bookmark := &db.Bookmark{
			MD5Hash: md5Hash,
			Title:   "Unknown (MD5: " + md5Hash[:16] + "...)",
			Notes:   note,
		}
		if err := db.CreateBookmark(bookmark); err != nil {
			return fmt.Errorf("failed to create bookmark: %w", err)
		}
		Successf("Bookmarked: %s", bookmark.Title)
		fmt.Println("Note: Could not fetch full book info. You can search for this book to get more details.")
		return nil
	}

	// Create bookmark with available info
	bookmark := &db.Bookmark{
		MD5Hash:  md5Hash,
		Title:    info.Filename,
		PageURL:  fmt.Sprintf("https://%s/md5/%s", anna.GetBaseURL(), md5Hash),
		Notes:    note,
	}

	// If filename is empty, use MD5
	if bookmark.Title == "" {
		bookmark.Title = "Book (MD5: " + md5Hash[:16] + "...)"
	}

	if err := db.CreateBookmark(bookmark); err != nil {
		return fmt.Errorf("failed to create bookmark: %w", err)
	}

	Successf("Bookmarked: %s", bookmark.Title)
	return nil
}

func removeBookmark(md5Hash string) error {
	bookmark, err := db.GetBookmarkByHash(md5Hash)
	if err != nil {
		return fmt.Errorf("bookmark not found")
	}

	if err := db.DeleteBookmarkByHash(md5Hash); err != nil {
		return fmt.Errorf("failed to remove bookmark: %w", err)
	}

	Successf("Removed bookmark: %s", bookmark.Title)
	return nil
}

func downloadBookmarks(ctx context.Context) error {
	bookmarks, err := db.ListBookmarks()
	if err != nil {
		return fmt.Errorf("failed to list bookmarks: %w", err)
	}

	if len(bookmarks) == 0 {
		fmt.Println("No bookmarks to download.")
		return nil
	}

	fmt.Printf("Downloading %d bookmark(s)...\n\n", len(bookmarks))

	success := 0
	var errors []error

	for _, b := range bookmarks {
		fmt.Printf("Processing: %s\n", b.Title)

		// Check if already downloaded
		existing, err := db.GetDownloadByHash(b.MD5Hash)
		if err == nil && existing != nil && existing.Status == db.StatusCompleted {
			fmt.Printf("  Already downloaded: %s\n", existing.FilePath)
			success++
			continue
		}

		// Start download
		if err := runDownloadByHash(ctx, b.MD5Hash, "", nil); err != nil {
			errors = append(errors, fmt.Errorf("%s: %w", b.Title, err))
		} else {
			success++
		}

		fmt.Println()
	}

	fmt.Printf("\nSummary: %d downloaded, %d failed\n", success, len(errors))

	if len(errors) > 0 {
		fmt.Println("\nFailed downloads:")
		for _, err := range errors {
			fmt.Printf("  - %s\n", err)
		}
	}

	return nil
}
