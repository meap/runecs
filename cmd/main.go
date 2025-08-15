// Copyright 2021 Petr Reichl. All rights reserved.
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

// Package herrors contains common Hugo errors and error related utilities.
package cmd

import (
	"errors"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/go-playground/validator"
	"github.com/spf13/cobra"
	"runecs.io/v1/pkg/ecs"
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
		for _, c := range commandsWithoutService {
			if cmd.Name() == c {
				serviceRequired = false

				break
			}
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
}

func initService() *ecs.Service {
	svc := ecs.Service{}
	serviceValue := rootCmd.Flag("service").Value.String()

	parsed := strings.Split(serviceValue, "/")
	if len(parsed) == 2 {
		svc.Cluster = parsed[0]
		svc.Service = parsed[1]
	} else if serviceValue != "" {
		log.Fatalf("Invalid service name %s\n", serviceValue)
	}

	validate := validator.New()
	if err := validate.Struct(&svc); err != nil {
		log.Fatalf("Missing configuration properties %v\n", err)
	}

	return &svc
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
