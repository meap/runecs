package ecs

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/ecs"
)

func (s *Service) listClusters(ctx context.Context, svc *ecs.Client) []string {
	response, err := svc.ListClusters(ctx, &ecs.ListClustersInput{})
	if err != nil {
		log.Println(fmt.Errorf("failed to list clusters. (%v)", err))
		return []string{}
	}

	return response.ClusterArns
}

func (s *Service) listServices(ctx context.Context, svc *ecs.Client, cluster string) []string {
	response, err := svc.ListServices(ctx, &ecs.ListServicesInput{
		Cluster: &cluster,
	})

	if err != nil {
		log.Println(fmt.Errorf("failed to list services in cluster %s. (%v)", cluster, err))
		return []string{}
	}

	return response.ServiceArns
}

func (s *Service) List() {
	cfg, err := s.initCfg()
	if err != nil {
		log.Fatalln(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	links := []string{}

	svc := ecs.NewFromConfig(cfg)
	clusters := s.listClusters(ctx, svc)
	for _, clusterArn := range clusters {
		services := s.listServices(ctx, svc, clusterArn)
		for _, serviceArn := range services {
			parts := strings.Split(serviceArn, "/")
			links = append(links, fmt.Sprintf("%s/%s", parts[1], parts[2]))
		}
	}

	fmt.Println()
	for _, link := range links {
		fmt.Println(link)
	}
}
