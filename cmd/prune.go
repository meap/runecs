package cmd

import (
	"github.com/spf13/cobra"
)

func newPruneCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:                   "prune",
		Short:                 "Deregister active task definitions",
		DisableFlagsInUseLine: true,
		Run:                   pruneHandler,
	}

	cmd.PersistentFlags().BoolP("dry-run", "", false, "dry run")
	cmd.PersistentFlags().IntP("keep-last", "", defaultLastNumberOfTasks, "keep last N task definitions")
	cmd.PersistentFlags().IntP("keep-days", "", defaultLastDays, "keep task definitions older than N days")
	return cmd
}

func pruneHandler(cmd *cobra.Command, args []string) {
	keepLastNr, _ := cmd.Flags().GetInt("keep-last")
	keepDays, _ := cmd.Flags().GetInt("keep-days")
	dryRun, _ := cmd.Flags().GetBool("dry-run")

	svc := initService()
	svc.Prune(keepLastNr, keepDays, dryRun)
}

func init() {
	rootCmd.AddCommand(newPruneCommand())
}
