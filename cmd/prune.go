package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
	"runecs.io/v1/internal/ecs"
)

func newPruneCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:                   "prune",
		Short:                 "Deregister active task definitions",
		DisableFlagsInUseLine: true,
		RunE:                  pruneHandler,
	}

	cmd.PersistentFlags().BoolP("dry-run", "", false, "dry run")
	cmd.PersistentFlags().IntP("keep-last", "", defaultLastNumberOfTasks, "keep last N task definitions")
	cmd.PersistentFlags().IntP("keep-days", "", defaultLastDays, "keep task definitions older than N days")

	return cmd
}

func pruneHandler(cmd *cobra.Command, args []string) error {
	keepLastNr, _ := cmd.Flags().GetInt("keep-last")
	keepDays, _ := cmd.Flags().GetInt("keep-days")
	dryRun, _ := cmd.Flags().GetBool("dry-run")

	// Set up context that cancels on interrupt signal for cancellable prune operations
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	profile := rootCmd.Flag("profile").Value.String()
	clients, err := ecs.NewAWSClients(ctx, profile)
	if err != nil {
		return fmt.Errorf("failed to initialize AWS clients: %w", err)
	}

	cluster, service, err := parseServiceFlag()
	if err != nil {
		return err
	}
	result, err := ecs.Prune(ctx, clients, cluster, service, keepLastNr, keepDays, dryRun)
	if err != nil {
		return fmt.Errorf("failed to prune service: %w", err)
	}

	// Display families being processed
	cmd.Printf("Processing %d task definition families: %v\n", len(result.Families), result.Families)
	cmd.Println()

	// Create lipgloss style for ARN formatting
	arnStyle := lipgloss.NewStyle().Bold(true)

	// Display detailed task actions
	for _, task := range result.ProcessedTasks {
		switch task.Action {
		case "kept":
			cmd.Printf("Task definition %s created %d days ago was skipped (%s)\n",
				arnStyle.Render(task.Arn), task.DaysOld, task.Reason)
		case "deleted":
			if result.DryRun {
				cmd.Printf("Task definition %s created %d days ago would be deregistered\n",
					arnStyle.Render(task.Arn), task.DaysOld)
			} else {
				cmd.Printf("Task definition %s created %d days ago was deregistered\n",
					arnStyle.Render(task.Arn), task.DaysOld)
			}
		case "skipped":
			cmd.Printf("Task definition %s skipped: %s\n", arnStyle.Render(task.Arn), task.Reason)
		}
	}

	cmd.Println()

	// Display summary
	if result.DryRun {
		cmd.Printf("Total of %d task definitions. Will delete %d definitions.\n",
			result.TotalCount, result.DeletedCount)
	} else {
		cmd.Printf("Total of %d task definitions. Deleted %d definitions.\n",
			result.TotalCount, result.DeletedCount)
	}

	return nil
}

func init() {
	rootCmd.AddCommand(newPruneCommand())
}
