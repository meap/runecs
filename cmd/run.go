// ABOUTME: Command-line interface for running ECS tasks
// ABOUTME: Handles user input and output formatting for task execution

package cmd

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
	"runecs.io/v1/pkg/ecs"
)

func newRunCommand() *cobra.Command {
	var dockerImageTag string
	var execWait bool

	cmd := &cobra.Command{
		Use:                   "run [cmd]",
		Short:                 "Execute a one-off process in an AWS ECS cluster",
		Args:                  cobra.MinimumNArgs(1),
		DisableFlagsInUseLine: true,
		Run:                   runHandler(&dockerImageTag, &execWait),
	}

	cmd.PersistentFlags().BoolVarP(&execWait, "wait", "w", false, "wait for task to finish")
	cmd.PersistentFlags().StringVarP(&dockerImageTag, "image-tag", "i", "", "docker image tag")
	return cmd
}

func runHandler(dockerImageTag *string, execWait *bool) func(*cobra.Command, []string) {
	return func(cmd *cobra.Command, args []string) {
		cluster, service := parseServiceFlag()

		// Set up context that cancels on interrupt signal for proper Ctrl+C handling
		ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer cancel()

		result, err := ecs.Execute(ctx, cluster, service, args, *execWait, *dockerImageTag)
		if err != nil {
			log.Fatalln(err)
		}

		// Display service loading information
		fmt.Printf("Service '%s' loaded.\n", result.ServiceName)

		// Display task definition information
		if result.NewTaskDefCreated {
			fmt.Printf("New task definition %s created\n", result.TaskDefinition)
		} else {
			fmt.Printf("Using task definition %s\n", result.TaskDefinition)
		}

		fmt.Println()

		// Display task execution information
		fmt.Printf("Task %s executed\n", result.TaskArn)

		// Display logs if waiting
		if *execWait {
			for _, logEntry := range result.Logs {
				fmt.Println(logEntry.StreamName, logEntry.Message)
			}

			if result.Finished {
				fmt.Printf("Task %s finished\n", result.TaskArn)
			}
		}
	}
}

func init() {
	rootCmd.AddCommand(newRunCommand())
}
