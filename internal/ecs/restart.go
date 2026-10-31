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
	"time"

	"github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/aws/aws-sdk-go-v2/service/ecs/types"
)

// restartWaitTimeout caps how long --wait blocks. Without a deployment
// circuit breaker a broken rollout never finishes, so CI runs need a bound.
const restartWaitTimeout = 30 * time.Minute

// restartWaitPollInterval is the cadence at which deployment state is polled.
const restartWaitPollInterval = 10 * time.Second

// describeServicesBatchSize is the maximum number of services DescribeServices accepts per call.
const describeServicesBatchSize = 10

func stopAll(ctx context.Context, cluster, service string, client *ecs.Client) ([]StoppedTaskInfo, error) {
	tasks, err := client.ListTasks(ctx, &ecs.ListTasksInput{
		Cluster:     &cluster,
		ServiceName: &service,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list tasks: %w", err)
	}

	var stoppedTasks []StoppedTaskInfo
	for _, taskArn := range tasks.TaskArns {
		output, err := client.StopTask(ctx, &ecs.StopTaskInput{
			Cluster: &cluster,
			Task:    &taskArn,
		})
		if err != nil {
			// Continue with other tasks but log this error
			continue
		}

		if output.Task == nil {
			continue
		}

		if output.Task.TaskArn == nil || output.Task.StartedAt == nil {
			continue
		}

		stoppedTasks = append(stoppedTasks, StoppedTaskInfo{
			TaskArn:   *output.Task.TaskArn,
			StartedAt: *output.Task.StartedAt,
		})
	}

	return stoppedTasks, nil
}

// forceNewDeploy starts a new deployment of the service and returns the
// service ARN, the task definition ARN and the ID of the new PRIMARY deployment.
func forceNewDeploy(ctx context.Context, cluster, service string, client *ecs.Client) (string, string, string, error) {
	taskDef, err := latestTaskDefinitionArn(ctx, cluster, service, client)
	if err != nil {
		return "", "", "", err
	}

	output, err := client.UpdateService(ctx, &ecs.UpdateServiceInput{
		Cluster:            &cluster,
		Service:            &service,
		TaskDefinition:     &taskDef,
		ForceNewDeployment: true,
	})

	if err != nil {
		return "", "", "", fmt.Errorf("failed to update service: %w", err)
	}

	if output.Service == nil {
		return "", "", "", errors.New("invalid service update response: missing service")
	}

	if output.Service.ServiceArn == nil {
		return "", "", "", errors.New("invalid service update response: missing service ARN")
	}

	if output.Service.TaskDefinition == nil {
		return "", "", "", errors.New("invalid service update response: missing task definition")
	}

	var deploymentID string
	for _, d := range output.Service.Deployments {
		if d.Status != nil && *d.Status == "PRIMARY" && d.Id != nil {
			deploymentID = *d.Id

			break
		}
	}

	if deploymentID == "" {
		return "", "", "", errors.New("invalid service update response: missing primary deployment")
	}

	return *output.Service.ServiceArn, *output.Service.TaskDefinition, deploymentID, nil
}

// checkDeployment reports whether the tracked deployment of a service has
// finished. It returns an error when the deployment failed, was rolled back
// or superseded, or the service is being deleted.
func checkDeployment(svc types.Service, deploymentID string) (bool, error) {
	if svc.Status != nil && (*svc.Status == "DRAINING" || *svc.Status == "INACTIVE") {
		return false, fmt.Errorf("service is %s", *svc.Status)
	}

	var deployment *types.Deployment
	for i := range svc.Deployments {
		if svc.Deployments[i].Id != nil && *svc.Deployments[i].Id == deploymentID {
			deployment = &svc.Deployments[i]

			break
		}
	}

	if deployment == nil || deployment.Status == nil || *deployment.Status != "PRIMARY" {
		return false, fmt.Errorf("deployment %s was rolled back or superseded", deploymentID)
	}

	switch deployment.RolloutState {
	case types.DeploymentRolloutStateCompleted:
		return true, nil
	case types.DeploymentRolloutStateFailed:
		reason := ""
		if deployment.RolloutStateReason != nil {
			reason = ": " + *deployment.RolloutStateReason
		}

		return false, fmt.Errorf("deployment %s failed%s", deploymentID, reason)
	case types.DeploymentRolloutStateInProgress:
		return false, nil
	default:
		// RolloutState is not reported for services behind a Classic Load Balancer.
		return len(svc.Deployments) == 1 && svc.RunningCount == svc.DesiredCount, nil
	}
}

// pollDeployments describes the pending services once, prints progress and
// removes finished services from pending. Failures are recorded in errs.
func pollDeployments(ctx context.Context, client *ecs.Client, cluster string, pending map[string]string, progress map[string]string, errs map[string]error) error {
	services := make([]string, 0, len(pending))
	for service := range pending {
		services = append(services, service)
	}

	for start := 0; start < len(services); start += describeServicesBatchSize {
		batch := services[start:min(start+describeServicesBatchSize, len(services))]

		output, err := client.DescribeServices(ctx, &ecs.DescribeServicesInput{
			Cluster:  &cluster,
			Services: batch,
		})
		if err != nil {
			return fmt.Errorf("failed to describe services: %w", err)
		}

		found := make(map[string]bool, len(batch))
		for _, svc := range output.Services {
			if svc.ServiceName == nil {
				continue
			}

			service := *svc.ServiceName
			deploymentID, ok := pending[service]
			if !ok {
				continue
			}
			found[service] = true

			current := fmt.Sprintf("running %d/%d", svc.RunningCount, svc.DesiredCount)
			if progress[service] != current {
				fmt.Printf("Service %s: %s\n", service, current)
				progress[service] = current
			}

			done, err := checkDeployment(svc, deploymentID)
			if err != nil {
				errs[service] = err
				delete(pending, service)
			} else if done {
				fmt.Printf("Service %s: new tasks are running.\n", service)
				delete(pending, service)
			}
		}

		for _, service := range batch {
			if !found[service] {
				errs[service] = errors.New("service not found while waiting")
				delete(pending, service)
			}
		}
	}

	return nil
}

// waitForDeployments polls until every tracked deployment (service name ->
// deployment ID) completes or fails. The returned map holds an error for each
// service that did not finish successfully.
func waitForDeployments(ctx context.Context, client *ecs.Client, cluster string, deployments map[string]string) map[string]error {
	ctx, cancel := context.WithTimeout(ctx, restartWaitTimeout)
	defer cancel()

	errs := make(map[string]error)
	progress := make(map[string]string)
	pending := make(map[string]string, len(deployments))
	for service, id := range deployments {
		pending[service] = id
	}

	fmt.Println("Waiting for new tasks to be running... Press CTRL+C to stop waiting (deployment will continue).")

	for {
		err := pollDeployments(ctx, client, cluster, pending, progress, errs)
		if err != nil && ctx.Err() == nil {
			for service := range pending {
				errs[service] = err
			}

			return errs
		}

		if len(pending) == 0 {
			return errs
		}

		select {
		case <-ctx.Done():
		case <-time.After(restartWaitPollInterval):
		}

		if ctx.Err() != nil {
			waitErr := fmt.Errorf("stopped waiting (deployment continues): %w", ctx.Err())
			if errors.Is(ctx.Err(), context.DeadlineExceeded) {
				waitErr = fmt.Errorf("timed out after %s waiting for new tasks (deployment continues)", restartWaitTimeout)
			}

			for service := range pending {
				errs[service] = waitErr
			}

			return errs
		}
	}
}

func RestartCluster(ctx context.Context, clients *AWSClients, cluster string, kill, wait bool) (*ClusterRestartResult, error) {
	serviceArns, err := getServiceArns(ctx, clients.ECS, cluster)
	if err != nil {
		return nil, err
	}

	result := &ClusterRestartResult{
		Cluster:  cluster,
		Services: nil,
	}

	for _, serviceArn := range serviceArns {
		serviceName, err := extractARNResource(serviceArn)
		if err != nil {
			return nil, fmt.Errorf("failed to extract service name from ARN %s: %w", serviceArn, err)
		}

		// Trigger all deployments first, then wait for them together below.
		restartResult, err := Restart(ctx, clients, cluster, serviceName, kill, false)
		result.Services = append(result.Services, ServiceRestartResult{
			Service: serviceName,
			Result:  restartResult,
			Err:     err,
		})
	}

	if !wait || kill {
		return result, nil
	}

	deployments := make(map[string]string)
	for _, svc := range result.Services {
		if svc.Err == nil {
			deployments[svc.Service] = svc.Result.DeploymentID
		}
	}

	if len(deployments) == 0 {
		return result, nil
	}

	errs := waitForDeployments(ctx, clients.ECS, cluster, deployments)
	for i := range result.Services {
		if err, ok := errs[result.Services[i].Service]; ok {
			result.Services[i].Err = err
		}
	}

	return result, nil
}

func Restart(ctx context.Context, clients *AWSClients, cluster, service string, kill, wait bool) (*RestartResult, error) {
	result := &RestartResult{}

	if kill {
		result.Method = "kill"

		stoppedTasks, err := stopAll(ctx, cluster, service, clients.ECS)
		if err != nil {
			return nil, fmt.Errorf("failed to stop tasks: %w", err)
		}
		result.StoppedTasks = stoppedTasks

		return result, nil
	}

	result.Method = "force_deploy"

	serviceArn, taskDefinition, deploymentID, err := forceNewDeploy(ctx, cluster, service, clients.ECS)
	if err != nil {
		return nil, fmt.Errorf("failed to force new deployment: %w", err)
	}
	result.ServiceArn = serviceArn
	result.TaskDefinition = taskDefinition
	result.DeploymentID = deploymentID

	if wait {
		if err := waitForDeployments(ctx, clients.ECS, cluster, map[string]string{service: deploymentID})[service]; err != nil {
			return result, err
		}
	}

	return result, nil
}
