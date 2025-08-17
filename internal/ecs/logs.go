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
	"fmt"
	"log/slog"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs/types"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	ecsTypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"runecs.io/v1/internal/utils"
)

const (
	// ErrStreamUnexpectedEvent indicates an unexpected event type was received from CloudWatch Logs stream
	ErrStreamUnexpectedEvent = "unexpected event type received from log stream"
	// ErrStreamNilEvent indicates a nil event was received from CloudWatch Logs stream
	ErrStreamNilEvent = "nil event received from log stream"
	// ErrStreamError indicates an error occurred in the CloudWatch Logs stream
	ErrStreamError = "log stream error occurred"
)

func getLogStreamPrefix(ctx context.Context, client *ecs.Client, taskDefinitionArn string) (logGroup, logStreamPrefix, containerName string, err error) {
	resp, err := client.DescribeTaskDefinition(ctx, &ecs.DescribeTaskDefinitionInput{
		TaskDefinition: &taskDefinitionArn,
	})
	if err != nil {
		return "", "", "", fmt.Errorf("failed to describe task definition %s: %w", taskDefinitionArn, err)
	}

	containerDef, err := utils.SafeGetFirstPtr(resp.TaskDefinition.ContainerDefinitions, "no container definitions found")
	if err != nil {
		return "", "", "", fmt.Errorf("failed to get container definition from task %s: %w", taskDefinitionArn, err)
	}

	if containerDef.Name == nil {
		return "", "", "", fmt.Errorf("container definition has no name in task %s", taskDefinitionArn)
	}

	containerName = *containerDef.Name

	logConfig := containerDef.LogConfiguration
	if logConfig != nil && logConfig.LogDriver == ecsTypes.LogDriverAwslogs {
		logGroup = logConfig.Options["awslogs-group"]
		logStreamPrefix = logConfig.Options["awslogs-stream-prefix"]
	}

	return logGroup, logStreamPrefix, containerName, nil
}

func GetServiceLogs(ctx context.Context, clients *AWSClients, cluster, service string, startTime *int64) ([]LogEntry, error) {
	latestTaskDefArn, err := latestTaskDefinitionArn(ctx, cluster, service, clients.ECS)
	if err != nil {
		return nil, fmt.Errorf("failed to get latest task definition for service %s: %w", service, err)
	}

	logGroup, logStreamPrefix, containerName, err := getLogStreamPrefix(ctx, clients.ECS, latestTaskDefArn)
	if err != nil {
		return nil, fmt.Errorf("failed to get log configuration: %w", err)
	}

	if logGroup == "" || logStreamPrefix == "" {
		return nil, fmt.Errorf("service %s does not have CloudWatch logging configured", service)
	}

	input := &ecs.ListTasksInput{
		Cluster:     aws.String(cluster),
		ServiceName: aws.String(service),
	}

	listTasksOutput, err := clients.ECS.ListTasks(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to list tasks for service %s: %w", service, err)
	}

	if len(listTasksOutput.TaskArns) == 0 {
		return nil, fmt.Errorf("no running tasks found for service %s", service)
	}

	var allLogs []LogEntry
	for _, taskArn := range listTasksOutput.TaskArns {
		processID, err := extractARNResource(taskArn)
		if err != nil {
			continue // Skip tasks with invalid ARNs
		}

		input := &cloudwatchlogs.FilterLogEventsInput{
			LogGroupName:   aws.String(logGroup),
			LogStreamNames: []string{fmt.Sprintf("%s/%s/%s", logStreamPrefix, containerName, processID)},
		}

		if startTime != nil {
			input.StartTime = startTime
		}

		output, err := clients.CloudWatchLogs.FilterLogEvents(ctx, input)
		if err != nil {
			continue // Skip tasks with log filtering errors
		}

		for _, event := range output.Events {
			if event.LogStreamName == nil || event.Message == nil || event.Timestamp == nil {
				continue // Skip events with missing required fields
			}
			allLogs = append(allLogs, LogEntry{
				StreamName: *event.LogStreamName,
				Message:    *event.Message,
				Timestamp:  *event.Timestamp,
			})
		}
	}

	return allLogs, nil
}

