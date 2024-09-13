package ecs

import (
	"context"
	"log"

	"github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/aws/aws-sdk-go-v2/service/ecs/types"
	"github.com/fatih/color"
	"github.com/rodaine/table"
)

func (s *Service) getFamilies(ctx context.Context, familyPrefix string, svc *ecs.Client) ([]string, error) {
	response, err := svc.ListTaskDefinitionFamilies(ctx, &ecs.ListTaskDefinitionFamiliesInput{
		FamilyPrefix: &familyPrefix,
	})

	if err != nil {
		return nil, err
	}

	return response.Families, nil
}

func (s *Service) getFamilyPrefix(ctx context.Context, svc *ecs.Client) (string, error) {
	service, err := s.loadService(ctx, svc)
	if err != nil {
		return "", err
	}

	response, err := svc.DescribeTaskDefinition(ctx, &ecs.DescribeTaskDefinitionInput{
		TaskDefinition: service.TaskDefinition,
	})

	if err != nil {
		return "", err
	}

	return *response.TaskDefinition.Family, nil
}

func (s *Service) latestTaskDefinition(ctx context.Context, svc *ecs.Client) (string, error) {
	_, err := s.loadService(ctx, svc)
	if err != nil {
		return "", err
	}

	prefix, err := s.getFamilyPrefix(ctx, svc)
	if err != nil {
		return "", err
	}

	response, err := svc.ListTaskDefinitions(ctx, &ecs.ListTaskDefinitionsInput{
		FamilyPrefix: &prefix,
		Sort:         types.SortOrderDesc,
	})

	if err != nil {
		return "", err
	}

	return response.TaskDefinitionArns[0], nil
}

func (s *Service) printRevisions(ctx context.Context, familyPrefix string, lastRevisionsNr int, svc *ecs.Client) {
	definitionInput := &ecs.ListTaskDefinitionsInput{
		FamilyPrefix: &familyPrefix,
		Sort:         types.SortOrderDesc,
	}

	headerFmt := color.New(color.FgBlue, color.Underline).SprintfFunc()
	columnFmt := color.New(color.FgYellow).SprintfFunc()

	tbl := table.New("Revision", "Created At", "Docker URI")
	tbl.WithHeaderFormatter(headerFmt).WithFirstColumnFormatter(columnFmt)

	total := 0

	for {
		response, err := svc.ListTaskDefinitions(ctx, definitionInput)

		if err != nil {
			log.Fatalln(err)
		}

		for _, def := range response.TaskDefinitionArns {
			if lastRevisionsNr != 0 && lastRevisionsNr < (total+1) {
				break
			}

			resp, err := svc.DescribeTaskDefinition(ctx, &ecs.DescribeTaskDefinitionInput{
				TaskDefinition: &def,
			})

			if err != nil {
				log.Printf("Failed to describe task definition %s. (%v)\n", def, err)
				continue
			}

			tbl.AddRow(resp.TaskDefinition.Revision, resp.TaskDefinition.RegisteredAt, *resp.TaskDefinition.ContainerDefinitions[0].Image)
			total++
		}

		if response.NextToken == nil {
			break
		}

		definitionInput.NextToken = response.NextToken
	}

	tbl.Print()
}

func (s *Service) Revisions(lastRevisionNr int) {
	cfg, err := s.initCfg()
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
		s.printRevisions(ctx, family, lastRevisionNr, svc)
	}
}
