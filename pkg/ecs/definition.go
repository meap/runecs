package ecs

import (
	"context"
	"fmt"
	"log"

	"github.com/aws/aws-sdk-go-v2/service/ecs"
)

func (s *Service) deregisterTaskFamily(family string, svc *ecs.Client) {
	definitionInput := &ecs.ListTaskDefinitionsInput{
		FamilyPrefix: &family,
	}

	for {
		resp, err := svc.ListTaskDefinitions(context.TODO(), definitionInput)
		if err != nil {
			log.Fatalln("Loading the list of definitions failed.")
		}

		count := len(resp.TaskDefinitionArns)

		for idx, def := range resp.TaskDefinitionArns {
			// The last in the family is skipped.
			if idx == count-1 && resp.NextToken == nil {
				fmt.Println("Task definition", def, "skipped.")
				continue
			}

			_, err := svc.DeregisterTaskDefinition(context.TODO(), &ecs.DeregisterTaskDefinitionInput{TaskDefinition: &def})
			if err != nil {
				fmt.Printf("Deregistering the task definition %s failed. (%v)\n", def, err)
				continue
			}

			fmt.Println("Task definition", def, "deregistered.")
		}

		if resp.NextToken == nil {
			break
		}

		definitionInput.NextToken = resp.NextToken
	}

	fmt.Println("All task definitions for family", family, "deregistered.")
}

func (s *Service) Prune() {
	cfg, err := s.initCfg()
	if err != nil {
		log.Fatalln(err)
	}

	svc := ecs.NewFromConfig(cfg)
	resp, err := svc.ListTaskDefinitionFamilies(context.TODO(), &ecs.ListTaskDefinitionFamiliesInput{})
	if err != nil {
		log.Fatalf("Listing task definition families failed. (%v)\n", err)
	}

	for _, family := range resp.Families {
		s.deregisterTaskFamily(family, svc)
	}
}
