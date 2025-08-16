package ecs

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
)

func getClusterArns(ctx context.Context, svc *ecs.Client) ([]string, error) {
	var clusterArns []string
	input := &ecs.ListClustersInput{}

	for {
		response, err := svc.ListClusters(ctx, input)
		if err != nil {
			return nil, fmt.Errorf("failed to list clusters: %w", err)
		}

		clusterArns = append(clusterArns, response.ClusterArns...)

		if response.NextToken == nil {
			break
		}

		input.NextToken = response.NextToken
	}

	return clusterArns, nil
}

func getServiceArns(ctx context.Context, svc *ecs.Client, cluster string) ([]string, error) {
	var serviceArns []string
	input := &ecs.ListServicesInput{
		Cluster: &cluster,
	}

	for {
		response, err := svc.ListServices(ctx, input)
		if err != nil {
			return nil, fmt.Errorf("failed to list services in cluster %s: %w", cluster, err)
		}

		serviceArns = append(serviceArns, response.ServiceArns...)

		if response.NextToken == nil {
			break
		}

		input.NextToken = response.NextToken
	}

	return serviceArns, nil
}

func getTaskDetails(ctx context.Context, svc *ecs.Client, cluster string, service string) ([]TaskInfo, error) {
	var allTaskArns []string
	input := &ecs.ListTasksInput{
		Cluster:     aws.String(cluster),
		ServiceName: aws.String(service),
	}

	for {
		listTasksOutput, err := svc.ListTasks(ctx, input)
		if err != nil {
			return nil, fmt.Errorf("failed to list tasks for service %s in cluster %s: %w", service, cluster, err)
		}

		allTaskArns = append(allTaskArns, listTasksOutput.TaskArns...)

		if listTasksOutput.NextToken == nil {
			break
		}

		input.NextToken = listTasksOutput.NextToken
	}

	arns := append([]string{}, allTaskArns...)

	describeTasksOutput, err := svc.DescribeTasks(ctx, &ecs.DescribeTasksInput{
		Cluster: aws.String(cluster),
		Tasks:   arns,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to describe tasks for service %s in cluster %s: %w", service, cluster, err)
	}

	tasks := []TaskInfo{}
	for _, task := range describeTasksOutput.Tasks {
		// Výpočet délky běhu úlohy
		var runningTime string
		if task.StartedAt != nil {
			duration := time.Since(*task.StartedAt)
			runningTime = formatRunningTime(duration)
		} else {
			runningTime = "Unknown"
		}

		taskID, err := extractARNResource(*task.TaskArn)
		if err != nil {
			return nil, fmt.Errorf("failed to extract task ID from ARN: %w", err)
		}

		tasks = append(tasks, TaskInfo{
			ID:          taskID,
			CPU:         *task.Cpu,
			Memory:      *task.Memory,
			RunningTime: runningTime,
		})
	}

	return tasks, nil
}

// GetClusters returns structured data about ECS clusters, services, and optionally tasks
func GetClusters(ctx context.Context, clients *AWSClients, includeTasks bool) ([]ClusterInfo, error) {
	clusterArns, err := getClusterArns(ctx, clients.ECS)
	if err != nil {
		return nil, fmt.Errorf("failed to list clusters: %w", err)
	}

	clusters := []ClusterInfo{}
	for _, clusterArn := range clusterArns {
		serviceArns, err := getServiceArns(ctx, clients.ECS, clusterArn)
		if err != nil {
			return nil, fmt.Errorf("failed to list services for cluster %s: %w", clusterArn, err)
		}

		services := []ServiceInfo{}
		for _, serviceArn := range serviceArns {
			serviceName, err := extractARNResource(serviceArn)
			if err != nil {
				return nil, fmt.Errorf("failed to extract service name from ARN %s: %w", serviceArn, err)
			}

			clusterName, err := extractARNResource(clusterArn)
			if err != nil {
				return nil, fmt.Errorf("failed to extract cluster name from ARN %s: %w", clusterArn, err)
			}

			service := ServiceInfo{
				Name:        serviceName,
				ClusterName: clusterName,
				Tasks:       []TaskInfo{},
			}

			if includeTasks {
				tasks, err := getTaskDetails(ctx, clients.ECS, clusterName, serviceName)
				if err != nil {
					return nil, fmt.Errorf("failed to list tasks for service %s/%s: %w", clusterName, serviceName, err)
				}
				service.Tasks = tasks
			}

			services = append(services, service)
		}

		// Extract cluster name from ARN
		clusterName, err := extractARNResource(clusterArn)
		if err != nil {
			return nil, fmt.Errorf("failed to extract cluster name from ARN %s: %w", clusterArn, err)
		}

		clusters = append(clusters, ClusterInfo{
			Name:     clusterName,
			Services: services,
		})
	}

	return clusters, nil
}
