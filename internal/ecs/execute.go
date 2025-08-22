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
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"runecs.io/v1/internal/utils"
)

func describeTask(ctx context.Context, client *ecs.Client, taskArn *string) (TaskDefinition, error) {
	resp, err := client.DescribeTaskDefinition(ctx, &ecs.DescribeTaskDefinitionInput{TaskDefinition: taskArn})
	if err != nil {
		return TaskDefinition{}, err
	}

	logGroup, logStreamPrefix, containerName, err := getLogStreamPrefix(ctx, client, *taskArn)
	if err != nil {
		return TaskDefinition{}, err
	}

	output := TaskDefinition{}
	output.Name = containerName
	output.LogGroup = logGroup
	output.LogStreamPrefix = logStreamPrefix

	if resp.TaskDefinition.Cpu == nil {
		return TaskDefinition{}, fmt.Errorf("task definition has no CPU specification: %s", *taskArn)
	}
	if resp.TaskDefinition.Memory == nil {
		return TaskDefinition{}, fmt.Errorf("task definition has no memory specification: %s", *taskArn)
	}

	output.Cpu = *resp.TaskDefinition.Cpu
	output.Memory = *resp.TaskDefinition.Memory

	// Extract RequiresCompatibilities - required field
	if len(resp.TaskDefinition.RequiresCompatibilities) == 0 {
		return TaskDefinition{}, fmt.Errorf("task definition has no compatibility requirements: %s", *taskArn)
	}
	compatibilities := make([]string, len(resp.TaskDefinition.RequiresCompatibilities))
	for i, compat := range resp.TaskDefinition.RequiresCompatibilities {
		compatibilities[i] = string(compat)
	}
	output.RequiresCompatibilities = compatibilities

	return output, nil
}

