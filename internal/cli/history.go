package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/billmal071/bookdl/internal/db"
)

var historyCmd = &cobra.Command{
	Use:   "history",
	Short: "View and manage search history",
	Long: `View and manage your search history.

Examples:
  bookdl history              List recent searches
  bookdl history clear        Clear all search history`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return showSearchHistory()
	},
}

var historyClearCmd = &cobra.Command{
	Use:   "clear",
	Short: "Clear all search history",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := db.ClearSearchHistory(); err != nil {
			return fmt.Errorf("failed to clear history: %w", err)
		}
		Successf("Search history cleared.")
		return nil
	},
}

var historyListCmd = &cobra.Command{
	Use:   "list",
	Short: "List recent searches",
	RunE: func(cmd *cobra.Command, args []string) error {
		limit, _ := cmd.Flags().GetInt("limit")
		return showSearchHistoryWithLimit(limit)
	},
}

func init() {
	historyListCmd.Flags().IntP("limit", "n", 20, "number of entries to show")

	historyCmd.AddCommand(historyClearCmd)
	historyCmd.AddCommand(historyListCmd)
}

// showSearchHistoryWithLimit shows history with a custom limit
func showSearchHistoryWithLimit(limit int) error {
	history, err := db.GetUniqueSearchHistory(limit)
	if err != nil {
		return fmt.Errorf("failed to get search history: %w", err)
	}

	if len(history) == 0 {
		fmt.Println("No search history.")
		fmt.Println("\nSearches are saved automatically when you search for books.")
		return nil
	}

	fmt.Printf("Recent Searches (%d):\n\n", len(history))

	for i, h := range history {
		fmt.Printf("  %d. \"%s\" (%d results)\n", i+1, h.Query, h.ResultCount)

		var filterParts []string
		if h.Filters.Format != "" {
			filterParts = append(filterParts, "format="+h.Filters.Format)
		}
		if h.Filters.Language != "" {
			filterParts = append(filterParts, "language="+h.Filters.Language)
		}
		if h.Filters.Year != "" {
			filterParts = append(filterParts, "year="+h.Filters.Year)
		}
		if h.Filters.MaxSize != "" {
			filterParts = append(filterParts, "max-size="+h.Filters.MaxSize)
		}
		if len(filterParts) > 0 {
			fmt.Printf("     Filters: %s\n", strings.Join(filterParts, ", "))
		}

		fmt.Printf("     %s\n\n", h.CreatedAt.Format("2006-01-02 15:04"))
	}

	return nil
}
