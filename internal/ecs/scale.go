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
)

const (
	minDesiredCount = 1
	maxDesiredCount = 1000
)

var ErrInvalidDesiredCount = errors.New("desired count must be greater than 0")

func Scale(ctx context.Context, clients *AWSClients, cluster, service string, desiredCount int32) (*ScaleResult, error) {
	if desiredCount < minDesiredCount {
		return nil, fmt.Errorf("%w: got %d, minimum is %d", ErrInvalidDesiredCount, desiredCount, minDesiredCount)
	}

	if desiredCount > maxDesiredCount {
		return nil, fmt.Errorf("desired count exceeds maximum: got %d, maximum is %d", desiredCount, maxDesiredCount)
	}

	describeResp, err := clients.ECS.DescribeServices(ctx, &ecs.DescribeServicesInput{
		Cluster:  &cluster,
		Services: []string{service},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to describe service: %w", err)
	}

	if len(describeResp.Services) == 0 {
		return nil, fmt.Errorf("service %s not found in cluster %s", service, cluster)
	}

	svc := describeResp.Services[0]
	if svc.Status == nil || *svc.Status != "ACTIVE" {
		return nil, fmt.Errorf("service %s is not in ACTIVE state", service)
	}

	previousDesiredCount := svc.DesiredCount

	updateResp, err := clients.ECS.UpdateService(ctx, &ecs.UpdateServiceInput{
		Cluster:      &cluster,
		Service:      &service,
		DesiredCount: &desiredCount,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to update service: %w", err)
	}

	result := &ScaleResult{
		PreviousDesiredCount: previousDesiredCount,
		NewDesiredCount:      desiredCount,
		ClusterName:          cluster,
		ServiceName:          service,
	}

	if updateResp.Service != nil && updateResp.Service.ServiceArn != nil {
		result.ServiceArn = *updateResp.Service.ServiceArn
	}

	return result, nil
}
