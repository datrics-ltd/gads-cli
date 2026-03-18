package cmd

import (
	"os"

	"github.com/datrics-ltd/gads-cli/internal/schema"
	"github.com/spf13/cobra"
)

var completionCmd = &cobra.Command{
	Use:   "completion [bash|zsh|fish|powershell]",
	Short: "Generate shell completion scripts",
	Long: `Generate shell completion scripts for gads.

To load completions:

Bash:
  $ source <(gads completion bash)

  # To load completions for each session, execute once:
  # Linux:
  $ gads completion bash > /etc/bash_completion.d/gads
  # macOS:
  $ gads completion bash > $(brew --prefix)/etc/bash_completion.d/gads

Zsh:
  # If shell completion is not already enabled in your environment,
  # you will need to enable it. You can execute the following once:
  $ echo "autoload -U compinit; compinit" >> ~/.zshrc

  # To load completions for each session, execute once:
  $ gads completion zsh > "${fpath[1]}/_gads"

  # You will need to start a new shell for this setup to take effect.

Fish:
  $ gads completion fish | source

  # To load completions for each session, execute once:
  $ gads completion fish > ~/.config/fish/completions/gads.fish

PowerShell:
  PS> gads completion powershell | Out-String | Invoke-Expression

  # To load completions for each session, add the following to your PowerShell profile:
  PS> gads completion powershell > gads.ps1
  # and source this file from your PowerShell profile.`,
	DisableFlagsInUseLine: true,
	ValidArgs:             []string{"bash", "zsh", "fish", "powershell"},
	Args:                  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		switch args[0] {
		case "bash":
			return rootCmd.GenBashCompletionV2(os.Stdout, true)
		case "zsh":
			return rootCmd.GenZshCompletion(os.Stdout)
		case "fish":
			return rootCmd.GenFishCompletion(os.Stdout, true)
		case "powershell":
			return rootCmd.GenPowerShellCompletionWithDesc(os.Stdout)
		}
		return nil
	},
}

// dateRangeCompletion completes Google Ads date range preset values.
func dateRangeCompletion(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	return []string{
		"TODAY",
		"YESTERDAY",
		"LAST_7_DAYS",
		"LAST_14_DAYS",
		"LAST_30_DAYS",
		"THIS_WEEK_SUN_TODAY",
		"THIS_WEEK_MON_TODAY",
		"LAST_WEEK_SUN_SAT",
		"LAST_WEEK_MON_SUN",
		"THIS_MONTH",
		"LAST_MONTH",
		"LAST_BUSINESS_WEEK",
		"ALL_TIME",
	}, cobra.ShellCompDirectiveNoFileComp
}

// resourceCompletion returns a ValidArgsFunction that completes resource names from the embedded schema.
func resourceCompletion() func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	return func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if len(args) > 0 {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		return schema.ListResources(), cobra.ShellCompDirectiveNoFileComp
	}
}

func init() {
	rootCmd.AddCommand(completionCmd)

	// --output flag completion (on root persistent flags, applies to all commands)
	_ = rootCmd.RegisterFlagCompletionFunc("output", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return []string{"table\tHuman-readable table", "json\tJSON array", "csv\tCSV with header row"}, cobra.ShellCompDirectiveNoFileComp
	})

	// schema <resource> — complete resource names
	schemaCmd.ValidArgsFunction = resourceCompletion()
}
