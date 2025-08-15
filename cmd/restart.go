package cmd

import (
	"fmt"
	"log"

	"github.com/dustin/go-humanize"
	"github.com/spf13/cobra"
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

	svc := initService()
	result, err := svc.Restart(kill)
	if err != nil {
		log.Fatalf("Restart failed: %v\n", err)
	}

	if result.Method == "kill" {
		for _, stoppedTask := range result.StoppedTasks {
			fmt.Printf("Stopped task %s started %s\n", stoppedTask.TaskArn, humanize.Time(stoppedTask.StartedAt))
		}
	} else {
		fmt.Printf("Service %s restarted by starting new tasks using task definition %s.\n", svc.Service, result.TaskDefinition)
	}

	fmt.Println("Done.")
}

func init() {
	rootCmd.AddCommand(newRestartCommand())
}
