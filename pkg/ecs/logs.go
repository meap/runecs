package ecs

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
)

type printProcessLogsOutput struct {
	lastEventTimestamp int64
}

func (s *Service) printProcessLogs(
	ctx context.Context, logGroupname string,
	logStreamPrefix string,
	taskArn string,
	name string,
	startTime *int64) (printProcessLogsOutput, error) {

	cfg, err := initCfg()
	if err != nil {
		return printProcessLogsOutput{}, fmt.Errorf("failed to initialize AWS configuration. (%w)", err)
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
		return printProcessLogsOutput{}, fmt.Errorf("failed to filter log events (%w)", err)
	}

	for _, event := range output.Events {
		fmt.Println(*event.LogStreamName, *event.Message)
	}

	var lastEventTimestamp int64
	if len(output.Events) > 0 {
		lastEventTimestamp = *output.Events[len(output.Events)-1].Timestamp + 1
	} else if startTime != nil {
		lastEventTimestamp = *startTime
	} else {
		lastEventTimestamp = 0
	}

	return printProcessLogsOutput{
		lastEventTimestamp: lastEventTimestamp,
	}, nil
}
