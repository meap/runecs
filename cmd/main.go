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
	"errors"
	"fmt"
	"log"
	"os"
	"slices"
	"strings"

	"github.com/spf13/cobra"
)

const (
	defaultLastNumberOfTasks = 50
	defaultLastDays          = 0
)

var rootCmd = &cobra.Command{
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		commandsWithoutService := []string{"completion", "help", "list", "version"}
		serviceValue := cmd.Flag("service").Value.String()

		serviceRequired := true
		if slices.Contains(commandsWithoutService, cmd.Name()) {
			serviceRequired = false
		}

		if serviceRequired && serviceValue == "" {
			return errors.New("--service flag is required for this command")
		}

		return nil
	},
}

func init() {
	rootCmd.CompletionOptions.DisableDefaultCmd = true
	rootCmd.PersistentFlags().String("service", "", "service name (cluster/service)")
	rootCmd.PersistentFlags().String("profile", "", "AWS profile to use for credentials")
}

func parseServiceFlag() (cluster, service string) {
	serviceValue := rootCmd.Flag("service").Value.String()

	parsed := strings.Split(serviceValue, "/")
	if len(parsed) == 2 {
		cluster = parsed[0]
		service = parsed[1]
	} else if serviceValue != "" {
		log.Fatalf("Invalid service name %s\n", serviceValue)
	}

	if cluster == "" || service == "" {
		log.Fatalf("Missing cluster or service configuration\n")
	}

	return cluster, service
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
