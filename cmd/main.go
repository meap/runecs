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

	"github.com/go-playground/validator"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"runecs.io/v1/pkg/ecs"
)

var (
	profile string
	verbose bool
)

var rootCmd = &cobra.Command{}

func init() {
	rootCmd.PersistentFlags().StringVarP(&profile, "profile", "p", "", "profile name with ECS cluster settings")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "", false, "verbose output")
	rootCmd.PersistentFlags().String("cluster", "", "ECS cluster name")
	rootCmd.PersistentFlags().String("service", "", "ECS service name")
	rootCmd.CompletionOptions.DisableDefaultCmd = true

	cobra.OnInitialize(initConfig)

	var dockerImageUri string
	var dryRun bool

	/////////
	// RUN //
	/////////

	var execWait bool

	runCmd := &cobra.Command{
		Use:   "run [cmd]",
		Short: "Execute a one-off process in an AWS ECS cluster",
		Args:  cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			svc := initService()
			svc.Execute(args, execWait, dockerImageUri)
		},
	}

	runCmd.PersistentFlags().BoolVarP(&execWait, "wait", "w", false, "wait for the task to finish")
	runCmd.PersistentFlags().StringVarP(&dockerImageUri, "image-uri", "i", "", "new docker image uri")
	rootCmd.AddCommand(runCmd)

	////////////////
	// PRUNE //
	////////////////

	var pruneKeepLast int
	var pruneKeepDays int

	pruneCmd := &cobra.Command{
		Use:   "prune",
		Short: "Mark task definitions as inactive",
		Run: func(cmd *cobra.Command, args []string) {
			svc := initService()
			svc.Prune(pruneKeepLast, pruneKeepDays, dryRun)
		},
	}

	pruneCmd.PersistentFlags().BoolVarP(&dryRun, "dry-run", "", false, "dry run")
	pruneCmd.PersistentFlags().IntVarP(&pruneKeepLast, "keep-last", "", 50, "keep last N task definitions")
	pruneCmd.PersistentFlags().IntVarP(&pruneKeepDays, "keep-days", "", 5, "keep task definitions older than N days")
	rootCmd.AddCommand(pruneCmd)

	////////////
	// DEPLOY //
	////////////

	deployCmd := &cobra.Command{
		Use:   "deploy",
		Short: "Deploy a new version of the task",
		Run: func(cmd *cobra.Command, args []string) {
			svc := initService()
			svc.Deploy(dockerImageUri)
		},
	}

	deployCmd.PersistentFlags().StringVarP(&dockerImageUri, "image-uri", "i", "", "new docker image uri")
	rootCmd.AddCommand(deployCmd)
}

func initService() *ecs.Service {
	svc := ecs.Service{}
	viper.Unmarshal(&svc)

	validate := validator.New()
	if err := validate.Struct(&svc); err != nil {
		log.Fatalf("Missing configuration properties %v\n", err)
	}

	return &svc
}

func initConfig() {
	if profile == "" {
		viper.AutomaticEnv()
		viper.BindEnv("AWS_REGION")
		viper.BindEnv("AWS_PROFILE")

		viper.BindPFlag("CLUSTER", rootCmd.Flags().Lookup("cluster"))
		viper.BindPFlag("SERVICE", rootCmd.Flags().Lookup("service"))

		return
	}

	viper.AddConfigPath("$HOME/.runecs/profiles")
	viper.AddConfigPath("./profiles")

	viper.SetConfigName(fmt.Sprintf("%s.yml", profile))
	viper.SetConfigType("yml")

	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err != nil {
		fmt.Println("Using config file:", viper.ConfigFileUsed())
	}
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
