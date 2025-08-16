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
	"time"

	"github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/aws/aws-sdk-go-v2/service/ecs/types"
)

func describeService(ctx context.Context, cluster, service string, client *ecs.Client) (ServiceDefinition, error) {
	resp, err := client.DescribeServices(ctx, &ecs.DescribeServicesInput{
		Cluster:  &cluster,
		Services: []string{service},
	})

	if err != nil {
		return ServiceDefinition{}, err
	}

	serviceInfo, err := safeGetFirstPtr(resp.Services, "no services found in response")
	if err != nil {
		return ServiceDefinition{}, fmt.Errorf("failed to get service information: %w", err)
	}

	// Fetch the latest task definition. Keep in mind that the active service may
	// have a different task definition that is available, see. *serviceInfo.TaskDefinition
	taskDef, err := latestTaskDefinitionArn(ctx, cluster, service, client)
	if err != nil {
		return ServiceDefinition{}, err
	}

	// Check for required network configuration
	if serviceInfo.NetworkConfiguration == nil {
		return ServiceDefinition{}, errors.New("service has no network configuration")
	}
	if serviceInfo.NetworkConfiguration.AwsvpcConfiguration == nil {
		return ServiceDefinition{}, errors.New("service has no AWSVPC configuration")
	}

	return ServiceDefinition{
		Subnets:        serviceInfo.NetworkConfiguration.AwsvpcConfiguration.Subnets,
		SecurityGroups: serviceInfo.NetworkConfiguration.AwsvpcConfiguration.SecurityGroups,
		TaskDef:        taskDef,
	}, nil
}

func describeTask(ctx context.Context, client *ecs.Client, taskArn *string) (TaskDefinition, error) {
	resp, err := client.DescribeTaskDefinition(ctx, &ecs.DescribeTaskDefinitionInput{TaskDefinition: taskArn})
	if err != nil {
		return TaskDefinition{}, err
	}

	containerDef, err := safeGetFirstPtr(resp.TaskDefinition.ContainerDefinitions, "no container definitions found")
	if err != nil {
		return TaskDefinition{}, fmt.Errorf("failed to get container definition from task %s: %w", *taskArn, err)
	}

	if containerDef.Name == nil {
		return TaskDefinition{}, fmt.Errorf("container definition has no name in task %s", *taskArn)
	}

	output := TaskDefinition{}
	output.Name = *containerDef.Name

	logConfig := containerDef.LogConfiguration
	if logConfig != nil && logConfig.LogDriver == types.LogDriverAwslogs {
		output.LogGroup = logConfig.Options["awslogs-group"]
		output.LogStreamPrefix = logConfig.Options["awslogs-stream-prefix"]
	}

	if resp.TaskDefinition.Cpu == nil {
		return TaskDefinition{}, fmt.Errorf("task definition has no CPU specification: %s", *taskArn)
	}
	if resp.TaskDefinition.Memory == nil {
		return TaskDefinition{}, fmt.Errorf("task definition has no memory specification: %s", *taskArn)
	}

	output.Cpu = *resp.TaskDefinition.Cpu
	output.Memory = *resp.TaskDefinition.Memory

	return output, nil
}

func wait(ctx context.Context, cluster string, client *ecs.Client, task string) (bool, error) {
	output, err := client.DescribeTasks(ctx, &ecs.DescribeTasksInput{
		Cluster: &cluster,
		Tasks:   []string{task},
	})

	if err != nil {
		return false, err
	}

	taskInfo, err := safeGetFirstPtr(output.Tasks, "no tasks found in response")
	if err != nil {
		return false, fmt.Errorf("failed to get task information: %w", err)
	}

	if taskInfo.LastStatus == nil {
		return false, errors.New("task has no status information")
	}

	if *taskInfo.LastStatus == "STOPPED" {
		container, err := safeGetFirstPtr(taskInfo.Containers, "no containers found in task")
		if err != nil {
			return false, fmt.Errorf("failed to get container information: %w", err)
		}

		if container.ExitCode == nil {
			return false, errors.New("stopped container has no exit code")
		}

		if taskInfo.TaskArn == nil {
			return false, errors.New("task has no ARN")
		}

		if exitCode := *container.ExitCode; exitCode != 0 {
			return true, fmt.Errorf("task %s failed with exit code %d", *taskInfo.TaskArn, exitCode)
		}
	}

	return *taskInfo.LastStatus == "STOPPED", nil
}

func Execute(ctx context.Context, clients *AWSClients, cluster, service string, cmd []string, waitForCompletion bool, dockerImageTag string) (*ExecuteResult, error) {
	sdef, err := describeService(ctx, cluster, service, clients.ECS)
	if err != nil {
		return nil, fmt.Errorf("error loading service %s in cluster %s: %w", service, cluster, err)
	}

	tdef, err := describeTask(ctx, clients.ECS, &sdef.TaskDef)
	if err != nil {
		return nil, fmt.Errorf("error loading task definition %s: %w", sdef.TaskDef, err)
	}

	var taskDef string
	newTaskDefCreated := false

	if dockerImageTag != "" {
		taskDef, err = cloneTaskDef(ctx, cluster, service, dockerImageTag, clients.ECS)
		if err != nil {
			return nil, err
		}
		newTaskDefCreated = true
	} else {
		taskDef = sdef.TaskDef
	}

	output, err := clients.ECS.RunTask(ctx, &ecs.RunTaskInput{
		Cluster:        &cluster,
		TaskDefinition: &taskDef,
		LaunchType:     "FARGATE",
		NetworkConfiguration: &types.NetworkConfiguration{
			AwsvpcConfiguration: &types.AwsVpcConfiguration{
				Subnets:        sdef.Subnets,
				SecurityGroups: sdef.SecurityGroups,
				AssignPublicIp: "ENABLED", // FIXME: Public IP is not needed (mostly)
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
		return nil, err
	}

	executedTask, err := safeGetFirstPtr(output.Tasks, "no tasks found in response")
	if err != nil {
		return nil, fmt.Errorf("failed to get executed task: %w", err)
	}

	if executedTask.TaskArn == nil {
		return nil, errors.New("executed task has no ARN")
	}

	result := &ExecuteResult{
		ServiceName:       service,
		TaskDefinition:    taskDef,
		TaskArn:           *executedTask.TaskArn,
		NewTaskDefCreated: newTaskDefCreated,
		Finished:          false,
		Logs:              []LogEntry{},
	}

	if waitForCompletion {
		var lastTimestamp *int64 = nil

		for {
			// Check for context cancellation before each iteration
			select {
			case <-ctx.Done():
				return result, ctx.Err()
			default:
			}

			logs, newTimestamp, err := getTaskLogs(ctx, tdef.LogGroup, tdef.LogStreamPrefix, *executedTask.TaskArn, tdef.Name, lastTimestamp, clients.CloudWatchLogs)
			if err == nil {
				result.Logs = append(result.Logs, logs...)
				lastTimestamp = &newTimestamp
			}

			success, err := wait(ctx, cluster, clients.ECS, *executedTask.TaskArn)
			if err != nil {
				return result, err
			}

			if success {
				result.Finished = true
				break
			}

			// Context-aware sleep to allow immediate cancellation
			select {
			case <-ctx.Done():
				return result, ctx.Err()
			case <-time.After(5 * time.Second):
				// Continue polling
			}
		}
	}

	return result, nil
}
