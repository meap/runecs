package cmd

import (
	"github.com/spf13/cobra"
	"runecs.io/v1/pkg/ecs"
)

func init() {
	listCmd := &cobra.Command{
		Use:                   "list",
		Short:                 "List of all services across clusters in the current region",
		DisableFlagsInUseLine: true,
		Run: func(cmd *cobra.Command, args []string) {
			all, _ := cmd.Flags().GetBool("all")

			ecs.List(all)
		},
	}

	listCmd.PersistentFlags().BoolP("all", "a", false, "list including running tasks")
	rootCmd.AddCommand(listCmd)
}
