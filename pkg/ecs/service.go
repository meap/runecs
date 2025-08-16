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
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/retry"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/aws/aws-sdk-go-v2/service/ecs/types"
	"github.com/jinzhu/copier"
)

const (
	defaultNumberOfRetries = 10
)

// Local types used by ECS functions
type ServiceDefinition struct {
	Subnets        []string
	SecurityGroups []string
	TaskDef        string
}

type TaskDefinition struct {
	Name            string
	LogGroup        string
	LogStreamPrefix string
	Cpu             string
	Memory          string
}

type LogEntry struct {
	StreamName string
	Message    string
	Timestamp  int64
}

type ExecuteResult struct {
	ServiceName       string
	TaskDefinition    string
	TaskArn           string
	NewTaskDefCreated bool
	Finished          bool
	Logs              []LogEntry
}

type RevisionEntry struct {
	Revision  int32
	CreatedAt time.Time
	DockerURI string
	Family    string
}

type RevisionsResult struct {
	Revisions []RevisionEntry
}

type DeployResult struct {
	TaskDefinitionArn string
	ServiceArn        string
}

type RestartResult struct {
	StoppedTasks   []StoppedTaskInfo
	ServiceArn     string
	TaskDefinition string
	Method         string // "kill" or "force_deploy"
}

type StoppedTaskInfo struct {
	TaskArn   string
	StartedAt time.Time
}

type TaskDefinitionPruneEntry struct {
	Arn     string
	DaysOld int
	Action  string // "kept", "deleted", "skipped"
	Reason  string
	Family  string
}

type PruneResult struct {
	Families       []string
	TotalCount     int
	DeletedCount   int
	KeptCount      int
	SkippedCount   int
	DryRun         bool
	ProcessedTasks []TaskDefinitionPruneEntry
}

// AWSClients holds initialized AWS service clients
type AWSClients struct {
	ECS            *ecs.Client
	CloudWatchLogs *cloudwatchlogs.Client
}

// =============================================================================
// Core Service Methods
// =============================================================================

// NewAWSClients creates and initializes AWS service clients with shared configuration
func NewAWSClients() (*AWSClients, error) {
	configFunctions := []func(*config.LoadOptions) error{
		config.WithRetryer(func() aws.Retryer {
			return retry.AddWithMaxAttempts(retry.NewStandard(), defaultNumberOfRetries)
		}),
	}

	cfg, err := config.LoadDefaultConfig(context.Background(), configFunctions...)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize AWS configuration: %w", err)
	}

	return &AWSClients{
		ECS:            ecs.NewFromConfig(cfg),
		CloudWatchLogs: cloudwatchlogs.NewFromConfig(cfg),
	}, nil
}

// =============================================================================
// Deploy Methods
// =============================================================================

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

	containerDef, err := safeGetFirstPtr(response.TaskDefinition.ContainerDefinitions, "no container definitions found")
	if err != nil {
		return "", fmt.Errorf("failed to get container definition: %w", err)
	}

	newDef := &ecs.RegisterTaskDefinitionInput{}
	if err := copier.Copy(newDef, response.TaskDefinition); err != nil {
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

// =============================================================================
// Execute Methods
// =============================================================================

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

			logs, newTimestamp, err := getProcessLogs(ctx, tdef.LogGroup, tdef.LogStreamPrefix, *executedTask.TaskArn, tdef.Name, lastTimestamp, clients.CloudWatchLogs)
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

// =============================================================================
// Logs Methods
// =============================================================================

func getProcessLogs(
	ctx context.Context, logGroupname string,
	logStreamPrefix string,
	taskArn string,
	name string,
	startTime *int64,
	client *cloudwatchlogs.Client) ([]LogEntry, int64, error) {

	processID, err := extractARNResource(taskArn)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to extract process ID from task ARN: %w", err)
	}

	input := &cloudwatchlogs.FilterLogEventsInput{
		LogGroupName:   aws.String(logGroupname),
		LogStreamNames: []string{fmt.Sprintf("%s/%s/%s", logStreamPrefix, name, processID)},
	}

	if startTime != nil {
		input.StartTime = startTime
	}

	output, err := client.FilterLogEvents(ctx, input)

	if err != nil {
		return nil, 0, fmt.Errorf("failed to filter log events (%w)", err)
	}

	var logs []LogEntry
	for _, event := range output.Events {
		if event.LogStreamName == nil || event.Message == nil || event.Timestamp == nil {
			continue // Skip events with missing required fields
		}
		logs = append(logs, LogEntry{
			StreamName: *event.LogStreamName,
			Message:    *event.Message,
			Timestamp:  *event.Timestamp,
		})
	}

	var lastEventTimestamp int64
	if len(output.Events) > 0 {
		lastEvent := output.Events[len(output.Events)-1]
		if lastEvent.Timestamp == nil {
			if startTime != nil {
				lastEventTimestamp = *startTime
			} else {
				lastEventTimestamp = 0
			}
		} else {
			lastEventTimestamp = *lastEvent.Timestamp + 1
		}
	} else if startTime != nil {
		lastEventTimestamp = *startTime
	} else {
		lastEventTimestamp = 0
	}

	return logs, lastEventTimestamp, nil
}

