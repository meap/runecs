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
	"log"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/aws/aws-sdk-go-v2/service/ecs/types"
)

// ECS parameters that are used to run jobs.
type Service struct {
	AwsRegion  string `yaml:"application" validate:"required"`
	AwsProfile string `yaml:"application" validate:"required"`
	Cluster    string `yaml:"application" validate:"required"`
	Service    string `yaml:"application" validate:"required"`
}

type serviceDef struct {
	Subnets        []string
	SecurityGroups []string
	TaskDef        string
}

type taskDef struct {
	Name string
}

func (s *Service) initCfg() (aws.Config, error) {
	cfg, err := config.LoadDefaultConfig(context.TODO(),
		config.WithDefaultRegion(s.AwsRegion),
		config.WithSharedConfigProfile(s.AwsProfile),
	)

	if err != nil {
		return aws.Config{}, err
	}

	return cfg, nil
}

func (s *Service) describeService(client *ecs.Client) (serviceDef, error) {
	resp, err := client.DescribeServices(context.TODO(), &ecs.DescribeServicesInput{
		Cluster:  &s.Cluster,
		Services: []string{s.Service},
	})

	if err != nil {
		return serviceDef{}, err
	}

	def := resp.Services[0]

	return serviceDef{
		Subnets:        def.NetworkConfiguration.AwsvpcConfiguration.Subnets,
		SecurityGroups: def.NetworkConfiguration.AwsvpcConfiguration.SecurityGroups,
		TaskDef:        *def.TaskDefinition,
	}, nil
}

func (s *Service) describeTask(client *ecs.Client, taskArn *string) taskDef {
	resp, _ := client.DescribeTaskDefinition(context.TODO(), &ecs.DescribeTaskDefinitionInput{TaskDefinition: taskArn})

	return taskDef{
		Name: *resp.TaskDefinition.ContainerDefinitions[0].Name,
	}
}

func (s *Service) Execute(cmd []string) {
	cfg, err := s.initCfg()
	if err != nil {
		log.Fatalln(err)
	}

	svc := ecs.NewFromConfig(cfg)

	sdef, err := s.describeService(svc)
	if err != nil {
		log.Fatalf("An error occurred while loading the service %s in the cluster %s: %v", s.Service, s.Cluster, err)
	}

	tdef := s.describeTask(svc, &sdef.TaskDef)

	output, err := svc.RunTask(context.TODO(), &ecs.RunTaskInput{
		Cluster:        &s.Cluster,
		TaskDefinition: &sdef.TaskDef,
		LaunchType:     "FARGATE",
		NetworkConfiguration: &types.NetworkConfiguration{
			AwsvpcConfiguration: &types.AwsVpcConfiguration{
				Subnets:        sdef.Subnets,
				SecurityGroups: sdef.SecurityGroups,
				AssignPublicIp: "ENABLED",
			},
		},
		Overrides: &types.TaskOverride{
			ContainerOverrides: []types.ContainerOverride{{
				Name:    &tdef.Name,
				Command: cmd,
			}},
		},
	})

	if err != nil {
		log.Fatalln(err)
	}

	log.Printf("Task %s executed.", *output.Tasks[0].TaskArn)
}
