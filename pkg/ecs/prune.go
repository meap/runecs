package ecs

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/aws/aws-sdk-go-v2/service/ecs/types"
)

func (s *Service) deregisterTaskFamily(ctx context.Context, family string, keepLast int, keepDays int, dryRun bool, svc *ecs.Client) {
	definitionInput := &ecs.ListTaskDefinitionsInput{
		FamilyPrefix: &family,
		Sort:         types.SortOrderDesc,
	}

	today := time.Now().UTC()
	totalCount := 0
	deleted := 0
	keep := 0

	for {
		resp, err := svc.ListTaskDefinitions(ctx, definitionInput)
		if err != nil {
			log.Fatalln("Loading the list of definitions failed.")
		}

		count := len(resp.TaskDefinitionArns)
		totalCount += count

		for _, def := range resp.TaskDefinitionArns {
			response, err := svc.DescribeTaskDefinition(ctx, &ecs.DescribeTaskDefinitionInput{
				TaskDefinition: &def,
			})

			if err != nil {
				log.Printf("Failed to describe task definition %s. (%v)\n", def, err)

				continue
			}

			diffInDays := int(today.Sub(*response.TaskDefinition.RegisteredAt).Hours() / 24)

			// Last X
			if keep < keepLast {
				fmt.Println("Task definition", def, "created", diffInDays, "days ago is skipped.")
				keep++

				continue
			}

			if diffInDays < keepDays {
				fmt.Println("Task definition", def, "created", diffInDays, "days ago is skipped.")

				continue
			}

			deleted++

			if !dryRun {
				_, err := svc.DeregisterTaskDefinition(ctx, &ecs.DeregisterTaskDefinitionInput{TaskDefinition: &def})
				if err != nil {
					fmt.Printf("Deregistering the task definition %s failed. (%v)\n", def, err)

					continue
				}

				fmt.Println("Task definition", def, "created", diffInDays, "days ago is deregistered.")
			}
		}

		if resp.NextToken == nil {
			break
		}

		definitionInput.NextToken = resp.NextToken
	}

	if dryRun {
		fmt.Printf("Total of %d task definitions. Will delete %d definitions.", totalCount, deleted)

		return
	}

	fmt.Printf("Total of %d task definitions. Deleted %d definitions.", totalCount, deleted)
}

func (s *Service) Prune(keepLast int, keepDays int, dryRun bool) {
	cfg, err := initCfg()
	if err != nil {
		log.Fatalln(err)
	}

	ctx := context.Background()

	svc := ecs.NewFromConfig(cfg)

	familyPrefix, err := s.getFamilyPrefix(ctx, svc)
	if err != nil {
		log.Fatalln(err)
	}

	response, err := s.getFamilies(ctx, familyPrefix, svc)
	if err != nil {
		log.Fatalln(err)
	}

	for _, family := range response {
		s.deregisterTaskFamily(ctx, family, keepLast, keepDays, dryRun, svc)
	}
}
