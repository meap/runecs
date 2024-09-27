package ecs

import (
	"context"
	"fmt"
	"log"

	"github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/dustin/go-humanize"
)

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
	taskDef, err := s.latestTaskDefinition(ctx, client)
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
	cfg, err := s.initCfg()
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
