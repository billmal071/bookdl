package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/billmal071/bookdl/internal/db"
)

var completionCmd = &cobra.Command{
	Use:   "completion [bash|zsh|fish|powershell]",
	Short: "Generate shell completion scripts",
	Long: `Generate shell completion scripts for bookdl.

To load completions:

Bash:
  $ source <(bookdl completion bash)

  # To load completions for each session, execute once:
  # Linux:
  $ bookdl completion bash > /etc/bash_completion.d/bookdl
  # macOS:
  $ bookdl completion bash > /usr/local/etc/bash_completion.d/bookdl

Zsh:
  # If shell completion is not already enabled in your environment,
  # you will need to enable it.  You can execute the following once:
  $ echo "autoload -U compinit; compinit" >> ~/.zshrc

  # To load completions for each session, execute once:
  $ bookdl completion zsh > "${fpath[1]}/_bookdl"

  # You will need to start a new shell for this setup to take effect.

Fish:
  $ bookdl completion fish | source

  # To load completions for each session, execute once:
  $ bookdl completion fish > ~/.config/fish/completions/bookdl.fish

PowerShell:
  PS> bookdl completion powershell | Out-String | Invoke-Expression

  # To load completions for every new session, run:
  PS> bookdl completion powershell > bookdl.ps1
  # and source this file from your PowerShell profile.`,
	DisableFlagsInUseLine: true,
	ValidArgs:             []string{"bash", "zsh", "fish", "powershell"},
	Args:                  cobra.ExactValidArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		switch args[0] {
		case "bash":
			return rootCmd.GenBashCompletion(os.Stdout)
		case "zsh":
			return rootCmd.GenZshCompletion(os.Stdout)
		case "fish":
			return rootCmd.GenFishCompletion(os.Stdout, true)
		case "powershell":
			return rootCmd.GenPowerShellCompletionWithDesc(os.Stdout)
		default:
			return fmt.Errorf("unsupported shell: %s", args[0])
		}
	},
}

func init() {
	// Add dynamic completion for download IDs
	resumeCmd.ValidArgsFunction = completeDownloadIDs
	pauseCmd.ValidArgsFunction = completeDownloadIDs
	restartCmd.ValidArgsFunction = completeDownloadIDs
	verifyCmd.ValidArgsFunction = completeDownloadIDs
}

// completeDownloadIDs provides dynamic completion for download IDs
func completeDownloadIDs(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	// Get all downloads
	downloads, err := db.ListDownloads("", true)
	if err != nil {
		return nil, cobra.ShellCompDirectiveError
	}

	var completions []string
	for _, d := range downloads {
		// Format: "ID:Title (Status)"
		completion := fmt.Sprintf("%d\t%s (%s)", d.ID, truncateTitle(d.Title, 40), d.Status)
		completions = append(completions, completion)
	}

	return completions, cobra.ShellCompDirectiveNoFileComp
}

// truncateTitle truncates a title to the specified length
func truncateTitle(title string, maxLen int) string {
	if len(title) <= maxLen {
		return title
	}
	return title[:maxLen-3] + "..."
}
