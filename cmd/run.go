// ABOUTME: Command-line interface for running ECS tasks
// ABOUTME: Handles user input and output formatting for task execution

package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/buildkite/shellwords"
	"github.com/spf13/cobra"
	"runecs.io/v1/internal/ecs"
)

func newRunCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:                   "run [cmd]",
		Short:                 "Execute a one-off process in an AWS ECS cluster",
		Args:                  cobra.MinimumNArgs(1),
		DisableFlagsInUseLine: true,
		RunE:                  runHandler,
	}

	cmd.PersistentFlags().BoolP("wait", "w", false, "wait for task to finish")
	cmd.PersistentFlags().StringP("image-tag", "i", "", "docker image tag")
	return cmd
}

func parseCommandArgs(args []string) ([]string, error) {
	if len(args) == 1 {
		parsed, err := shellwords.Split(args[0])
		if err != nil {
			return nil, fmt.Errorf("failed to parse command: %w", err)
		}
		return parsed, nil
	}
	return args, nil
}

func runHandler(cmd *cobra.Command, args []string) error {
	cluster, service, err := parseServiceFlag()
	if err != nil {
		return err
	}

	// Set up context that cancels on interrupt signal for proper Ctrl+C handling
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	profile := rootCmd.Flag("profile").Value.String()
	clients, err := ecs.NewAWSClients(ctx, profile)
	if err != nil {
		return fmt.Errorf("failed to initialize AWS clients: %w", err)
	}

	execWait, err := cmd.Flags().GetBool("wait")
	if err != nil {
		return fmt.Errorf("failed to get wait flag: %w", err)
	}

	dockerImageTag, err := cmd.Flags().GetString("image-tag")
	if err != nil {
		return fmt.Errorf("failed to get image-tag flag: %w", err)
	}

	parsedArgs, err := parseCommandArgs(args)
	if err != nil {
		return fmt.Errorf("error parsing command arguments: %w", err)
	}

	result, err := ecs.Execute(ctx, clients, cluster, service, parsedArgs, execWait, dockerImageTag)
	if err != nil {
		return err
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
	if execWait {
		for _, logEntry := range result.Logs {
			fmt.Println(logEntry.StreamName, logEntry.Message)
		}

		if result.Finished {
			fmt.Printf("Task %s finished\n", result.TaskArn)
		}
	}
	return nil
}

func init() {
	rootCmd.AddCommand(newRunCommand())
}
