package ecs

import (
	"context"
	"fmt"
	"log"

	"github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/jinzhu/copier"
)

func (s *Service) cloneTaskDef(dockerImageUri string, svc *ecs.Client) (string, error) {
	service, err := s.loadService(svc)
	if err != nil {
		return "", err
	}

	// Get the last task definition ARN.
	// Load the latest task definition.
	response, err := svc.DescribeTaskDefinition(context.TODO(), &ecs.DescribeTaskDefinitionInput{
		TaskDefinition: service.TaskDefinition,
	})

	if err != nil {
		return "", err
	}

	if len(response.TaskDefinition.ContainerDefinitions) > 1 {
		return "", fmt.Errorf("multiple container definitions in a single task are not supported")
	}

	newDef := &ecs.RegisterTaskDefinitionInput{}
	copier.Copy(newDef, response.TaskDefinition)

	newDef.ContainerDefinitions[0].Image = &dockerImageUri

	output, err := svc.RegisterTaskDefinition(context.TODO(), newDef)
	if err != nil {
		return "", err
	}

	return *output.TaskDefinition.TaskDefinitionArn, nil
}

func (s *Service) Deploy(dockerImageUri string) {
	cfg, err := s.initCfg()
	if err != nil {
		log.Fatalln(err)
	}

	svc := ecs.NewFromConfig(cfg)

	// Clones the latest version of the task definition and inserts the new Docker URI.
	taskDefinitionArn, err := s.cloneTaskDef(dockerImageUri, svc)
	if err != nil {
		log.Fatalln(err)
	}

	fmt.Printf("New revision of the task %s has been created\n", taskDefinitionArn)

	updateOutput, err := svc.UpdateService(context.TODO(), &ecs.UpdateServiceInput{
		Cluster:        &s.Cluster,
		Service:        &s.Service,
		TaskDefinition: &taskDefinitionArn,
	})

	if err != nil {
		log.Fatalln(err)
	}

	fmt.Printf("Service %s has been updated.\n", *updateOutput.Service.ServiceArn)
}
