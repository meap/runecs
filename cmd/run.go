package cmd

import (
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
		svc.Execute(args, *execWait, *dockerImageTag)
	}
}

func init() {
	rootCmd.AddCommand(newRunCommand())
}
