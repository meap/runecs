package cmd

import (
	"github.com/spf13/cobra"
	"runecs.io/v1/pkg/ecs"
)

func newListCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:                   "list",
		Short:                 "List of all services across clusters in the current region",
		DisableFlagsInUseLine: true,
		Run:                   listHandler,
	}

	cmd.PersistentFlags().BoolP("all", "a", false, "list including running tasks")
	return cmd
}

func listHandler(cmd *cobra.Command, args []string) {
	all, _ := cmd.Flags().GetBool("all")
	ecs.List(all)
}

func init() {
	rootCmd.AddCommand(newListCommand())
}