// =============================================================================
// Prune Methods
// =============================================================================

func deregisterTaskFamily(ctx context.Context, family string, keepLast int, keepDays int, dryRun bool, svc *ecs.Client) (int, int, int, []TaskDefinitionPruneEntry, error) {
	definitionInput := &ecs.ListTaskDefinitionsInput{
		FamilyPrefix: &family,
		Sort:         types.SortOrderDesc,
	}

	today := time.Now().UTC()
	totalCount := 0
	deleted := 0
	keep := 0
	var processedTasks []TaskDefinitionPruneEntry

	for {
		resp, err := svc.ListTaskDefinitions(ctx, definitionInput)
		if err != nil {
			return 0, 0, 0, nil, fmt.Errorf("loading the list of definitions failed: %w", err)
		}

		count := len(resp.TaskDefinitionArns)
		totalCount += count

		for _, def := range resp.TaskDefinitionArns {
			response, err := svc.DescribeTaskDefinition(ctx, &ecs.DescribeTaskDefinitionInput{
				TaskDefinition: &def,
			})

			if err != nil {
				processedTasks = append(processedTasks, TaskDefinitionPruneEntry{
					Arn:     def,
					DaysOld: -1,
					Action:  "skipped",
					Reason:  fmt.Sprintf("Failed to describe: %v", err),
					Family:  family,
				})
				continue
			}

			if response.TaskDefinition.RegisteredAt == nil {
				processedTasks = append(processedTasks, TaskDefinitionPruneEntry{
					Arn:     def,
					DaysOld: -1,
					Action:  "skipped",
					Reason:  "Missing registration date",
					Family:  family,
				})
				continue
			}

			diffInDays := int(today.Sub(*response.TaskDefinition.RegisteredAt).Hours() / 24)

			// Last X
			if keep < keepLast {
				processedTasks = append(processedTasks, TaskDefinitionPruneEntry{
					Arn:     def,
					DaysOld: diffInDays,
					Action:  "kept",
					Reason:  fmt.Sprintf("keeping last %d definitions", keepLast),
					Family:  family,
				})
				keep++
				continue
			}

			if diffInDays < keepDays {
				processedTasks = append(processedTasks, TaskDefinitionPruneEntry{
					Arn:     def,
					DaysOld: diffInDays,
					Action:  "kept",
					Reason:  fmt.Sprintf("newer than %d days", keepDays),
					Family:  family,
				})
				continue
			}

			deleted++

			if !dryRun {
				_, err := svc.DeregisterTaskDefinition(ctx, &ecs.DeregisterTaskDefinitionInput{TaskDefinition: &def})
				if err != nil {
					processedTasks = append(processedTasks, TaskDefinitionPruneEntry{
						Arn:     def,
						DaysOld: diffInDays,
						Action:  "skipped",
						Reason:  fmt.Sprintf("deregistration failed: %v", err),
						Family:  family,
					})
					deleted-- // Decrement since it wasn't actually deleted
					continue
				}

				processedTasks = append(processedTasks, TaskDefinitionPruneEntry{
					Arn:     def,
					DaysOld: diffInDays,
					Action:  "deleted",
					Reason:  "deregistered successfully",
					Family:  family,
				})
			} else {
				processedTasks = append(processedTasks, TaskDefinitionPruneEntry{
					Arn:     def,
					DaysOld: diffInDays,
					Action:  "deleted",
					Reason:  "dry run - would be deleted",
					Family:  family,
				})
			}
		}

		if resp.NextToken == nil {
			break
		}

		definitionInput.NextToken = resp.NextToken
	}

	skipped := totalCount - deleted - keep
	return totalCount, deleted, skipped, processedTasks, nil
}

func Prune(ctx context.Context, clients *AWSClients, cluster, service string, keepLast int, keepDays int, dryRun bool) (*PruneResult, error) {
	familyPrefix, err := getFamilyPrefix(ctx, cluster, service, clients.ECS)
	if err != nil {
		return nil, err
	}

	families, err := getFamilies(ctx, familyPrefix, clients.ECS)
	if err != nil {
		return nil, err
	}

	result := &PruneResult{
		Families:       families,
		TotalCount:     0,
		DeletedCount:   0,
		KeptCount:      0,
		SkippedCount:   0,
		DryRun:         dryRun,
		ProcessedTasks: []TaskDefinitionPruneEntry{},
	}

	for _, family := range families {
		totalCount, deletedCount, skippedCount, processedTasks, err := deregisterTaskFamily(ctx, family, keepLast, keepDays, dryRun, clients.ECS)
		if err != nil {
			return nil, err
		}

		result.TotalCount += totalCount
		result.DeletedCount += deletedCount
		result.SkippedCount += skippedCount
		result.ProcessedTasks = append(result.ProcessedTasks, processedTasks...)
	}

	// Calculate kept count
	result.KeptCount = result.TotalCount - result.DeletedCount - result.SkippedCount

	return result, nil
}

