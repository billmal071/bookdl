package cli

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/billmal071/bookdl/internal/config"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage configuration",
	Long: `View and modify bookdl configuration.

Configuration is stored in ~/.config/bookdl/config.yaml

Examples:
  bookdl config get anna.api_key
  bookdl config set anna.api_key YOUR_API_KEY
  bookdl config set downloads.path ~/Books`,
}

var configGetCmd = &cobra.Command{
	Use:   "get [key]",
	Short: "Get a configuration value",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		key := args[0]
		value := config.GetValue(key)
		if value == nil {
			return fmt.Errorf("key not found: %s", key)
		}
		fmt.Printf("%s = %v\n", key, value)
		return nil
	},
}

var configSetCmd = &cobra.Command{
	Use:   "set [key] [value]",
	Short: "Set a configuration value",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		key := args[0]
		value := args[1]

		if err := config.Set(key, value); err != nil {
			return fmt.Errorf("failed to set config: %w", err)
		}

		Successf("Set %s = %s", key, value)
		fmt.Printf("Config saved to: %s\n", config.GetConfigPath())
		return nil
	},
}

var configPathCmd = &cobra.Command{
	Use:   "path",
	Short: "Show configuration file path",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("Config file: %s\n", config.GetConfigPath())
		fmt.Printf("Database:    %s\n", config.GetDBPath())
		fmt.Printf("Config dir:  %s\n", config.GetConfigDir())
	},
}

var configOrganizeCmd = &cobra.Command{
	Use:   "organize [mode]",
	Short: "Set file organization mode",
	Long: `Set how downloaded files are organized.

Available modes:
  flat     - All files in the download directory (default)
  author   - Organize by author name
  format   - Organize by file format (EPUB, PDF, etc.)
  year     - Organize by publication year
  custom   - Use a custom pattern (set with --pattern)

Examples:
  bookdl config organize author
  bookdl config organize custom --pattern "{author}/{year}"
  bookdl config organize flat`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		mode := args[0]

		// Validate mode
		validModes := []string{"flat", "author", "format", "year", "custom"}
		valid := false
		for _, m := range validModes {
			if mode == m {
				valid = true
				break
			}
		}
		if !valid {
			return fmt.Errorf("invalid mode: %s (use flat, author, format, year, or custom)", mode)
		}

		// Set the mode
		if err := config.Set("files.organize_mode", mode); err != nil {
			return fmt.Errorf("failed to set organize mode: %w", err)
		}

		// If custom mode, also set pattern if provided
		pattern, _ := cmd.Flags().GetString("pattern")
		if mode == "custom" && pattern != "" {
			if err := config.Set("files.organize_pattern", pattern); err != nil {
				return fmt.Errorf("failed to set pattern: %w", err)
			}
			Successf("File organization set to: %s (pattern: %s)", mode, pattern)
		} else {
			Successf("File organization set to: %s", mode)
		}

		// Handle rename flag
		rename, _ := cmd.Flags().GetBool("rename")
		if cmd.Flags().Changed("rename") {
			renameStr := "false"
			if rename {
				renameStr = "true"
			}
			if err := config.Set("files.rename_files", renameStr); err != nil {
				return fmt.Errorf("failed to set rename option: %w", err)
			}
			if rename {
				fmt.Println("Files will be renamed based on metadata.")
			}
		}

		return nil
	},
}

func init() {
	configOrganizeCmd.Flags().StringP("pattern", "p", "", "custom organization pattern (for custom mode)")
	configOrganizeCmd.Flags().Bool("rename", false, "rename files based on metadata")

	configCmd.AddCommand(configGetCmd)
	configCmd.AddCommand(configSetCmd)
	configCmd.AddCommand(configPathCmd)
	configCmd.AddCommand(configOrganizeCmd)
}
