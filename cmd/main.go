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
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/go-playground/validator"
	"github.com/spf13/cobra"
	"runecs.io/v1/pkg/ecs"
)

var service string

var rootCmd = &cobra.Command{
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		commandsWithoutService := []string{"completion", "help", "list", "version"}

		serviceRequired := true
		for _, c := range commandsWithoutService {
			if cmd.Name() == c {
				serviceRequired = false
				break
			}
		}

		if serviceRequired && service == "" {
			return fmt.Errorf("--service flag is required for this command")
		}

		return nil
	},
}

func init() {
	var dockerImageTag string
	var dryRun bool

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

	var pruneKeepLast int
	var pruneKeepDays int

	pruneCmd := &cobra.Command{
		Use:                   "prune",
		Short:                 "Deregister active task definitions",
		DisableFlagsInUseLine: true,
		Run: func(cmd *cobra.Command, args []string) {
			svc := initService()
			svc.Prune(pruneKeepLast, pruneKeepDays, dryRun)
		},
	}

	pruneCmd.PersistentFlags().BoolVarP(&dryRun, "dry-run", "", false, "dry run")
	pruneCmd.PersistentFlags().IntVarP(&pruneKeepLast, "keep-last", "", 50, "keep last N task definitions")
	pruneCmd.PersistentFlags().IntVarP(&pruneKeepDays, "keep-days", "", 0, "keep task definitions older than N days")
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
				return fmt.Errorf("--image-tag flag is required")
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

	var lastRevisionNr int

	revisionsCmd := &cobra.Command{
		Use:                   "revisions",
		Short:                 "List of active task definitions",
		DisableFlagsInUseLine: true,
		Run: func(cmd *cobra.Command, args []string) {
			svc := initService()
			svc.Revisions(lastRevisionNr)
		},
	}

	revisionsCmd.PersistentFlags().IntVarP(&lastRevisionNr, "last", "", 0, "last N revisions")
	rootCmd.AddCommand(revisionsCmd)

	///////////////
	// LIST      //
	///////////////

	listCmd := &cobra.Command{
		Use:                   "list",
		Short:                 "List of all services across clusters in the current region",
		DisableFlagsInUseLine: true,
		Run: func(cmd *cobra.Command, args []string) {
			svc := initService()
			svc.List()
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
			switch args[0] {
			case "bash":
				cmd.Root().GenBashCompletion(os.Stdout)
			case "zsh":
				cmd.Root().GenZshCompletion(os.Stdout)
			case "fish":
				cmd.Root().GenFishCompletion(os.Stdout, true)
			case "powershell":
				cmd.Root().GenPowerShellCompletion(os.Stdout)
			}
		},
	}

	rootCmd.AddCommand(completionCmd)

	rootCmd.CompletionOptions.DisableDefaultCmd = true
	rootCmd.PersistentFlags().StringVar(&service, "service", "", "service name (cluster/service)")
}

func initService() *ecs.Service {
	svc := ecs.Service{}

	parsed := strings.Split(service, "/")
	if len(parsed) == 2 {
		svc.Cluster = parsed[0]
		svc.Service = parsed[1]
	} else if service != "" {
		log.Fatalf("Invalid service name %s\n", service)
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