func checkTaskStatus(ctx context.Context, cluster string, client *ecs.Client, task string) (bool, error) {
	output, err := client.DescribeTasks(ctx, &ecs.DescribeTasksInput{
		Cluster: &cluster,
		Tasks:   []string{task},
	})

	if err != nil {
		return false, err
	}

	taskInfo, err := utils.SafeGetFirstPtr(output.Tasks, "no tasks found in response")
	if err != nil {
		return false, fmt.Errorf("failed to get task information: %w", err)
	}

	if taskInfo.LastStatus == nil {
		return false, errors.New("task has no status information")
	}

	if *taskInfo.LastStatus == "STOPPED" {
		container, err := utils.SafeGetFirstPtr(taskInfo.Containers, "no containers found in task")
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

// waitForTaskCompletion waits for an ECS task to complete and collects its logs
func waitForTaskCompletion(ctx context.Context, clients *AWSClients, cluster string, taskArn string, tdef TaskDefinition, result *ExecuteResult) error {
	// Get the account ID using STS
	identity, err := clients.STS.GetCallerIdentity(ctx, &sts.GetCallerIdentityInput{})
	if err != nil {
		return fmt.Errorf("failed to get caller identity: %w", err)
	}

	// Extract partition from caller's ARN to handle different AWS partitions
	partition, err := extractPartitionFromARN(*identity.Arn)
	if err != nil {
		return fmt.Errorf("failed to extract partition from caller ARN: %w", err)
	}

	// Construct the LogGroup ARN with correct partition
	logGroupArn := buildARN(partition, "logs", clients.Region, *identity.Account, fmt.Sprintf("log-group:%s", tdef.LogGroup))

	// Extract task ID from task ARN for log stream pattern
	taskID, err := extractARNResource(taskArn)
	if err != nil {
		return fmt.Errorf("failed to extract task ID from ARN: %w", err)
	}

	// Build log stream prefix pattern
	logStreamPrefixPattern := fmt.Sprintf("%s/%s/%s", tdef.LogStreamPrefix, tdef.Name, taskID)

	// Start tailing logs
	logChan, closeFunc, err := TailLogGroups(ctx, clients.CloudWatchLogs, []string{logGroupArn}, []string{logStreamPrefixPattern})
	if err != nil {
		return fmt.Errorf("failed to start log tailing: %w", err)
	}
	defer closeFunc()

	// Inform user about cancellation option
	fmt.Println("Waiting for task to complete and streaming logs... Press CTRL+C to stop waiting (task will continue running).")

	// Create a context that can be cancelled when task completes
	taskCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Start goroutine to collect logs
	logsDone := make(chan struct{})

	go func() {
		defer close(logsDone)
		for {
			select {
			case <-taskCtx.Done():
				return
			case logEntry, ok := <-logChan:
				if !ok {
					return
				}
				result.Logs = append(result.Logs, logEntry)
			}
		}
	}()

	// Poll task status
	for {
		// Check for context cancellation before each iteration
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		success, err := checkTaskStatus(ctx, cluster, clients.ECS, taskArn)
		if err != nil {
			return err
		}

		if success {
			result.Finished = true
			// Cancel the log collection context and wait for it to finish
			cancel()
			<-logsDone

			break
		}

		// Context-aware sleep to allow immediate cancellation
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(5 * time.Second):
			// Continue polling
		}
	}

	return nil
}

func Execute(ctx context.Context, clients *AWSClients, cluster, service string, cmd []string, waitForCompletion bool, dockerImageTag string) (*ExecuteResult, error) {
	// Describe the service to get its configuration
	resp, err := clients.ECS.DescribeServices(ctx, &ecs.DescribeServicesInput{
		Cluster:  &cluster,
		Services: []string{service},
	})
	if err != nil {
		return nil, fmt.Errorf("error describing service %s in cluster %s: %w", service, cluster, err)
	}

	serviceInfo, err := utils.SafeGetFirstPtr(resp.Services, "no services found in response")
	if err != nil {
		return nil, fmt.Errorf("failed to get service information: %w", err)
	}

	// Fetch the latest task definition
	latestTaskDefArn, err := latestTaskDefinitionArn(ctx, cluster, service, clients.ECS)
	if err != nil {
		return nil, fmt.Errorf("error getting task definition for service %s: %w", service, err)
	}

	// Extract network configuration if available
	taskDefArn := latestTaskDefArn

	var subnets []string

	var securityGroups []string

	if serviceInfo.NetworkConfiguration != nil && serviceInfo.NetworkConfiguration.AwsvpcConfiguration != nil {
		subnets = serviceInfo.NetworkConfiguration.AwsvpcConfiguration.Subnets
		securityGroups = serviceInfo.NetworkConfiguration.AwsvpcConfiguration.SecurityGroups
	}

	// Extract capacity provider strategy if available
	var capacityProviderStrategy []types.CapacityProviderStrategyItem
	if len(serviceInfo.CapacityProviderStrategy) > 0 {
		capacityProviderStrategy = serviceInfo.CapacityProviderStrategy
	}

	tdef, err := describeTask(ctx, clients.ECS, &taskDefArn)
	if err != nil {
		return nil, fmt.Errorf("error loading task definition %s: %w", taskDefArn, err)
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
		taskDef = taskDefArn
	}

	runTaskInput := &ecs.RunTaskInput{
		Cluster:        &cluster,
		TaskDefinition: &taskDef,
		Overrides: &types.TaskOverride{
			ContainerOverrides: []types.ContainerOverride{{
				Name:    &tdef.Name,
				Command: cmd,
			}},
		},
	}

	// Use CapacityProviderStrategy if available, otherwise use LaunchType
	if len(capacityProviderStrategy) > 0 {
		runTaskInput.CapacityProviderStrategy = capacityProviderStrategy
	} else {
		runTaskInput.LaunchType = types.LaunchType(tdef.RequiresCompatibilities[0])
	}

	// Only set NetworkConfiguration if service has network configuration
	if len(subnets) > 0 || len(securityGroups) > 0 {
		runTaskInput.NetworkConfiguration = &types.NetworkConfiguration{
			AwsvpcConfiguration: &types.AwsVpcConfiguration{
				Subnets:        subnets,
				SecurityGroups: securityGroups,
				AssignPublicIp: "ENABLED", // FIXME: Public IP is not needed (mostly)
			},
		}
	}

	output, err := clients.ECS.RunTask(ctx, runTaskInput)
	if err != nil {
		return nil, err
	}

	executedTask, err := utils.SafeGetFirstPtr(output.Tasks, "no tasks found in response")
	if err != nil {
		return nil, fmt.Errorf("failed to get executed task: %w", err)
	}

	if executedTask.TaskArn == nil {
		return nil, errors.New("executed task has no ARN")
	}

	result := &ExecuteResult{
		TaskDefinition:    taskDef,
		TaskArn:           *executedTask.TaskArn,
		NewTaskDefCreated: newTaskDefCreated,
		Finished:          false,
		Logs:              []LogEntry{},
	}

	if waitForCompletion {
		if err := waitForTaskCompletion(ctx, clients, cluster, *executedTask.TaskArn, tdef, result); err != nil {
			return result, err
		}
	}

	return result, nil
}
