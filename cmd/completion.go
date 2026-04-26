package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var completionNoDesc bool

var completionCmd = &cobra.Command{
	Use:   "completion [bash|zsh|fish|powershell]",
	Short: "Generate shell completion scripts",
	Long: `Generate shell completion scripts for go-layout.

Load examples:
  bash:       source <(go-layout completion bash)
  zsh:        source <(go-layout completion zsh)
  fish:       go-layout completion fish | source
  powershell: go-layout completion powershell | Out-String | Invoke-Expression`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		switch args[0] {
		case "bash":
			return rootCmd.GenBashCompletionV2(os.Stdout, !completionNoDesc)
		case "zsh":
			return rootCmd.GenZshCompletion(os.Stdout)
		case "fish":
			return rootCmd.GenFishCompletion(os.Stdout, !completionNoDesc)
		case "powershell":
			return rootCmd.GenPowerShellCompletionWithDesc(os.Stdout)
		default:
			return fmt.Errorf("unsupported shell type %q", args[0])
		}
	},
}

func init() {
	completionCmd.Flags().BoolVar(&completionNoDesc, "no-descriptions", false, "Disable completion descriptions where supported")
}
