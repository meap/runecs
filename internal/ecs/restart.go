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

	"github.com/aws/aws-sdk-go-v2/service/ecs"
)

func stopAll(ctx context.Context, cluster, service string, client *ecs.Client) ([]StoppedTaskInfo, error) {
	tasks, err := client.ListTasks(ctx, &ecs.ListTasksInput{
		Cluster:     &cluster,
		ServiceName: &service,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list tasks: %w", err)
	}

	var stoppedTasks []StoppedTaskInfo
	for _, taskArn := range tasks.TaskArns {
		output, err := client.StopTask(ctx, &ecs.StopTaskInput{
			Cluster: &cluster,
			Task:    &taskArn,
		})
		if err != nil {
			// Continue with other tasks but log this error
			continue
		}

		if output.Task == nil {
			continue
		}

		if output.Task.TaskArn == nil || output.Task.StartedAt == nil {
			continue
		}

		stoppedTasks = append(stoppedTasks, StoppedTaskInfo{
			TaskArn:   *output.Task.TaskArn,
			StartedAt: *output.Task.StartedAt,
		})
	}

	return stoppedTasks, nil
}

func forceNewDeploy(ctx context.Context, cluster, service string, client *ecs.Client) (string, string, error) {
	taskDef, err := latestTaskDefinitionArn(ctx, cluster, service, client)
	if err != nil {
		return "", "", err
	}

	output, err := client.UpdateService(ctx, &ecs.UpdateServiceInput{
		Cluster:            &cluster,
		Service:            &service,
		TaskDefinition:     &taskDef,
		ForceNewDeployment: true,
	})

	if err != nil {
		return "", "", fmt.Errorf("failed to update service: %w", err)
	}

	if output.Service == nil {
		return "", "", errors.New("invalid service update response: missing service")
	}

	if output.Service.ServiceArn == nil {
		return "", "", errors.New("invalid service update response: missing service ARN")
	}

	if output.Service.TaskDefinition == nil {
		return "", "", errors.New("invalid service update response: missing task definition")
	}

	return *output.Service.ServiceArn, *output.Service.TaskDefinition, nil
}

func Restart(ctx context.Context, clients *AWSClients, cluster, service string, kill bool) (*RestartResult, error) {
	result := &RestartResult{}

	if kill {
		result.Method = "kill"

		stoppedTasks, err := stopAll(ctx, cluster, service, clients.ECS)
		if err != nil {
			return nil, fmt.Errorf("failed to stop tasks: %w", err)
		}
		result.StoppedTasks = stoppedTasks
	} else {
		result.Method = "force_deploy"

		serviceArn, taskDefinition, err := forceNewDeploy(ctx, cluster, service, clients.ECS)
		if err != nil {
			return nil, fmt.Errorf("failed to force new deployment: %w", err)
		}
		result.ServiceArn = serviceArn
		result.TaskDefinition = taskDefinition
	}

	return result, nil
}
