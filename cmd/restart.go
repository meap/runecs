package cmd

import (
	"github.com/spf13/cobra"
)

func init() {
	restartCmd := &cobra.Command{
		Use:                   "restart",
		Short:                 "Restart the service",
		DisableFlagsInUseLine: true,
		Run: func(cmd *cobra.Command, args []string) {
			kill, _ := cmd.Flags().GetBool("kill")

			svc := initService()
			svc.Restart(kill)
		},
	}

	restartCmd.PersistentFlags().BoolP("kill", "", false, "Stops running tasks, ECS starts a new one if the health check is properly set")
	rootCmd.AddCommand(restartCmd)
}
