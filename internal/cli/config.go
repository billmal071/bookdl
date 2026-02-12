package cli

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/williams/bookdl/internal/config"
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

func init() {
	configCmd.AddCommand(configGetCmd)
	configCmd.AddCommand(configSetCmd)
	configCmd.AddCommand(configPathCmd)
}
