package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
	"runecs.io/v1/internal/ecs"
)

func newDeployCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:                   "deploy",
		Short:                 "Deploy a new version of the task",
		DisableFlagsInUseLine: true,
		PreRunE:               deployPreRunE,
		RunE:                  deployHandler,
	}

	cmd.PersistentFlags().StringP("image-tag", "i", "", "docker image tag")

	return cmd
}

func deployPreRunE(cmd *cobra.Command, args []string) error {
	dockerImageTag, err := cmd.Flags().GetString("image-tag")
	if err != nil {
		return fmt.Errorf("failed to get image-tag flag: %w", err)
	}

	if dockerImageTag == "" {
		return errors.New("--image-tag flag is required")
	}

	return nil
}

func deployHandler(cmd *cobra.Command, args []string) error {
	// Set up context that cancels on interrupt signal for cancellable deploy operations
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	profile := rootCmd.Flag("profile").Value.String()

	clients, err := ecs.NewAWSClients(ctx, profile)

	if err != nil {
		return fmt.Errorf("failed to initialize AWS clients: %w", err)
	}

	dockerImageTag, err := cmd.Flags().GetString("image-tag")
	if err != nil {
		return fmt.Errorf("failed to get image-tag flag: %w", err)
	}

	cluster, service, err := parseServiceFlag()
	if err != nil {
		return err
	}

	result, err := ecs.Deploy(ctx, clients, cluster, service, dockerImageTag)

	if err != nil {
		return fmt.Errorf("deploy failed: %w", err)
	}

	cmd.Printf("New task revision %s has been created\n", result.TaskDefinitionArn)
	cmd.Printf("Service %s has been updated.\n", result.ServiceArn)

	return nil
}

func init() {
	rootCmd.AddCommand(newDeployCommand())
}
