package ecs

import (
	"context"
	"fmt"
	"log"
	"strings"

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

func List() {
	cfg, err := initCfg()
	if err != nil {
		log.Fatalln(err)
	}

	ctx := context.Background()

	links := []string{}

	svc := ecs.NewFromConfig(cfg)
	clusters := listClusters(ctx, svc)

	for _, clusterArn := range clusters {
		services := listServices(ctx, svc, clusterArn)
		for _, serviceArn := range services {
			parts := strings.Split(serviceArn, "/")
			links = append(links, fmt.Sprintf("%s/%s", parts[1], parts[2]))
		}
	}

	for _, link := range links {
		fmt.Println(link)
	}
}
