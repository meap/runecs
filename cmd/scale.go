// Copyright (c) Petr Reichl and affiliates. All rights reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
	"runecs.io/v1/internal/ecs"
)

func newScaleCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:                   "scale <value>",
		Short:                 "Scale the number of running tasks for a service",
		Args:                  cobra.ExactArgs(1),
		DisableFlagsInUseLine: true,
		PreRunE:               scalePreRunE,
		RunE:                  scaleHandler,
	}

	return cmd
}

func scalePreRunE(cmd *cobra.Command, args []string) error {
	if len(args) != 1 {
		return errors.New("scale command requires exactly one argument: the desired task count")
	}

	value64, err := strconv.ParseInt(args[0], 10, 32)
	if err != nil {
		return fmt.Errorf("invalid scale value: %w", err)
	}
	value := int(value64)

	const minTaskCount = 1

	const maxTaskCount = 1000

	if value < minTaskCount || value > maxTaskCount {
		return fmt.Errorf("scale value must be between %d and %d", minTaskCount, maxTaskCount)
	}

	return nil
}

func scaleHandler(cmd *cobra.Command, args []string) error {
	value64, err := strconv.ParseInt(args[0], 10, 32)
	if err != nil {
		return fmt.Errorf("invalid scale value: %w", err)
	}
	value := int32(value64)

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

	result, err := ecs.Scale(ctx, clients, cluster, service, value)
	if err != nil {
		return fmt.Errorf("failed to scale service: %w", err)
	}

	// Create lipgloss style for service name formatting
	boldStyle := lipgloss.NewStyle().Bold(true)

	fmt.Printf("Service %s scaled from %d to %d tasks\n",
		boldStyle.Render(fmt.Sprintf("%s/%s", result.ClusterName, result.ServiceName)),
		result.PreviousDesiredCount, result.NewDesiredCount)

	return nil
}

func init() {
	rootCmd.AddCommand(newScaleCommand())
}
