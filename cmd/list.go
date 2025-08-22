package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/list"
	"github.com/charmbracelet/lipgloss/table"
	"github.com/spf13/cobra"
	"runecs.io/v1/internal/ecs"
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

	// Set up context that cancels on interrupt signal for cancellable list operations
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	profile := rootCmd.Flag("profile").Value.String()
	clients, err := ecs.NewAWSClients(ctx, profile)

	if err != nil {
		return fmt.Errorf("failed to initialize AWS clients: %w", err)
	}

	clusters, err := ecs.GetClusters(ctx, clients, all)
	if err != nil {
		return err
	}

	if all {
		displayServicesWithDetails(cmd, clusters, clients.Region)
	} else {
		displayServices(cmd, clusters, clients.Region)
	}

	return nil
}

func displayServices(cmd *cobra.Command, clusters []ecs.ClusterInfo, region string) {
	boldStyle := lipgloss.NewStyle().Bold(true)
	enumStyle := lipgloss.NewStyle().MarginLeft(2).MarginRight(1)

	cmd.Printf("Services in all clusters (region: %s):\n", boldStyle.Render(region))
	cmd.Println()

	for _, cluster := range clusters {
		list := list.New().EnumeratorStyle(enumStyle)

		cmd.Println(cluster.Name)

		for _, service := range cluster.Services {
			serviceName := boldStyle.Render(service.Name)
			list.Item(fmt.Sprintf("%s/%s", service.ClusterName, serviceName))
		}

		if len(cluster.Services) > 0 {
			cmd.Println(list)
			cmd.Println()
		}
	}
}

func displayServicesWithDetails(cmd *cobra.Command, clusters []ecs.ClusterInfo, region string) {
	boldStyle := lipgloss.NewStyle().Bold(true)
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
		cmd.Println("No running tasks found.")

		return
	}

	t := table.New().
		Border(lipgloss.NormalBorder()).
		StyleFunc(func(row, col int) lipgloss.Style {
			if row == table.HeaderRow {
				return headerStyle
			}
			// Right-align CPU (col 2), Memory (col 3), and Running Time (col 4) columns
			if col == 2 || col == 3 || col == 4 {
				return cellStyle.Align(lipgloss.Right)
			}

			return cellStyle
		}).
		Headers("Service", "Task ID", "CPU", "Memory", "Running Time").
		Rows(rows...)

	cmd.Printf("Services with running tasks (region: %s):\n", boldStyle.Render(region))
	cmd.Println()
	cmd.Println(t)
}

func init() {
	rootCmd.AddCommand(newListCommand())
}
