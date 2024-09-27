package ecs

import (
	"context"
	"fmt"
	"log"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
)

func (s *Service) printProcessLogs(
	ctx context.Context, logGroupname string,
	logStreamPrefix string,
	taskArn string,
	name string) error {
	log.Printf("Loading logs for %s: %s", logGroupname, taskArn)

	cfg, err := s.initCfg()
	if err != nil {
		return fmt.Errorf("failed to initialize AWS configuration. (%w)", err)
	}

	processID := extractProcessID(taskArn)
	client := cloudwatchlogs.NewFromConfig(cfg)

	output, err := client.FilterLogEvents(ctx, &cloudwatchlogs.FilterLogEventsInput{
		LogGroupName:   aws.String(logGroupname),
		LogStreamNames: []string{fmt.Sprintf("%s/%s/%s", logStreamPrefix, name, processID)},
	})

	if err != nil {
		return fmt.Errorf("failed to filter log events (%w)", err)
	}

	for _, event := range output.Events {
		fmt.Println(*event.LogStreamName, *event.Message)
	}

	return nil
}
