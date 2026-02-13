package cli

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/billmal071/bookdl/internal/config"
	"github.com/billmal071/bookdl/internal/db"
)

var cacheCmd = &cobra.Command{
	Use:   "cache",
	Short: "Manage search cache",
	Long: `Manage the search results cache.

Examples:
  bookdl cache stats    # Show cache statistics
  bookdl cache clear    # Clear all cached results
  bookdl cache enable   # Enable caching
  bookdl cache disable  # Disable caching`,
}

var cacheStatsCmd = &cobra.Command{
	Use:   "stats",
	Short: "Show cache statistics",
	RunE: func(cmd *cobra.Command, args []string) error {
		total, expired, err := db.GetCacheStats()
		if err != nil {
			return fmt.Errorf("failed to get cache stats: %w", err)
		}

		cfg := config.Get()

		fmt.Println("Search Cache Statistics")
		fmt.Println("─────────────────────────")
		fmt.Printf("Status: %s\n", enabledStatus(cfg.Cache.Enabled))
		fmt.Printf("Total cached results: %d\n", total)
		fmt.Printf("Expired entries: %d\n", expired)
		fmt.Printf("Valid entries: %d\n", total-expired)
		fmt.Printf("Cache TTL: %v\n", cfg.Cache.TTL)

		if expired > 0 {
			fmt.Println("\nTip: Run 'bookdl cache clean' to remove expired entries")
		}

		return nil
	},
}

var cacheClearCmd = &cobra.Command{
	Use:   "clear",
	Short: "Clear all cached search results",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := db.ClearSearchCache(); err != nil {
			return fmt.Errorf("failed to clear cache: %w", err)
		}
		Successf("Cache cleared")
		return nil
	},
}

var cacheCleanCmd = &cobra.Command{
	Use:   "clean",
	Short: "Remove expired cache entries",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := db.CleanExpiredCache(); err != nil {
			return fmt.Errorf("failed to clean cache: %w", err)
		}
		Successf("Expired entries removed")
		return nil
	},
}

var cacheEnableCmd = &cobra.Command{
	Use:   "enable",
	Short: "Enable search result caching",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := config.Set("cache.enabled", "true"); err != nil {
			return fmt.Errorf("failed to enable cache: %w", err)
		}
		Successf("Cache enabled")
		return nil
	},
}

var cacheDisableCmd = &cobra.Command{
	Use:   "disable",
	Short: "Disable search result caching",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := config.Set("cache.enabled", "false"); err != nil {
			return fmt.Errorf("failed to disable cache: %w", err)
		}
		Successf("Cache disabled")
		return nil
	},
}

func init() {
	cacheCmd.AddCommand(cacheStatsCmd)
	cacheCmd.AddCommand(cacheClearCmd)
	cacheCmd.AddCommand(cacheCleanCmd)
	cacheCmd.AddCommand(cacheEnableCmd)
	cacheCmd.AddCommand(cacheDisableCmd)
}

func enabledStatus(enabled bool) string {
	if enabled {
		return "enabled ✓"
	}
	return "disabled"
}
