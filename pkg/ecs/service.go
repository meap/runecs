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
package ecs

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/retry"
	"github.com/aws/aws-sdk-go-v2/config"
)

// ECS parameters that are used to run jobs.
type Service struct {
	AwsRegion  string `yaml:"application" validate:"required"`
	AwsProfile string `yaml:"application" validate:"required"`
	Cluster    string `yaml:"application" validate:"required"`
	Service    string `yaml:"application" validate:"required"`
}

func (s *Service) initCfg() (aws.Config, error) {
	cfg, err := config.LoadDefaultConfig(context.TODO(),
		config.WithDefaultRegion(s.AwsRegion),
		config.WithSharedConfigProfile(s.AwsProfile),
		config.WithRetryer(func() aws.Retryer {
			return retry.AddWithMaxAttempts(retry.NewStandard(), 10)
		}),
	)

	if err != nil {
		return aws.Config{}, err
	}

	return cfg, nil
}
