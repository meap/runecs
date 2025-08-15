package cmd

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/list"
	"github.com/charmbracelet/lipgloss/table"
	"github.com/spf13/cobra"
	"runecs.io/v1/pkg/ecs"
)

func newListCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:                   "list",
		Short:                 "List all services across clusters in the current region",
		DisableFlagsInUseLine: true,
		RunE:                  listHandler,
	}

	cmd.PersistentFlags().BoolP("all", "a", false, "include running tasks in output")
	return cmd
}

func listHandler(cmd *cobra.Command, args []string) error {
	all, _ := cmd.Flags().GetBool("all")

	clusters, err := ecs.GetClusters(all)
	if err != nil {
		return err
	}

	if all {
		displayServicesWithDetails(clusters)
	} else {
		displayServices(clusters)
	}

	return nil
}

func displayServices(clusters []ecs.ClusterInfo) {
	boldStyle := lipgloss.NewStyle().Bold(true)
	enumStyle := lipgloss.NewStyle().MarginLeft(2).MarginRight(1)

	fmt.Println("Services in all clusters:")
	fmt.Println()

	for _, cluster := range clusters {
		list := list.New().EnumeratorStyle(enumStyle)
		fmt.Println(cluster.Name)

		for _, service := range cluster.Services {
			serviceName := boldStyle.Render(service.Name)
			list.Item(fmt.Sprintf("%s/%s", service.ClusterName, serviceName))
		}

		if len(cluster.Services) > 0 {
			fmt.Println(list)
			fmt.Println()
		}
	}
}

func displayServicesWithDetails(clusters []ecs.ClusterInfo) {
	headerStyle := lipgloss.NewStyle().Bold(true).Align(lipgloss.Center)
	cellStyle := lipgloss.NewStyle().Padding(0, 1)

	var rows [][]string

	for _, cluster := range clusters {
		for _, service := range cluster.Services {
			boldServiceName := lipgloss.NewStyle().Bold(true).Render(service.Name)
			serviceName := fmt.Sprintf("%s/%s", service.ClusterName, boldServiceName)
			for _, task := range service.Tasks {
				rows = append(rows, []string{
					serviceName,
					task.ID,
					task.CPU,
					task.Memory,
					task.RunningTime,
				})
			}
		}
	}

	if len(rows) == 0 {
		fmt.Println("No running tasks found.")
		return
	}

	t := table.New().
		Border(lipgloss.NormalBorder()).
		StyleFunc(func(row, col int) lipgloss.Style {
			if row == table.HeaderRow {
				return headerStyle
			}
			return cellStyle
		}).
		Headers("Service", "Task ID", "CPU", "Memory", "Running Time").
		Rows(rows...)

	fmt.Println("Services with running tasks:")
	fmt.Println()
	fmt.Println(t)
}

func init() {
	rootCmd.AddCommand(newListCommand())
}
