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
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/aws/aws-sdk-go-v2/service/ecs/types"
)

func deregisterTaskFamily(ctx context.Context, family string, keepLast int, keepDays int, dryRun bool, svc *ecs.Client) (int, int, int, []TaskDefinitionPruneEntry, error) {
	definitionInput := &ecs.ListTaskDefinitionsInput{
		FamilyPrefix: &family,
		Sort:         types.SortOrderDesc,
	}

	today := time.Now().UTC()
	totalCount := 0
	deleted := 0
	keep := 0

	var processedTasks []TaskDefinitionPruneEntry

	for {
		resp, err := svc.ListTaskDefinitions(ctx, definitionInput)
		if err != nil {
			return 0, 0, 0, nil, fmt.Errorf("loading the list of definitions failed: %w", err)
		}

		count := len(resp.TaskDefinitionArns)
		totalCount += count

		for _, def := range resp.TaskDefinitionArns {
			response, err := svc.DescribeTaskDefinition(ctx, &ecs.DescribeTaskDefinitionInput{
				TaskDefinition: &def,
			})

			if err != nil {
				processedTasks = append(processedTasks, TaskDefinitionPruneEntry{
					Arn:     def,
					DaysOld: -1,
					Action:  "skipped",
					Reason:  fmt.Sprintf("Failed to describe: %v", err),
					Family:  family,
				})
				continue
			}

			if response.TaskDefinition.RegisteredAt == nil {
				processedTasks = append(processedTasks, TaskDefinitionPruneEntry{
					Arn:     def,
					DaysOld: -1,
					Action:  "skipped",
					Reason:  "Missing registration date",
					Family:  family,
				})
				continue
			}

			diffInDays := int(today.Sub(*response.TaskDefinition.RegisteredAt).Hours() / 24)

			// Last X
			if keep < keepLast {
				processedTasks = append(processedTasks, TaskDefinitionPruneEntry{
					Arn:     def,
					DaysOld: diffInDays,
					Action:  "kept",
					Reason:  fmt.Sprintf("keeping last %d definitions", keepLast),
					Family:  family,
				})
				keep++
				continue
			}

			if diffInDays < keepDays {
				processedTasks = append(processedTasks, TaskDefinitionPruneEntry{
					Arn:     def,
					DaysOld: diffInDays,
					Action:  "kept",
					Reason:  fmt.Sprintf("newer than %d days", keepDays),
					Family:  family,
				})
				continue
			}

			deleted++

			if !dryRun {
				_, err := svc.DeregisterTaskDefinition(ctx, &ecs.DeregisterTaskDefinitionInput{TaskDefinition: &def})
				if err != nil {
					processedTasks = append(processedTasks, TaskDefinitionPruneEntry{
						Arn:     def,
						DaysOld: diffInDays,
						Action:  "skipped",
						Reason:  fmt.Sprintf("deregistration failed: %v", err),
						Family:  family,
					})
					deleted-- // Decrement since it wasn't actually deleted
					continue
				}

				processedTasks = append(processedTasks, TaskDefinitionPruneEntry{
					Arn:     def,
					DaysOld: diffInDays,
					Action:  "deleted",
					Reason:  "deregistered successfully",
					Family:  family,
				})
			} else {
				processedTasks = append(processedTasks, TaskDefinitionPruneEntry{
					Arn:     def,
					DaysOld: diffInDays,
					Action:  "deleted",
					Reason:  "dry run - would be deleted",
					Family:  family,
				})
			}
		}

		if resp.NextToken == nil {
			break
		}

		definitionInput.NextToken = resp.NextToken
	}

	skipped := totalCount - deleted - keep
	return totalCount, deleted, skipped, processedTasks, nil
}

func Prune(ctx context.Context, clients *AWSClients, cluster, service string, keepLast int, keepDays int, dryRun bool) (*PruneResult, error) {
	familyPrefix, err := getFamilyPrefix(ctx, cluster, service, clients.ECS)
	if err != nil {
		return nil, err
	}

	families, err := getFamilies(ctx, familyPrefix, clients.ECS)
	if err != nil {
		return nil, err
	}

	result := &PruneResult{
		Families:       families,
		TotalCount:     0,
		DeletedCount:   0,
		KeptCount:      0,
		SkippedCount:   0,
		DryRun:         dryRun,
		ProcessedTasks: []TaskDefinitionPruneEntry{},
	}

	for _, family := range families {
		totalCount, deletedCount, skippedCount, processedTasks, err := deregisterTaskFamily(ctx, family, keepLast, keepDays, dryRun, clients.ECS)
		if err != nil {
			return nil, err
		}

		result.TotalCount += totalCount
		result.DeletedCount += deletedCount
		result.SkippedCount += skippedCount
		result.ProcessedTasks = append(result.ProcessedTasks, processedTasks...)
	}

	// Calculate kept count
	result.KeptCount = result.TotalCount - result.DeletedCount - result.SkippedCount

	return result, nil
}
