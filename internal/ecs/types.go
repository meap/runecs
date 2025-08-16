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
	"time"

	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
)

const (
	defaultNumberOfRetries = 10
)

// AWSClients holds initialized AWS service clients
type AWSClients struct {
	ECS            *ecs.Client
	CloudWatchLogs *cloudwatchlogs.Client
	Region         string
}

// TaskDefinition represents task definition metadata
type TaskDefinition struct {
	Name                    string
	LogGroup                string
	LogStreamPrefix         string
	Cpu                     string
	Memory                  string
	RequiresCompatibilities []string
}

// LogEntry represents a single log entry from CloudWatch
type LogEntry struct {
	StreamName string
	Message    string
	Timestamp  int64
}

// ExecuteResult contains the result of task execution
type ExecuteResult struct {
	ServiceName       string
	TaskDefinition    string
	TaskArn           string
	NewTaskDefCreated bool
	Finished          bool
	Logs              []LogEntry
}

// RevisionEntry represents a single task definition revision
type RevisionEntry struct {
	Revision  int32
	CreatedAt time.Time
	DockerURI string
	Family    string
}

// RevisionsResult contains the list of task definition revisions
type RevisionsResult struct {
	Revisions []RevisionEntry
}

// DeployResult contains the result of a deployment operation
type DeployResult struct {
	TaskDefinitionArn string
	ServiceArn        string
}

// RestartResult contains the result of a service restart operation
type RestartResult struct {
	StoppedTasks   []StoppedTaskInfo
	ServiceArn     string
	TaskDefinition string
	Method         string // "kill" or "force_deploy"
}

// StoppedTaskInfo represents information about a stopped task
type StoppedTaskInfo struct {
	TaskArn   string
	StartedAt time.Time
}

// TaskDefinitionPruneEntry represents a task definition processed during pruning
type TaskDefinitionPruneEntry struct {
	Arn     string
	DaysOld int
	Action  string // "kept", "deleted", "skipped"
	Reason  string
	Family  string
}

// PruneResult contains the result of a task definition pruning operation
type PruneResult struct {
	Families       []string
	TotalCount     int
	DeletedCount   int
	KeptCount      int
	SkippedCount   int
	DryRun         bool
	ProcessedTasks []TaskDefinitionPruneEntry
}

// TaskInfo represents an ECS task with its details for listing
type TaskInfo struct {
	ID          string
	CPU         string
	Memory      string
	RunningTime string
}

// ServiceInfo represents an ECS service with its details for listing
type ServiceInfo struct {
	Name        string
	ClusterName string
	Tasks       []TaskInfo
}

// ClusterInfo represents an ECS cluster with its services for listing
type ClusterInfo struct {
	Name     string
	Services []ServiceInfo
}

// ScaleResult contains the result of a service scaling operation
type ScaleResult struct {
	ServiceArn           string
	PreviousDesiredCount int32
	NewDesiredCount      int32
	ClusterName          string
	ServiceName          string
}
