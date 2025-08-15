package cmd

import (
	"github.com/spf13/cobra"
)

func newRevisionsCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:                   "revisions",
		Short:                 "List of active task definitions",
		DisableFlagsInUseLine: true,
		Run:                   revisionsHandler,
	}

	cmd.PersistentFlags().IntP("last", "", 0, "last N revisions")
	return cmd
}

func revisionsHandler(cmd *cobra.Command, args []string) {
	revNr, _ := cmd.Flags().GetInt("last")

	svc := initService()
	svc.Revisions(revNr)
}

func init() {
	rootCmd.AddCommand(newRevisionsCommand())
}
