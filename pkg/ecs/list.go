package ecs

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/ecs"
)

func (s *Service) listClusters(svc *ecs.Client) []string {
	response, err := svc.ListClusters(context.TODO(), &ecs.ListClustersInput{})
	if err != nil {
		log.Println(fmt.Errorf("failed to list clusters. (%v)", err))
		return []string{}
	}

	return response.ClusterArns
}

func (s *Service) listServices(svc *ecs.Client, cluster string) []string {
	response, err := svc.ListServices(context.TODO(), &ecs.ListServicesInput{
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

	links := []string{}

	svc := ecs.NewFromConfig(cfg)
	clusters := s.listClusters(svc)
	for _, clusterArn := range clusters {
		services := s.listServices(svc, clusterArn)
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
