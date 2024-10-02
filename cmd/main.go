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
	var dockerImageTag string

	/////////
	// RUN //
	/////////

	var execWait bool

	runCmd := &cobra.Command{
		Use:                   "run [cmd]",
		Short:                 "Execute a one-off process in an AWS ECS cluster",
		Args:                  cobra.MinimumNArgs(1),
		DisableFlagsInUseLine: true,
		Run: func(cmd *cobra.Command, args []string) {
			svc := initService()
			svc.Execute(args, execWait, dockerImageTag)
		},
	}

	runCmd.PersistentFlags().BoolVarP(&execWait, "wait", "w", false, "wait for the task to finish")
	runCmd.PersistentFlags().StringVarP(&dockerImageTag, "image-tag", "i", "", "docker image tag")
	rootCmd.AddCommand(runCmd)

	////////////////
	// PRUNE      //
	////////////////

	pruneCmd := &cobra.Command{
		Use:                   "prune",
		Short:                 "Deregister active task definitions",
		DisableFlagsInUseLine: true,
		Run: func(cmd *cobra.Command, args []string) {
			keepLastNr, _ := cmd.Flags().GetInt("keep-last")
			keepDays, _ := cmd.Flags().GetInt("keep-days")
			dryRun, _ := cmd.Flags().GetBool("dry-run")

			svc := initService()
			svc.Prune(keepLastNr, keepDays, dryRun)
		},
	}

	pruneCmd.PersistentFlags().BoolP("dry-run", "", false, "dry run")
	pruneCmd.PersistentFlags().IntP("keep-last", "", defaultLastNumberOfTasks, "keep last N task definitions")
	pruneCmd.PersistentFlags().IntP("keep-days", "", defaultLastDays, "keep task definitions older than N days")
	rootCmd.AddCommand(pruneCmd)

	////////////
	// DEPLOY //
	////////////

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

	///////////////
	// REVISIONS //
	///////////////

	revisionsCmd := &cobra.Command{
		Use:                   "revisions",
		Short:                 "List of active task definitions",
		DisableFlagsInUseLine: true,
		Run: func(cmd *cobra.Command, args []string) {
			revNr, _ := cmd.Flags().GetInt("last")

			svc := initService()
			svc.Revisions(revNr)
		},
	}

	revisionsCmd.PersistentFlags().IntP("last", "", 0, "last N revisions")
	rootCmd.AddCommand(revisionsCmd)

	///////////////
	// RESTART   //
	///////////////

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

	///////////////
	// LIST      //
	///////////////

	listCmd := &cobra.Command{
		Use:                   "list",
		Short:                 "List of all services across clusters in the current region",
		DisableFlagsInUseLine: true,
		Run: func(cmd *cobra.Command, args []string) {
			ecs.List()
		},
	}

	rootCmd.AddCommand(listCmd)

	////////////////
	// COMPLETION //
	////////////////

	completionCmd := &cobra.Command{
		Use:                   "completion [bash|zsh|fish|powershell]",
		Short:                 "Generate shell completion scripts",
		DisableFlagsInUseLine: true,
		ValidArgs:             []string{"bash", "zsh", "fish", "powershell"},
		Args:                  cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs),
		Run: func(cmd *cobra.Command, args []string) {
			var err error

			switch args[0] {
			case "bash":
				err = cmd.Root().GenBashCompletion(os.Stdout)
			case "zsh":
				err = cmd.Root().GenZshCompletion(os.Stdout)
			case "fish":
				err = cmd.Root().GenFishCompletion(os.Stdout, true)
			case "powershell":
				err = cmd.Root().GenPowerShellCompletion(os.Stdout)
			}

			if err != nil {
				log.Fatalf("failed to generate completion script: %v\n", err)
			}
		},
	}

	rootCmd.AddCommand(completionCmd)

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
