package ecs

import (
	"context"
	"fmt"
	"log"

	"github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/aws/aws-sdk-go-v2/service/ecs/types"
)

func (s *Service) getFamilies(familyPrefix string, svc *ecs.Client) ([]string, error) {
	response, err := svc.ListTaskDefinitionFamilies(context.TODO(), &ecs.ListTaskDefinitionFamiliesInput{
		FamilyPrefix: &familyPrefix,
	})

	if err != nil {
		return nil, err
	}

	return response.Families, nil
}

func (s *Service) getFamilyPrefix(svc *ecs.Client) (string, error) {
	service, err := s.loadService(svc)
	if err != nil {
		return "", err
	}

	response, err := svc.DescribeTaskDefinition(context.TODO(), &ecs.DescribeTaskDefinitionInput{
		TaskDefinition: service.TaskDefinition,
	})

	if err != nil {
		return "", err
	}

	return *response.TaskDefinition.Family, nil
}

func (s *Service) printRevisions(familyPrefix string, svc *ecs.Client) {
	definitionInput := &ecs.ListTaskDefinitionsInput{
		FamilyPrefix: &familyPrefix,
		Sort:         types.SortOrderDesc,
	}

	for {
		response, err := svc.ListTaskDefinitions(context.TODO(), definitionInput)

		if err != nil {
			log.Fatalln(err)
		}

		for _, def := range response.TaskDefinitionArns {
			resp, err := svc.DescribeTaskDefinition(context.TODO(), &ecs.DescribeTaskDefinitionInput{
				TaskDefinition: &def,
			})

			if err != nil {
				log.Printf("Failed to describe task definition %s. (%v)\n", def, err)
				continue
			}

			fmt.Println(resp.TaskDefinition.RegisteredAt, *resp.TaskDefinition.ContainerDefinitions[0].Image)
		}

		if response.NextToken == nil {
			break
		}

		definitionInput.NextToken = response.NextToken
	}
}

func (s *Service) Revisions() {
	cfg, err := s.initCfg()
	if err != nil {
		log.Fatalln(err)
	}

	svc := ecs.NewFromConfig(cfg)

	familyPrefix, err := s.getFamilyPrefix(svc)
	if err != nil {
		log.Fatalln(err)
	}

	response, err := s.getFamilies(familyPrefix, svc)
	if err != nil {
		log.Fatalln(err)
	}

	for _, family := range response {
		s.printRevisions(family, svc)
	}
}
