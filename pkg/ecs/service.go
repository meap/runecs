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

// =============================================================================
// Core Service Methods
// =============================================================================

func initCfg() (aws.Config, error) {
	configFunctions := []func(*config.LoadOptions) error{
		config.WithRetryer(func() aws.Retryer {
			return retry.AddWithMaxAttempts(retry.NewStandard(), defaultNumberOfRetries)
		}),
	}

	cfg, err := config.LoadDefaultConfig(context.Background(), configFunctions...)

	if err != nil {
		return aws.Config{}, err
	}

	return cfg, nil
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

	newDef := &ecs.RegisterTaskDefinitionInput{}
	if err := copier.Copy(newDef, response.TaskDefinition); err != nil {
		return "", err
	}

	split := strings.Split(*newDef.ContainerDefinitions[0].Image, ":")
	newDockerURI := fmt.Sprintf("%s:%s", split[0], dockerImageTag)

	newDef.ContainerDefinitions[0].Image = &newDockerURI

	output, err := svc.RegisterTaskDefinition(ctx, newDef)
	if err != nil {
		return "", err
	}

	return *output.TaskDefinition.TaskDefinitionArn, nil
}

func Deploy(cluster, service, dockerImageTag string) (*DeployResult, error) {
	cfg, err := initCfg()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize AWS configuration: %w", err)
	}

	ctx := context.Background()

	svc := ecs.NewFromConfig(cfg)

	// Clones the latest version of the task definition and inserts the new Docker URI.
	TaskDefinitionArn, err := cloneTaskDef(ctx, cluster, service, dockerImageTag, svc)
	if err != nil {
		return nil, fmt.Errorf("failed to clone task definition: %w", err)
	}

	updateOutput, err := svc.UpdateService(ctx, &ecs.UpdateServiceInput{
		Cluster:        &cluster,
		Service:        &service,
		TaskDefinition: &TaskDefinitionArn,
	})

	if err != nil {
		return nil, fmt.Errorf("failed to update service: %w", err)
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

	def := resp.Services[0]

	// Fetch the latest task definition. Keep in mind that the active service may
	// have a different task definition that is available, see. *def.TaskDefinition
	taskDef, err := latestTaskDefinitionArn(ctx, cluster, service, client)
	if err != nil {
		return ServiceDefinition{}, err
	}

	return ServiceDefinition{
		Subnets:        def.NetworkConfiguration.AwsvpcConfiguration.Subnets,
		SecurityGroups: def.NetworkConfiguration.AwsvpcConfiguration.SecurityGroups,
		TaskDef:        taskDef,
	}, nil
}

