package ecs

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
)

func listClusters(ctx context.Context, svc *ecs.Client) []string {
	response, err := svc.ListClusters(ctx, &ecs.ListClustersInput{})
	if err != nil {
		log.Println(fmt.Errorf("failed to list clusters. (%w)", err))

		return []string{}
	}

	return response.ClusterArns
}

func listServices(ctx context.Context, svc *ecs.Client, cluster string) []string {
	response, err := svc.ListServices(ctx, &ecs.ListServicesInput{
		Cluster: &cluster,
	})

	if err != nil {
		log.Println(fmt.Errorf("failed to list services in cluster %s. (%w)", cluster, err))

		return []string{}
	}

	return response.ServiceArns
}

func listTasks(ctx context.Context, svc *ecs.Client, cluster string, service string) []string {
	listTasksOutput, err := svc.ListTasks(ctx, &ecs.ListTasksInput{
		Cluster:     aws.String(cluster),
		ServiceName: aws.String(service),
	})

	if err != nil {
		log.Printf("failed to list tasks for service %s in cluster %s. (%v)", service, cluster, err)

		return []string{}
	}

	arns := []string{}
	for _, taskArn := range listTasksOutput.TaskArns {
		arns = append(arns, taskArn)
	}

	describeTasksOutput, err := svc.DescribeTasks(ctx, &ecs.DescribeTasksInput{
		Cluster: aws.String(cluster),
		Tasks:   arns,
	})
	if err != nil {
		log.Printf("failed to describe tasks for service %s in cluster %s. (%v)", service, cluster, err)

		return []string{}
	}

	output := []string{}
	for _, task := range describeTasksOutput.Tasks {
		// Výpočet délky běhu úlohy
		var runningTime string
		if task.StartedAt != nil {
			duration := time.Since(*task.StartedAt)
			runningTime = fmt.Sprintf("%dh %dm %ds", int(duration.Hours()), int(duration.Minutes())%60, int(duration.Seconds())%60)
		} else {
			runningTime = "Unknown"
		}

		output = append(output, fmt.Sprintf(
			"%s: %s (Cpu) / %s (Memory) (Running for: %s)",
			*task.TaskArn,
			*task.Cpu,
			*task.Memory,
			runningTime,
		))
	}

	return output
}

func List(includeTasks bool) {
	cfg, err := initCfg()
	if err != nil {
		log.Fatalln(err)
	}

	ctx := context.Background()

	svc := ecs.NewFromConfig(cfg)
	clusters := listClusters(ctx, svc)

	for _, clusterArn := range clusters {
		services := listServices(ctx, svc, clusterArn)
		for _, serviceArn := range services {
			parts := strings.Split(serviceArn, "/")
			link := fmt.Sprintf("%s/%s", parts[1], parts[2])
			tasks := listTasks(ctx, svc, parts[1], parts[2])

			fmt.Println(link)
			if includeTasks {
				for _, task := range tasks {
					fmt.Printf("  %s\n", task)
				}
			}
		}
	}
}