// =============================================================================
// Restart Methods
// =============================================================================

func stopAll(ctx context.Context, cluster, service string, client *ecs.Client) ([]StoppedTaskInfo, error) {
	tasks, err := client.ListTasks(ctx, &ecs.ListTasksInput{
		Cluster:     &cluster,
		ServiceName: &service,
	})
	if err != nil {
		return nil, err
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
		return "", "", err
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

// =============================================================================
// Revision Methods
// =============================================================================

func getFamilies(ctx context.Context, familyPrefix string, svc *ecs.Client) ([]string, error) {
	response, err := svc.ListTaskDefinitionFamilies(ctx, &ecs.ListTaskDefinitionFamiliesInput{
		FamilyPrefix: &familyPrefix,
	})

	if err != nil {
		return nil, err
	}

	return response.Families, nil
}

func getFamilyPrefix(ctx context.Context, cluster, service string, svc *ecs.Client) (string, error) {
	serviceResponse, err := svc.DescribeServices(ctx, &ecs.DescribeServicesInput{
		Cluster:  &cluster,
		Services: []string{service},
	})
	if err != nil {
		return "", err
	}

	serviceInfo, err := safeGetFirstPtr(serviceResponse.Services, "no services found in response")
	if err != nil {
		return "", fmt.Errorf("failed to get service information: %w", err)
	}

	response, err := svc.DescribeTaskDefinition(ctx, &ecs.DescribeTaskDefinitionInput{
		TaskDefinition: serviceInfo.TaskDefinition,
	})

	if err != nil {
		return "", err
	}

	if response.TaskDefinition.Family == nil {
		return "", errors.New("task definition has no family name")
	}

	return *response.TaskDefinition.Family, nil
}

func latestTaskDefinitionArn(ctx context.Context, cluster, service string, svc *ecs.Client) (string, error) {
	prefix, err := getFamilyPrefix(ctx, cluster, service, svc)
	if err != nil {
		return "", err
	}

	response, err := svc.ListTaskDefinitions(ctx, &ecs.ListTaskDefinitionsInput{
		FamilyPrefix: &prefix,
		Sort:         types.SortOrderDesc,
	})

	if err != nil {
		return "", err
	}

	arn, err := safeGetFirst(response.TaskDefinitionArns, "no task definition ARNs found in response")
	if err != nil {
		return "", fmt.Errorf("failed to get task definition ARN: %w", err)
	}

	return arn, nil
}

func getRevisions(ctx context.Context, familyPrefix string, lastRevisionsNr int, svc *ecs.Client) ([]RevisionEntry, error) {
	definitionInput := &ecs.ListTaskDefinitionsInput{
		FamilyPrefix: &familyPrefix,
		Sort:         types.SortOrderDesc,
	}

	var revisions []RevisionEntry
	total := 0

	for {
		response, err := svc.ListTaskDefinitions(ctx, definitionInput)

		if err != nil {
			return nil, err
		}

		for _, def := range response.TaskDefinitionArns {
			if lastRevisionsNr != 0 && lastRevisionsNr < (total+1) {
				break
			}

			resp, err := svc.DescribeTaskDefinition(ctx, &ecs.DescribeTaskDefinitionInput{
				TaskDefinition: &def,
			})

			if err != nil {
				// Log error but continue processing other revisions
				continue
			}

			if resp.TaskDefinition.RegisteredAt == nil {
				continue // Skip revisions without registration date
			}

			containerDef, err := safeGetFirstPtr(resp.TaskDefinition.ContainerDefinitions, "no container definitions found")
			if err != nil {
				continue // Skip revisions without container definitions
			}

			if containerDef.Image == nil {
				continue // Skip revisions without image
			}

			revisions = append(revisions, RevisionEntry{
				Revision:  resp.TaskDefinition.Revision,
				CreatedAt: *resp.TaskDefinition.RegisteredAt,
				DockerURI: *containerDef.Image,
				Family:    familyPrefix,
			})
			total++
		}

		if response.NextToken == nil {
			break
		}

		definitionInput.NextToken = response.NextToken
	}

	return revisions, nil
}

func Revisions(ctx context.Context, clients *AWSClients, cluster, service string, lastRevisionNr int) (*RevisionsResult, error) {
	familyPrefix, err := getFamilyPrefix(ctx, cluster, service, clients.ECS)
	if err != nil {
		return nil, err
	}

	response, err := getFamilies(ctx, familyPrefix, clients.ECS)
	if err != nil {
		return nil, err
	}

	var allRevisions []RevisionEntry
	for _, family := range response {
		revisions, err := getRevisions(ctx, family, lastRevisionNr, clients.ECS)
		if err != nil {
			return nil, err
		}
		allRevisions = append(allRevisions, revisions...)
	}

	return &RevisionsResult{
		Revisions: allRevisions,
	}, nil
}
