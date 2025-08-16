package cmd

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/dustin/go-humanize"
	"github.com/spf13/cobra"
	"runecs.io/v1/pkg/ecs"
)

func newRestartCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:                   "restart",
		Short:                 "Restart the service",
		DisableFlagsInUseLine: true,
		Run:                   restartHandler,
	}

	cmd.PersistentFlags().BoolP("kill", "", false, "Stops running tasks, ECS starts a new one if the health check is properly set")
	return cmd
}

func restartHandler(cmd *cobra.Command, args []string) {
	kill, _ := cmd.Flags().GetBool("kill")

	// Set up context that cancels on interrupt signal for cancellable restart operations
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	profile := rootCmd.Flag("profile").Value.String()
	clients, err := ecs.NewAWSClients(profile)
	if err != nil {
		log.Fatalf("Failed to initialize AWS clients: %v\n", err)
	}

	cluster, service := parseServiceFlag()
	result, err := ecs.Restart(ctx, clients, cluster, service, kill)
	if err != nil {
		log.Fatalf("Restart failed: %v\n", err)
	}

	if result.Method == "kill" {
		for _, stoppedTask := range result.StoppedTasks {
			fmt.Printf("Stopped task %s started %s\n", stoppedTask.TaskArn, humanize.Time(stoppedTask.StartedAt))
		}
	} else {
		fmt.Printf("Service %s restarted by starting new tasks using task definition %s.\n", service, result.TaskDefinition)
	}

	fmt.Println("Done.")
}

func init() {
	rootCmd.AddCommand(newRestartCommand())
}
