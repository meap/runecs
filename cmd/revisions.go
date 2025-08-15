package cmd

import (
	"github.com/spf13/cobra"
)

func init() {
	revisionsCmd := &cobra.Command{
		Use:                   "revisions",
		Short:                 "List of active task definitions",
		DisableFlagsInUseLine: true,
		Run: func(cmd *cobra.Command, args []string) {
			revNr, _ := cmd.Flags().GetInt("last")

			svc := initService()
			svc.Revisions(revNr)
		},
	}

	revisionsCmd.PersistentFlags().IntP("last", "", 0, "last N revisions")
	rootCmd.AddCommand(revisionsCmd)
}
