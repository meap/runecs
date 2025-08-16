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

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
)

func getTaskLogs(
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
