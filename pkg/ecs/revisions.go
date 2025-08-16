// Copyright (c) Petr Reichl and affiliates. All rights reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package ecs

import (
	"context"
	"errors"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/aws/aws-sdk-go-v2/service/ecs/types"
)

func getFamilies(ctx context.Context, familyPrefix string, svc *ecs.Client) ([]string, error) {
	response, err := svc.ListTaskDefinitionFamilies(ctx, &ecs.ListTaskDefinitionFamiliesInput{
		FamilyPrefix: &familyPrefix,
	})

	if err != nil {
		return nil, err
	}

	return response.Families, nil
}

func getFamilyPrefix(ctx context.Context, cluster, service string, svc *ecs.Client) (string, error) {
	serviceResponse, err := svc.DescribeServices(ctx, &ecs.DescribeServicesInput{
		Cluster:  &cluster,
		Services: []string{service},
	})
	if err != nil {
		return "", err
	}

	serviceInfo, err := safeGetFirstPtr(serviceResponse.Services, "no services found in response")
	if err != nil {
		return "", fmt.Errorf("failed to get service information: %w", err)
	}

	response, err := svc.DescribeTaskDefinition(ctx, &ecs.DescribeTaskDefinitionInput{
		TaskDefinition: serviceInfo.TaskDefinition,
	})

	if err != nil {
		return "", err
	}

	if response.TaskDefinition.Family == nil {
		return "", errors.New("task definition has no family name")
	}

	return *response.TaskDefinition.Family, nil
}

func latestTaskDefinitionArn(ctx context.Context, cluster, service string, svc *ecs.Client) (string, error) {
	prefix, err := getFamilyPrefix(ctx, cluster, service, svc)
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

	arn, err := safeGetFirst(response.TaskDefinitionArns, "no task definition ARNs found in response")
	if err != nil {
		return "", fmt.Errorf("failed to get task definition ARN: %w", err)
	}

	return arn, nil
}

func getRevisions(ctx context.Context, familyPrefix string, lastRevisionsNr int, svc *ecs.Client) ([]RevisionEntry, error) {
	definitionInput := &ecs.ListTaskDefinitionsInput{
		FamilyPrefix: &familyPrefix,
		Sort:         types.SortOrderDesc,
	}

	var revisions []RevisionEntry
	total := 0

	for {
		response, err := svc.ListTaskDefinitions(ctx, definitionInput)

		if err != nil {
			return nil, err
		}

		for _, def := range response.TaskDefinitionArns {
			if lastRevisionsNr != 0 && lastRevisionsNr < (total+1) {
				break
			}

			resp, err := svc.DescribeTaskDefinition(ctx, &ecs.DescribeTaskDefinitionInput{
				TaskDefinition: &def,
			})

			if err != nil {
				// Log error but continue processing other revisions
				continue
			}

			if resp.TaskDefinition.RegisteredAt == nil {
				continue // Skip revisions without registration date
			}

			containerDef, err := safeGetFirstPtr(resp.TaskDefinition.ContainerDefinitions, "no container definitions found")
			if err != nil {
				continue // Skip revisions without container definitions
			}

			if containerDef.Image == nil {
				continue // Skip revisions without image
			}

			revisions = append(revisions, RevisionEntry{
				Revision:  resp.TaskDefinition.Revision,
				CreatedAt: *resp.TaskDefinition.RegisteredAt,
				DockerURI: *containerDef.Image,
				Family:    familyPrefix,
			})
			total++
		}

		if response.NextToken == nil {
			break
		}

		definitionInput.NextToken = response.NextToken
	}

	return revisions, nil
}

func Revisions(ctx context.Context, clients *AWSClients, cluster, service string, lastRevisionNr int) (*RevisionsResult, error) {
	familyPrefix, err := getFamilyPrefix(ctx, cluster, service, clients.ECS)
	if err != nil {
		return nil, err
	}

	response, err := getFamilies(ctx, familyPrefix, clients.ECS)
	if err != nil {
		return nil, err
	}

	var allRevisions []RevisionEntry
	for _, family := range response {
		revisions, err := getRevisions(ctx, family, lastRevisionNr, clients.ECS)
		if err != nil {
			return nil, err
		}
		allRevisions = append(allRevisions, revisions...)
	}

	return &RevisionsResult{
		Revisions: allRevisions,
	}, nil
}
