package ecs

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
)

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

func getClusterArns(ctx context.Context, svc *ecs.Client) ([]string, error) {
	response, err := svc.ListClusters(ctx, &ecs.ListClustersInput{})
	if err != nil {
		return nil, fmt.Errorf("failed to list clusters: %w", err)
	}

	return response.ClusterArns, nil
}

func getServiceArns(ctx context.Context, svc *ecs.Client, cluster string) ([]string, error) {
	response, err := svc.ListServices(ctx, &ecs.ListServicesInput{
		Cluster: &cluster,
	})

	if err != nil {
		return nil, fmt.Errorf("failed to list services in cluster %s: %w", cluster, err)
	}

	return response.ServiceArns, nil
}

func getTaskDetails(ctx context.Context, svc *ecs.Client, cluster string, service string) ([]TaskInfo, error) {
	listTasksOutput, err := svc.ListTasks(ctx, &ecs.ListTasksInput{
		Cluster:     aws.String(cluster),
		ServiceName: aws.String(service),
	})

	if err != nil {
		return nil, fmt.Errorf("failed to list tasks for service %s in cluster %s: %w", service, cluster, err)
	}

	arns := append([]string{}, listTasksOutput.TaskArns...)

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

		taskID := extractLastPart(*task.TaskArn)

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
func GetClusters(includeTasks bool) ([]ClusterInfo, error) {
	cfg, err := initCfg()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize AWS configuration: %w", err)
	}

	ctx := context.Background()

	svc := ecs.NewFromConfig(cfg)
	clusterArns, err := getClusterArns(ctx, svc)
	if err != nil {
		return nil, fmt.Errorf("failed to list clusters: %w", err)
	}

	clusters := []ClusterInfo{}
	for _, clusterArn := range clusterArns {
		serviceArns, err := getServiceArns(ctx, svc, clusterArn)
		if err != nil {
			return nil, fmt.Errorf("failed to list services for cluster %s: %w", clusterArn, err)
		}

		services := []ServiceInfo{}
		for _, serviceArn := range serviceArns {
			parts := strings.Split(serviceArn, "/")
			clusterName := parts[1]
			serviceName := parts[2]

			service := ServiceInfo{
				Name:        serviceName,
				ClusterName: clusterName,
				Tasks:       []TaskInfo{},
			}

			if includeTasks {
				tasks, err := getTaskDetails(ctx, svc, clusterName, serviceName)
				if err != nil {
					return nil, fmt.Errorf("failed to list tasks for service %s/%s: %w", clusterName, serviceName, err)
				}
				service.Tasks = tasks
			}

			services = append(services, service)
		}

		// Extract cluster name from ARN
		clusterParts := strings.Split(clusterArn, "/")
		clusterName := clusterParts[len(clusterParts)-1]

		clusters = append(clusters, ClusterInfo{
			Name:     clusterName,
			Services: services,
		})
	}

	return clusters, nil
}
