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

package ecs

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/jinzhu/copier"
	"runecs.io/v1/internal/utils"
)

func cloneTaskDef(ctx context.Context, cluster, service, dockerImageTag string, svc *ecs.Client) (string, error) {
	// Get the last task definition ARN.
	// Load the latest task definition.
	latestDef, err := latestTaskDefinitionArn(ctx, cluster, service, svc)
	if err != nil {
		return "", err
	}

	response, err := svc.DescribeTaskDefinition(ctx, &ecs.DescribeTaskDefinitionInput{
		TaskDefinition: &latestDef,
	})

	if err != nil {
		return "", err
	}

	if len(response.TaskDefinition.ContainerDefinitions) > 1 {
		return "", errors.New("multiple container definitions in a single task are not supported")
	}

	containerDef, err := utils.SafeGetFirstPtr(response.TaskDefinition.ContainerDefinitions, "no container definitions found")
	if err != nil {
		return "", fmt.Errorf("failed to get container definition: %w", err)
	}

	newDef := &ecs.RegisterTaskDefinitionInput{}
	err = copier.Copy(newDef, response.TaskDefinition)
	if err != nil {
		return "", err
	}

	if containerDef.Image == nil {
		return "", errors.New("container definition has no image specified")
	}

	split := strings.Split(*containerDef.Image, ":")
	newDockerURI := fmt.Sprintf("%s:%s", split[0], dockerImageTag)

	newDef.ContainerDefinitions[0].Image = &newDockerURI

	output, err := svc.RegisterTaskDefinition(ctx, newDef)
	if err != nil {
		return "", err
	}

	if output.TaskDefinition == nil || output.TaskDefinition.TaskDefinitionArn == nil {
		return "", errors.New("invalid task definition response: missing ARN")
	}

	return *output.TaskDefinition.TaskDefinitionArn, nil
}

func Deploy(ctx context.Context, clients *AWSClients, cluster, service, dockerImageTag string) (*DeployResult, error) {
	// Clones the latest version of the task definition and inserts the new Docker URI.
	TaskDefinitionArn, err := cloneTaskDef(ctx, cluster, service, dockerImageTag, clients.ECS)
	if err != nil {
		return nil, fmt.Errorf("failed to clone task definition: %w", err)
	}

	updateOutput, err := clients.ECS.UpdateService(ctx, &ecs.UpdateServiceInput{
		Cluster:        &cluster,
		Service:        &service,
		TaskDefinition: &TaskDefinitionArn,
	})

	if err != nil {
		return nil, fmt.Errorf("failed to update service: %w", err)
	}

	if updateOutput.Service == nil || updateOutput.Service.ServiceArn == nil {
		return nil, errors.New("invalid service update response: missing service ARN")
	}

	return &DeployResult{
		TaskDefinitionArn: TaskDefinitionArn,
		ServiceArn:        *updateOutput.Service.ServiceArn,
	}, nil
}
