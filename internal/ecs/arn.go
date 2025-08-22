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
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws/arn"
)

// extractARNResource extracts the resource identifier from an AWS ARN
// using the official AWS SDK ARN parser
func extractARNResource(arnString string) (string, error) {
	parsedARN, err := arn.Parse(arnString)
	if err != nil {
		return "", fmt.Errorf("failed to parse ARN %q: %w", arnString, err)
	}

	// Extract the resource ID from the Resource field
	// For resources like "task/abc123" or "user/David", get the part after "/"
	if idx := strings.LastIndex(parsedARN.Resource, "/"); idx != -1 {
		return parsedARN.Resource[idx+1:], nil
	}

	// If no "/" in resource, return the whole resource
	return parsedARN.Resource, nil
}

// extractPartitionFromARN extracts the partition from an AWS ARN
// Returns the partition (e.g., "aws", "aws-cn", "aws-us-gov")
func extractPartitionFromARN(arnString string) (string, error) {
	parsedARN, err := arn.Parse(arnString)
	if err != nil {
		return "", fmt.Errorf("failed to parse ARN %q: %w", arnString, err)
	}

	return parsedARN.Partition, nil
}

// buildARN constructs an AWS ARN using the provided components
// and returns the canonical string representation
func buildARN(partition, service, region, accountID, resource string) string {
	arnStruct := arn.ARN{
		Partition: partition,
		Service:   service,
		Region:    region,
		AccountID: accountID,
		Resource:  resource,
	}

	return arnStruct.String()
}
