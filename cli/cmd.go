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
package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"runecs.io/v1/ecs"
)

var (
	profile string
	verbose bool
)

var rootCmd = &cobra.Command{
	Use:   "run",
	Short: "Runs the command as a new task in the ECS container.",
	Args:  cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		service := &ecs.Service{
			Region:  viper.GetString(fmt.Sprintf("Profiles.%s.AwsRegion", profile)),
			Profile: viper.GetString(fmt.Sprintf("Profiles.%s.AwsProfile", profile)),
			Cluster: viper.GetString(fmt.Sprintf("Profiles.%s.Cluster", profile)),
			Service: viper.GetString(fmt.Sprintf("Profiles.%s.Service", profile)),
		}

		service.Execute(args)
	},
}

func init() {
	cobra.OnInitialize(initConfig)

	rootCmd.PersistentFlags().StringVarP(&profile, "profile", "p", "Default", "Profile name that defines the ECS service settings.")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "", false, "Verbose output")
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
