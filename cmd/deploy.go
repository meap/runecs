package cmd

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
	"runecs.io/v1/pkg/ecs"
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
		// Set up context that cancels on interrupt signal for cancellable deploy operations
		ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer cancel()

		clients, err := ecs.NewAWSClients()
		if err != nil {
			log.Fatalf("Failed to initialize AWS clients: %v\n", err)
		}

		cluster, service := parseServiceFlag()
		result, err := ecs.Deploy(ctx, clients, cluster, service, *dockerImageTag)
		if err != nil {
			log.Fatalf("Deploy failed: %v\n", err)
		}

		fmt.Printf("New task revision %s has been created\n", result.TaskDefinitionArn)
		fmt.Printf("Service %s has been updated.\n", result.ServiceArn)
	}
}

func init() {
	rootCmd.AddCommand(newDeployCommand())
}
