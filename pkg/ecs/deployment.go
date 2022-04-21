package ecs

import (
	"context"
	"fmt"
	"log"

	"github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/aws/aws-sdk-go-v2/service/ecs/types"
	"github.com/jinzhu/copier"
)

func (s *Service) cloneTaskDef(taskDefinitionArn string, dockerImageUri string, svc *ecs.Client) (string, error) {
	// Get the last task definition ARN.
	// Load the latest task definition.
	response, err := svc.DescribeTaskDefinition(context.TODO(), &ecs.DescribeTaskDefinitionInput{
		TaskDefinition: &taskDefinitionArn,
	})

	if err != nil {
		return "-1", err
	}

	if len(response.TaskDefinition.ContainerDefinitions) > 1 {
		log.Fatalln("Multiple container definitions in a single task are not supported.")
	}

	newDef := &ecs.RegisterTaskDefinitionInput{}
	copier.Copy(newDef, response.TaskDefinition)

	newDef.ContainerDefinitions[0].Image = &dockerImageUri

	output, err := svc.RegisterTaskDefinition(context.TODO(), newDef)
	if err != nil {
		log.Fatalln(err)
	}

	return *output.TaskDefinition.TaskDefinitionArn, nil
}

func (s *Service) loadService(svc *ecs.Client) (types.Service, error) {
	response, err := svc.DescribeServices(context.TODO(), &ecs.DescribeServicesInput{
		Cluster:  &s.Cluster,
		Services: []string{s.Service},
	})

	if err != nil {
		return types.Service{}, err
	}

	return response.Services[0], nil
}

func (s *Service) Deploy(dockerImageUri string, run bool) {
	cfg, err := s.initCfg()
	if err != nil {
		log.Fatalln(err)
	}

	svc := ecs.NewFromConfig(cfg)

	service, err := s.loadService(svc)
	if err != nil {
		log.Fatalln(err)
	}

	// Clones the latest version of the task definition and inserts the new Docker URI.
	taskDefinitionArn, err := s.cloneTaskDef(*service.TaskDefinition, dockerImageUri, svc)
	if err != nil {
		log.Fatalln(err)
	}

	fmt.Printf("New revision of the task %s has been created\n", taskDefinitionArn)

	if run {
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
}
