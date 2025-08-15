package cmd

import (
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
	svc.Restart(kill)
}

func init() {
	rootCmd.AddCommand(newRestartCommand())
}
