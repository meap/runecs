package cmd

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
	"runecs.io/v1/pkg/ecs"
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

	// Set up context that cancels on interrupt signal for cancellable prune operations
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	profile := rootCmd.Flag("profile").Value.String()
	clients, err := ecs.NewAWSClients(ctx, profile)
	if err != nil {
		log.Fatalf("Failed to initialize AWS clients: %v\n", err)
	}

	cluster, service := parseServiceFlag()
	result, err := ecs.Prune(ctx, clients, cluster, service, keepLastNr, keepDays, dryRun)
	if err != nil {
		log.Fatalln(err)
	}

	// Display families being processed
	fmt.Printf("Processing %d task definition families: %v\n", len(result.Families), result.Families)
	fmt.Println()

	// Create lipgloss style for ARN formatting
	arnStyle := lipgloss.NewStyle().Bold(true)

	// Display detailed task actions
	for _, task := range result.ProcessedTasks {
		switch task.Action {
		case "kept":
			fmt.Printf("Task definition %s created %d days ago was skipped (%s)\n",
				arnStyle.Render(task.Arn), task.DaysOld, task.Reason)
		case "deleted":
			if result.DryRun {
				fmt.Printf("Task definition %s created %d days ago would be deregistered\n",
					arnStyle.Render(task.Arn), task.DaysOld)
			} else {
				fmt.Printf("Task definition %s created %d days ago was deregistered\n",
					arnStyle.Render(task.Arn), task.DaysOld)
			}
		case "skipped":
			fmt.Printf("Task definition %s skipped: %s\n", arnStyle.Render(task.Arn), task.Reason)
		}
	}

	fmt.Println()

	// Display summary
	if result.DryRun {
		fmt.Printf("Total of %d task definitions. Will delete %d definitions.\n",
			result.TotalCount, result.DeletedCount)
	} else {
		fmt.Printf("Total of %d task definitions. Deleted %d definitions.\n",
			result.TotalCount, result.DeletedCount)
	}
}

func init() {
	rootCmd.AddCommand(newPruneCommand())
}