func TailLogGroups(ctx context.Context, cwClient *cloudwatchlogs.Client, logGroupIdentifiers []string, logStreamPrefixes []string) (<-chan LogEntry, func(), error) {
	startLiveTailInput := &cloudwatchlogs.StartLiveTailInput{
		LogGroupIdentifiers:   logGroupIdentifiers,
		LogStreamNamePrefixes: logStreamPrefixes,
	}

	response, err := cwClient.StartLiveTail(ctx, startLiveTailInput)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to start live tail: %w", err)
	}

	stream := response.GetStream()
	logChan := make(chan LogEntry, 100)

	go func() {
		defer close(logChan)
		defer stream.Close()

		eventsChan := stream.Events()
		for {
			select {
			case <-ctx.Done():
				return
			case event, ok := <-eventsChan:
				if !ok {
					return
				}

				switch e := event.(type) {
				case *types.StartLiveTailResponseStreamMemberSessionStart:
					continue
				case *types.StartLiveTailResponseStreamMemberSessionUpdate:
					for _, logEvent := range e.Value.SessionResults {
						if logEvent.Message == nil || logEvent.Timestamp == nil || logEvent.LogStreamName == nil {
							continue
						}
						logEntry := LogEntry{
							StreamName: *logEvent.LogStreamName,
							Message:    *logEvent.Message,
							Timestamp:  *logEvent.Timestamp,
						}
						select {
						case logChan <- logEntry:
						case <-ctx.Done():
							return
						}
					}
				default:
					if err := stream.Err(); err != nil {
						slog.Error(ErrStreamError,
							"error", err,
							"context", "CloudWatch logs live tail stream")
						return
					}
					if event == nil {
						slog.Debug(ErrStreamNilEvent,
							"context", "CloudWatch logs live tail stream")
						return
					}
					slog.Warn(ErrStreamUnexpectedEvent,
						"event_type", fmt.Sprintf("%T", event),
						"context", "CloudWatch logs live tail stream")
				}
			}
		}
	}()

	closeFunc := func() {
		stream.Close()
	}

	return logChan, closeFunc, nil
}

func TailServiceLogs(ctx context.Context, clients *AWSClients, cluster, service string) (<-chan LogEntry, func(), error) {
	latestTaskDefArn, err := latestTaskDefinitionArn(ctx, cluster, service, clients.ECS)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get latest task definition for service %s: %w", service, err)
	}

	logGroup, logStreamPrefix, containerName, err := getLogStreamPrefix(ctx, clients.ECS, latestTaskDefArn)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get log configuration: %w", err)
	}

	if logGroup == "" || logStreamPrefix == "" {
		return nil, nil, fmt.Errorf("service %s does not have CloudWatch logging configured", service)
	}

	// Get the account ID using STS
	identity, err := clients.STS.GetCallerIdentity(ctx, &sts.GetCallerIdentityInput{})
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get caller identity: %w", err)
	}

	// Extract partition from caller's ARN to handle different AWS partitions
	partition, err := extractPartitionFromARN(*identity.Arn)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to extract partition from caller ARN: %w", err)
	}

	// Construct the LogGroup ARN with correct partition
	logGroupArn := buildARN(partition, "logs", clients.Region, *identity.Account, fmt.Sprintf("log-group:%s", logGroup))

	// Use LogStreamNamePrefixes to capture all streams for this service's containers
	logStreamPrefixPattern := fmt.Sprintf("%s/%s/", logStreamPrefix, containerName)

	return TailLogGroups(ctx, clients.CloudWatchLogs, []string{logGroupArn}, []string{logStreamPrefixPattern})
}
