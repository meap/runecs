package cmd

import (
	"errors"

	"github.com/spf13/cobra"
)

func init() {
	var dockerImageTag string

	deployCmd := &cobra.Command{
		Use:                   "deploy",
		Short:                 "Deploy a new version of the task",
		DisableFlagsInUseLine: true,
		PreRunE: func(cmd *cobra.Command, args []string) error {
			if dockerImageTag == "" {
				return errors.New("--image-tag flag is required")
			}

			return nil
		},
		Run: func(cmd *cobra.Command, args []string) {
			svc := initService()
			svc.Deploy(dockerImageTag)
		},
	}

	deployCmd.PersistentFlags().StringVarP(&dockerImageTag, "image-tag", "i", "", "docker image tag")
	rootCmd.AddCommand(deployCmd)
}