func describeTask(ctx context.Context, client *ecs.Client, taskArn *string) (TaskDefinition, error) {
	resp, _ := client.DescribeTaskDefinition(ctx, &ecs.DescribeTaskDefinitionInput{TaskDefinition: taskArn})

	if len(resp.TaskDefinition.ContainerDefinitions) == 0 {
		return TaskDefinition{}, fmt.Errorf("no container definitions found in task definition %s", *taskArn)
	}

	output := TaskDefinition{}
	output.Name = *resp.TaskDefinition.ContainerDefinitions[0].Name

	logConfig := resp.TaskDefinition.ContainerDefinitions[0].LogConfiguration
	if logConfig != nil && logConfig.LogDriver == types.LogDriverAwslogs {
		output.LogGroup = logConfig.Options["awslogs-group"]
		output.LogStreamPrefix = logConfig.Options["awslogs-stream-prefix"]
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

	if *output.Tasks[0].LastStatus == "STOPPED" {
		if exitCode := *output.Tasks[0].Containers[0].ExitCode; exitCode != 0 {
			return true, fmt.Errorf("task %s failed with exit code %d", *output.Tasks[0].TaskArn, exitCode)
		}
	}

	return *output.Tasks[0].LastStatus == "STOPPED", nil
}

func Execute(cluster, service string, cmd []string, waitForCompletion bool, dockerImageTag string) (*ExecuteResult, error) {
	cfg, err := initCfg()
	if err != nil {
		return nil, err
	}

	ctx := context.Background()

	svc := ecs.NewFromConfig(cfg)

	sdef, err := describeService(ctx, cluster, service, svc)
	if err != nil {
		return nil, fmt.Errorf("error loading service %s in cluster %s: %w", service, cluster, err)
	}

	tdef, err := describeTask(ctx, svc, &sdef.TaskDef)
	if err != nil {
		return nil, fmt.Errorf("error loading task definition %s: %w", sdef.TaskDef, err)
	}

	var taskDef string
	newTaskDefCreated := false

	if dockerImageTag != "" {
		taskDef, err = cloneTaskDef(ctx, cluster, service, dockerImageTag, svc)
		if err != nil {
			return nil, err
		}
		newTaskDefCreated = true
	} else {
		taskDef = sdef.TaskDef
	}

	output, err := svc.RunTask(ctx, &ecs.RunTaskInput{
		Cluster:        &cluster,
		TaskDefinition: &taskDef,
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
		return nil, err
	}

	executedTask := output.Tasks[0]
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
			logs, newTimestamp, err := getProcessLogs(ctx, tdef.LogGroup, tdef.LogStreamPrefix, *executedTask.TaskArn, tdef.Name, lastTimestamp)
			if err == nil {
				result.Logs = append(result.Logs, logs...)
				lastTimestamp = &newTimestamp
			}

			success, err := wait(ctx, cluster, svc, *executedTask.TaskArn)
			if err != nil {
				return result, err
			}

			if success {
				result.Finished = true
				break
			}

			time.Sleep(5 * time.Second)
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
	startTime *int64) ([]LogEntry, int64, error) {

	cfg, err := initCfg()
	if err != nil {
		return nil, 0, fmt.Errorf("failed to initialize AWS configuration. (%w)", err)
	}

	processID := extractProcessID(taskArn)
	client := cloudwatchlogs.NewFromConfig(cfg)

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
		logs = append(logs, LogEntry{
			StreamName: *event.LogStreamName,
			Message:    *event.Message,
			Timestamp:  *event.Timestamp,
		})
	}

	var lastEventTimestamp int64
	if len(output.Events) > 0 {
		lastEventTimestamp = *output.Events[len(output.Events)-1].Timestamp + 1
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

func Prune(cluster, service string, keepLast int, keepDays int, dryRun bool) (*PruneResult, error) {
	cfg, err := initCfg()
	if err != nil {
		return nil, err
	}

	ctx := context.Background()

	svc := ecs.NewFromConfig(cfg)

	familyPrefix, err := getFamilyPrefix(ctx, cluster, service, svc)
	if err != nil {
		return nil, err
	}

	families, err := getFamilies(ctx, familyPrefix, svc)
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
		totalCount, deletedCount, skippedCount, processedTasks, err := deregisterTaskFamily(ctx, family, keepLast, keepDays, dryRun, svc)
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

	return *output.Service.ServiceArn, *output.Service.TaskDefinition, nil
}

func Restart(cluster, service string, kill bool) (*RestartResult, error) {
	cfg, err := initCfg()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize AWS configuration: %w", err)
	}

	ctx := context.Background()
	svc := ecs.NewFromConfig(cfg)

	result := &RestartResult{}

	if kill {
		result.Method = "kill"
		stoppedTasks, err := stopAll(ctx, cluster, service, svc)
		if err != nil {
			return nil, fmt.Errorf("failed to stop tasks: %w", err)
		}
		result.StoppedTasks = stoppedTasks
	} else {
		result.Method = "force_deploy"
		serviceArn, taskDefinition, err := forceNewDeploy(ctx, cluster, service, svc)
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

	serviceInfo := serviceResponse.Services[0]
	response, err := svc.DescribeTaskDefinition(ctx, &ecs.DescribeTaskDefinitionInput{
		TaskDefinition: serviceInfo.TaskDefinition,
	})

	if err != nil {
		return "", err
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

	return response.TaskDefinitionArns[0], nil
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

			revisions = append(revisions, RevisionEntry{
				Revision:  resp.TaskDefinition.Revision,
				CreatedAt: *resp.TaskDefinition.RegisteredAt,
				DockerURI: *resp.TaskDefinition.ContainerDefinitions[0].Image,
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

func Revisions(cluster, service string, lastRevisionNr int) (*RevisionsResult, error) {
	cfg, err := initCfg()
	if err != nil {
		return nil, err
	}

	ctx := context.Background()

	svc := ecs.NewFromConfig(cfg)

	familyPrefix, err := getFamilyPrefix(ctx, cluster, service, svc)
	if err != nil {
		return nil, err
	}

	response, err := getFamilies(ctx, familyPrefix, svc)
	if err != nil {
		return nil, err
	}

	var allRevisions []RevisionEntry
	for _, family := range response {
		revisions, err := getRevisions(ctx, family, lastRevisionNr, svc)
		if err != nil {
			return nil, err
		}
		allRevisions = append(allRevisions, revisions...)
	}

	return &RevisionsResult{
		Revisions: allRevisions,
	}, nil
}
