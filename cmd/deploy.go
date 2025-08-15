package cmd

import (
	"errors"

	"github.com/spf13/cobra"
)

func newDeployCommand() *cobra.Command {
	var dockerImageTag string

	cmd := &cobra.Command{
		Use:                   "deploy",
		Short:                 "Deploy a new version of the task",
		DisableFlagsInUseLine: true,
		PreRunE:               deployPreRunE(&dockerImageTag),
		Run:                   deployHandler(&dockerImageTag),
	}

	cmd.PersistentFlags().StringVarP(&dockerImageTag, "image-tag", "i", "", "docker image tag")
	return cmd
}

func deployPreRunE(dockerImageTag *string) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, args []string) error {
		if *dockerImageTag == "" {
			return errors.New("--image-tag flag is required")
		}
		return nil
	}
}

func deployHandler(dockerImageTag *string) func(*cobra.Command, []string) {
	return func(cmd *cobra.Command, args []string) {
		svc := initService()
		svc.Deploy(*dockerImageTag)
	}
}

func init() {
	rootCmd.AddCommand(newDeployCommand())
}
