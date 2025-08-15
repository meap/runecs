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
	"log"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/retry"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/aws/aws-sdk-go-v2/service/ecs/types"
	"github.com/dustin/go-humanize"
	"github.com/jinzhu/copier"
)

const (
	defaultNumberOfRetries = 10
)

// ECS parameters that are used to run jobs.
type Service struct {
	Cluster string `mapstructure:"CLUSTER"`
	Service string `mapstructure:"SERVICE"`
}

// Local types used by Service methods
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

// =============================================================================
// Core Service Methods
// =============================================================================

func (s *Service) loadService(ctx context.Context, svc *ecs.Client) (types.Service, error) {
	response, err := svc.DescribeServices(ctx, &ecs.DescribeServicesInput{
		Cluster:  &s.Cluster,
		Services: []string{s.Service},
	})

	if err != nil {
		return types.Service{}, err
	}

	return response.Services[0], nil
}

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

func (s *Service) cloneTaskDef(ctx context.Context, dockerImageTag string, svc *ecs.Client) (string, error) {
	_, err := s.loadService(ctx, svc)
	if err != nil {
		return "", err
	}

	// Get the last task definition ARN.
	// Load the latest task definition.
	latestDef, err := s.latestTaskDefinitionArn(ctx, svc)
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

func (s *Service) Deploy(dockerImageTag string) {
	cfg, err := initCfg()
	if err != nil {
		log.Fatalln(err)
	}

	ctx := context.Background()

	svc := ecs.NewFromConfig(cfg)

	// Clones the latest version of the task definition and inserts the new Docker URI.
	TaskDefinitionArn, err := s.cloneTaskDef(ctx, dockerImageTag, svc)
	if err != nil {
		log.Fatalln(err)
	}

	fmt.Printf("New revision of the task %s has been created\n", TaskDefinitionArn)

	updateOutput, err := svc.UpdateService(ctx, &ecs.UpdateServiceInput{
		Cluster:        &s.Cluster,
		Service:        &s.Service,
		TaskDefinition: &TaskDefinitionArn,
	})

	if err != nil {
		log.Fatalln(err)
	}

	fmt.Printf("Service %s has been updated.\n", *updateOutput.Service.ServiceArn)
}

// =============================================================================
// Execute Methods
// =============================================================================

func (s *Service) describeService(ctx context.Context, client *ecs.Client) (ServiceDefinition, error) {
	resp, err := client.DescribeServices(ctx, &ecs.DescribeServicesInput{
		Cluster:  &s.Cluster,
		Services: []string{s.Service},
	})

	if err != nil {
		return ServiceDefinition{}, err
	}

	def := resp.Services[0]

	// Fetch the latest task definition. Keep in mind that the active service may
	// have a different task definition that is available, see. *def.TaskDefinition
	taskDef, err := s.latestTaskDefinitionArn(ctx, client)
	if err != nil {
		return ServiceDefinition{}, err
	}

	return ServiceDefinition{
		Subnets:        def.NetworkConfiguration.AwsvpcConfiguration.Subnets,
		SecurityGroups: def.NetworkConfiguration.AwsvpcConfiguration.SecurityGroups,
		TaskDef:        taskDef,
	}, nil
}

func (s *Service) describeTask(ctx context.Context, client *ecs.Client, taskArn *string) (TaskDefinition, error) {
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

func (s *Service) wait(ctx context.Context, client *ecs.Client, task string) (bool, error) {
	output, err := client.DescribeTasks(ctx, &ecs.DescribeTasksInput{
		Cluster: &s.Cluster,
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

func (s *Service) Execute(cmd []string, wait bool, dockerImageTag string) (*ExecuteResult, error) {
	cfg, err := initCfg()
	if err != nil {
		return nil, err
	}

	ctx := context.Background()

	svc := ecs.NewFromConfig(cfg)

	sdef, err := s.describeService(ctx, svc)
	if err != nil {
		return nil, fmt.Errorf("error loading service %s in cluster %s: %w", s.Service, s.Cluster, err)
	}

	tdef, err := s.describeTask(ctx, svc, &sdef.TaskDef)
	if err != nil {
		return nil, fmt.Errorf("error loading task definition %s: %w", sdef.TaskDef, err)
	}

	var taskDef string
	newTaskDefCreated := false

	if dockerImageTag != "" {
		taskDef, err = s.cloneTaskDef(ctx, dockerImageTag, svc)
		if err != nil {
			return nil, err
		}
		newTaskDefCreated = true
	} else {
		taskDef = sdef.TaskDef
	}

	output, err := svc.RunTask(ctx, &ecs.RunTaskInput{
		Cluster:        &s.Cluster,
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
		ServiceName:       s.Service,
		TaskDefinition:    taskDef,
		TaskArn:           *executedTask.TaskArn,
		NewTaskDefCreated: newTaskDefCreated,
		Finished:          false,
		Logs:              []LogEntry{},
	}

	if wait {
		var lastTimestamp *int64 = nil

		for {
			logs, newTimestamp, err := s.getProcessLogs(ctx, tdef.LogGroup, tdef.LogStreamPrefix, *executedTask.TaskArn, tdef.Name, lastTimestamp)
			if err == nil {
				result.Logs = append(result.Logs, logs...)
				lastTimestamp = &newTimestamp
			}

			success, err := s.wait(ctx, svc, *executedTask.TaskArn)
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

func (s *Service) getProcessLogs(
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

func (s *Service) deregisterTaskFamily(ctx context.Context, family string, keepLast int, keepDays int, dryRun bool, svc *ecs.Client) {
	definitionInput := &ecs.ListTaskDefinitionsInput{
		FamilyPrefix: &family,
		Sort:         types.SortOrderDesc,
	}

	today := time.Now().UTC()
	totalCount := 0
	deleted := 0
	keep := 0

	for {
		resp, err := svc.ListTaskDefinitions(ctx, definitionInput)
		if err != nil {
			log.Fatalln("Loading the list of definitions failed.")
		}

		count := len(resp.TaskDefinitionArns)
		totalCount += count

		for _, def := range resp.TaskDefinitionArns {
			response, err := svc.DescribeTaskDefinition(ctx, &ecs.DescribeTaskDefinitionInput{
				TaskDefinition: &def,
			})

			if err != nil {
				log.Printf("Failed to describe task definition %s. (%v)\n", def, err)

				continue
			}

			diffInDays := int(today.Sub(*response.TaskDefinition.RegisteredAt).Hours() / 24)

			// Last X
			if keep < keepLast {
				fmt.Println("Task definition", def, "created", diffInDays, "days ago is skipped.")
				keep++

				continue
			}

			if diffInDays < keepDays {
				fmt.Println("Task definition", def, "created", diffInDays, "days ago is skipped.")

				continue
			}

			deleted++

			if !dryRun {
				_, err := svc.DeregisterTaskDefinition(ctx, &ecs.DeregisterTaskDefinitionInput{TaskDefinition: &def})
				if err != nil {
					fmt.Printf("Deregistering the task definition %s failed. (%v)\n", def, err)

					continue
				}

				fmt.Println("Task definition", def, "created", diffInDays, "days ago is deregistered.")
			}
		}

		if resp.NextToken == nil {
			break
		}

		definitionInput.NextToken = resp.NextToken
	}

	if dryRun {
		fmt.Printf("Total of %d task definitions. Will delete %d definitions.", totalCount, deleted)

		return
	}

	fmt.Printf("Total of %d task definitions. Deleted %d definitions.", totalCount, deleted)
}

func (s *Service) Prune(keepLast int, keepDays int, dryRun bool) {
	cfg, err := initCfg()
	if err != nil {
		log.Fatalln(err)
	}

	ctx := context.Background()

	svc := ecs.NewFromConfig(cfg)

	familyPrefix, err := s.getFamilyPrefix(ctx, svc)
	if err != nil {
		log.Fatalln(err)
	}

	response, err := s.getFamilies(ctx, familyPrefix, svc)
	if err != nil {
		log.Fatalln(err)
	}

	for _, family := range response {
		s.deregisterTaskFamily(ctx, family, keepLast, keepDays, dryRun, svc)
	}
}

// =============================================================================
// Restart Methods
// =============================================================================

func (s *Service) stopAll(ctx context.Context, client *ecs.Client) error {
	tasks, err := client.ListTasks(ctx, &ecs.ListTasksInput{
		Cluster:     &s.Cluster,
		ServiceName: &s.Service,
	})
	if err != nil {
		return err
	}

	for _, taskArn := range tasks.TaskArns {
		output, err := client.StopTask(ctx, &ecs.StopTaskInput{
			Cluster: &s.Cluster,
			Task:    &taskArn,
		})
		if err != nil {
			log.Println(fmt.Errorf("failed to stop task %s. (%w)", taskArn, err))

			continue
		}

		fmt.Printf("Stopped task %s started %s\n", *output.Task.TaskArn, humanize.Time(*output.Task.StartedAt))
	}

	return nil
}

func (s *Service) forceNewDeploy(ctx context.Context, client *ecs.Client) error {
	taskDef, err := s.latestTaskDefinitionArn(ctx, client)
	if err != nil {
		return err
	}

	output, err := client.UpdateService(ctx, &ecs.UpdateServiceInput{
		Cluster:            &s.Cluster,
		Service:            &s.Service,
		TaskDefinition:     &taskDef,
		ForceNewDeployment: true,
	})

	if err != nil {
		return err
	}

	fmt.Printf("Service %s restarted by starting new tasks using the task definition %s.\n", s.Service, *output.Service.TaskDefinition)

	return nil
}

func (s *Service) Restart(kill bool) {
	cfg, err := initCfg()
	if err != nil {
		log.Fatalln(err)
	}

	svc := ecs.NewFromConfig(cfg)
	if kill {
		err := s.stopAll(context.Background(), svc)
		if err != nil {
			log.Fatalln(err)
		}
	} else {
		err := s.forceNewDeploy(context.Background(), svc)
		if err != nil {
			log.Fatalln(err)
		}
	}

	fmt.Println("Done.")
}

// =============================================================================
// Revision Methods
// =============================================================================

func (s *Service) getFamilies(ctx context.Context, familyPrefix string, svc *ecs.Client) ([]string, error) {
	response, err := svc.ListTaskDefinitionFamilies(ctx, &ecs.ListTaskDefinitionFamiliesInput{
		FamilyPrefix: &familyPrefix,
	})

	if err != nil {
		return nil, err
	}

	return response.Families, nil
}

func (s *Service) getFamilyPrefix(ctx context.Context, svc *ecs.Client) (string, error) {
	service, err := s.loadService(ctx, svc)
	if err != nil {
		return "", err
	}

	response, err := svc.DescribeTaskDefinition(ctx, &ecs.DescribeTaskDefinitionInput{
		TaskDefinition: service.TaskDefinition,
	})

	if err != nil {
		return "", err
	}

	return *response.TaskDefinition.Family, nil
}

func (s *Service) latestTaskDefinitionArn(ctx context.Context, svc *ecs.Client) (string, error) {
	_, err := s.loadService(ctx, svc)
	if err != nil {
		return "", err
	}

	prefix, err := s.getFamilyPrefix(ctx, svc)
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

func (s *Service) getRevisions(ctx context.Context, familyPrefix string, lastRevisionsNr int, svc *ecs.Client) ([]RevisionEntry, error) {
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

func (s *Service) Revisions(lastRevisionNr int) (*RevisionsResult, error) {
	cfg, err := initCfg()
	if err != nil {
		return nil, err
	}

	ctx := context.Background()

	svc := ecs.NewFromConfig(cfg)

	familyPrefix, err := s.getFamilyPrefix(ctx, svc)
	if err != nil {
		return nil, err
	}

	response, err := s.getFamilies(ctx, familyPrefix, svc)
	if err != nil {
		return nil, err
	}

	var allRevisions []RevisionEntry
	for _, family := range response {
		revisions, err := s.getRevisions(ctx, family, lastRevisionNr, svc)
		if err != nil {
			return nil, err
		}
		allRevisions = append(allRevisions, revisions...)
	}

	return &RevisionsResult{
		Revisions: allRevisions,
	}, nil
}
