package cmd

import (
	"log"
	"os"

	"github.com/spf13/cobra"
)

func newCompletionCommand() *cobra.Command {
	return &cobra.Command{
		Use:                   "completion [bash|zsh|fish|powershell]",
		Short:                 "Generate shell completion scripts",
		DisableFlagsInUseLine: true,
		ValidArgs:             []string{"bash", "zsh", "fish", "powershell"},
		Args:                  cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs),
		Run:                   completionHandler,
	}
}

func completionHandler(cmd *cobra.Command, args []string) {
	var err error

	switch args[0] {
	case "bash":
		err = cmd.Root().GenBashCompletion(os.Stdout)
	case "zsh":
		err = cmd.Root().GenZshCompletion(os.Stdout)
	case "fish":
		err = cmd.Root().GenFishCompletion(os.Stdout, true)
	case "powershell":
		err = cmd.Root().GenPowerShellCompletion(os.Stdout)
	}

	if err != nil {
		log.Fatalf("failed to generate completion script: %v\n", err)
	}
}

func init() {
	rootCmd.AddCommand(newCompletionCommand())
}
