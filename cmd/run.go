// ABOUTME: Command-line interface for running ECS tasks
// ABOUTME: Handles user input and output formatting for task execution

package cmd

import (
	"fmt"
	"log"

	"github.com/spf13/cobra"
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

	cmd.PersistentFlags().BoolVarP(&execWait, "wait", "w", false, "wait for the task to finish")
	cmd.PersistentFlags().StringVarP(&dockerImageTag, "image-tag", "i", "", "docker image tag")
	return cmd
}

func runHandler(dockerImageTag *string, execWait *bool) func(*cobra.Command, []string) {
	return func(cmd *cobra.Command, args []string) {
		svc := initService()

		result, err := svc.Execute(args, *execWait, *dockerImageTag)
		if err != nil {
			log.Fatalln(err)
		}

		// Display service loading information
		fmt.Printf("Service '%s' loaded.\n", result.ServiceName)

		// Display task definition information
		if result.NewTaskDefCreated {
			fmt.Printf("New task definition %s is created\n", result.TaskDefinition)
		} else {
			fmt.Printf("The task definition %s is used\n", result.TaskDefinition)
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
				fmt.Printf("task %s finished\n", result.TaskArn)
			}
		}
	}
}

func init() {
	rootCmd.AddCommand(newRunCommand())
}
