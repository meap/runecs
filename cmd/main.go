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

var runCmd = &cobra.Command{
	Use:   "run [cmd]",
	Short: "Execute a one-off process in an AWS ECS cluster.",
	Args:  cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		svc := initService()
		svc.Execute(args)
	},
}

var deregisterCmd = &cobra.Command{
	Use:   "deregister",
	Short: "Unregisters all inactive task definitions.",
	Run: func(cmd *cobra.Command, args []string) {
		svc := initService()
		svc.Deregister()
	},
}

var rootCmd = &cobra.Command{}

func init() {
	cobra.OnInitialize(initConfig)

	rootCmd.PersistentFlags().StringVarP(&profile, "profile", "p", "default", "profile name with ECS cluster settings")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "", false, "verbose output")
	rootCmd.CompletionOptions.DisableDefaultCmd = true

	rootCmd.AddCommand(runCmd)
	rootCmd.AddCommand(deregisterCmd)
}

func initService() *ecs.Service {
	svc := ecs.Service{}
	viper.UnmarshalKey(fmt.Sprintf("Profiles.%s", profile), &svc)

	validate := validator.New()
	if err := validate.Struct(&svc); err != nil {
		log.Fatalf("Missing configuration properties %v\n", err)
	}

	return &svc
}

func initConfig() {
	homeDir, _ := os.UserHomeDir()

	viper.AddConfigPath(homeDir)
	viper.AddConfigPath(".")
	viper.SetConfigName(".runecs.yml")
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
