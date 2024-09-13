package ecs

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/jinzhu/copier"
)

func (s *Service) cloneTaskDef(ctx context.Context, dockerImageTag string, svc *ecs.Client) (string, error) {
	_, err := s.loadService(ctx, svc)
	if err != nil {
		return "", err
	}

	// Get the last task definition ARN.
	// Load the latest task definition.
	latestDef, err := s.latestTaskDefinition(ctx, svc)
	if err != nil {
		return "", nil
	}

	response, err := svc.DescribeTaskDefinition(ctx, &ecs.DescribeTaskDefinitionInput{
		TaskDefinition: &latestDef,
	})

	if err != nil {
		return "", err
	}

	if len(response.TaskDefinition.ContainerDefinitions) > 1 {
		return "", fmt.Errorf("multiple container definitions in a single task are not supported")
	}

	newDef := &ecs.RegisterTaskDefinitionInput{}
	copier.Copy(newDef, response.TaskDefinition)

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
	cfg, err := s.initCfg()
	if err != nil {
		log.Fatalln(err)
	}

	ctx := context.Background()

	svc := ecs.NewFromConfig(cfg)

	// Clones the latest version of the task definition and inserts the new Docker URI.
	taskDefinitionArn, err := s.cloneTaskDef(ctx, dockerImageTag, svc)
	if err != nil {
		log.Fatalln(err)
	}

	fmt.Printf("New revision of the task %s has been created\n", taskDefinitionArn)

	updateOutput, err := svc.UpdateService(ctx, &ecs.UpdateServiceInput{
		Cluster:        &s.Cluster,
		Service:        &s.Service,
		TaskDefinition: &taskDefinitionArn,
	})

	if err != nil {
		log.Fatalln(err)
	}

	fmt.Printf("Service %s has been updated.\n", *updateOutput.Service.ServiceArn)
}
