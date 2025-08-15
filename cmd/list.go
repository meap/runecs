package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"runecs.io/v1/pkg/ecs"
)

func newListCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:                   "list",
		Short:                 "List of all services across clusters in the current region",
		DisableFlagsInUseLine: true,
		RunE:                  listHandler,
	}

	cmd.PersistentFlags().BoolP("all", "a", false, "list including running tasks")
	return cmd
}

func listHandler(cmd *cobra.Command, args []string) error {
	all, _ := cmd.Flags().GetBool("all")

	clusters, err := ecs.GetClusters(all)
	if err != nil {
		return err
	}

	// Format and print the output in the cmd layer
	for _, cluster := range clusters {
		for _, service := range cluster.Services {
			fmt.Printf("%s/%s\n", service.ClusterName, service.Name)
			if all {
				for _, task := range service.Tasks {
					fmt.Printf("  %s: %s (Cpu) / %s (Memory) (Running for: %s)\n",
						task.ID,
						task.CPU,
						task.Memory,
						task.RunningTime,
					)
				}
			}
		}
	}

	return nil
}

func init() {
	rootCmd.AddCommand(newListCommand())
}
