package cli

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/billmal071/bookdl/internal/anna"
	"github.com/billmal071/bookdl/internal/tui"
)

var searchCmd = &cobra.Command{
	Use:   "search [query]",
	Short: "Search for books",
	Long: `Search Anna's Archive for books matching the query.

By default, shows an interactive selector to choose from the results.
Use -d/--download to immediately start downloading the selected book.

Examples:
  bookdl search "clean code"
  bookdl search -n 5 "golang programming"
  bookdl search -f epub "design patterns"
  bookdl search -d "pragmatic programmer"`,
	Args: cobra.MinimumNArgs(1),
	RunE: runSearch,
}

func init() {
	searchCmd.Flags().IntP("limit", "n", 5, "number of results to show")
	searchCmd.Flags().StringP("format", "f", "", "filter by format (epub, pdf)")
	searchCmd.Flags().BoolP("download", "d", false, "immediately download selected book")
	searchCmd.Flags().Bool("no-interactive", false, "disable interactive mode, just print results")
}

func runSearch(cmd *cobra.Command, args []string) error {
	query := strings.Join(args, " ")
	limit, _ := cmd.Flags().GetInt("limit")
	format, _ := cmd.Flags().GetString("format")
	autoDownload, _ := cmd.Flags().GetBool("download")
	noInteractive, _ := cmd.Flags().GetBool("no-interactive")

	Printf("Searching for: %s\n", query)

	// Create client and search
	client := anna.NewClient()

	ctx, cancel := context.WithTimeout(cmd.Context(), 60*time.Second)
	defer cancel()

	// Get extra results for filtering
	searchLimit := limit * 2
	if searchLimit < 10 {
		searchLimit = 10
	}

	books, err := client.Search(ctx, query, searchLimit)
	if err != nil {
		return fmt.Errorf("search failed: %w", err)
	}

	// Filter by format if specified
	if format != "" {
		books = filterByFormat(books, format)
	}

	// Limit results
	if len(books) > limit {
		books = books[:limit]
	}

	if len(books) == 0 {
		fmt.Println("No books found matching your query.")
		return nil
	}

	Printf("Found %d result(s)\n\n", len(books))

	// Non-interactive mode: just print results
	if noInteractive {
		printBooks(books)
		return nil
	}

	// Create load more function for pagination
	currentPage := 1
	loadMore := func() ([]*anna.Book, error) {
		currentPage++
		newCtx, newCancel := context.WithTimeout(cmd.Context(), 60*time.Second)
		defer newCancel()

		moreBooks, err := client.SearchPage(newCtx, query, searchLimit, currentPage)
		if err != nil {
			return nil, err
		}

		// Filter by format if specified
		if format != "" {
			moreBooks = filterByFormat(moreBooks, format)
		}

		// Limit results
		if len(moreBooks) > limit {
			moreBooks = moreBooks[:limit]
		}

		return moreBooks, nil
	}

	// Interactive selection with load more support
	selected, err := tui.RunSelectorWithLoadMore(books, loadMore)
	if err != nil {
		return fmt.Errorf("selection failed: %w", err)
	}

	if selected == nil {
		return nil // User cancelled
	}

	fmt.Println()

	if autoDownload {
		return startBookDownload(cmd.Context(), selected)
	}

	// Print selected book info
	fmt.Printf("Selected: %s\n", selected.Title)
	fmt.Printf("MD5: %s\n", selected.MD5Hash)
	fmt.Printf("\nTo download, run:\n")
	fmt.Printf("  bookdl download %s\n", selected.MD5Hash)

	return nil
}

// filterByFormat filters books by format
func filterByFormat(books []*anna.Book, format string) []*anna.Book {
	format = strings.ToUpper(format)
	var filtered []*anna.Book
	for _, book := range books {
		if strings.ToUpper(book.Format) == format {
			filtered = append(filtered, book)
		}
	}
	return filtered
}

// printBooks prints books in a simple format
func printBooks(books []*anna.Book) {
	for i, book := range books {
		fmt.Printf("%d. %s\n", i+1, book.Title)
		if book.Authors != "" {
			fmt.Printf("   Author: %s\n", book.Authors)
		}
		fmt.Printf("   Format: %s", book.Format)
		if book.Size != "" {
			fmt.Printf(" | Size: %s", book.Size)
		}
		if book.Language != "" {
			fmt.Printf(" | Language: %s", book.Language)
		}
		fmt.Println()
		fmt.Printf("   MD5: %s\n", book.MD5Hash)
		fmt.Println()
	}
}

// startBookDownload initiates a download for the selected book
func startBookDownload(ctx context.Context, book *anna.Book) error {
	// This will be implemented in the download command
	// For now, just print the command to run
	fmt.Printf("Starting download: %s\n", book.Title)
	return runDownloadByHash(ctx, book.MD5Hash, "", book)
}
