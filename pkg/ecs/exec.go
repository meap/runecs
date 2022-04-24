package ecs

import (
	"context"
	"log"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/aws/aws-sdk-go-v2/service/ecs/types"
)

type serviceDef struct {
	Subnets        []string
	SecurityGroups []string
	TaskDef        string
}

type taskDef struct {
	Name string
}

func (s *Service) describeService(client *ecs.Client) (serviceDef, error) {
	resp, err := client.DescribeServices(context.TODO(), &ecs.DescribeServicesInput{
		Cluster:  &s.Cluster,
		Services: []string{s.Service},
	})

	if err != nil {
		return serviceDef{}, err
	}

	def := resp.Services[0]
	log.Printf("Service '%s' loaded.", *def.ServiceName)

	return serviceDef{
		Subnets:        def.NetworkConfiguration.AwsvpcConfiguration.Subnets,
		SecurityGroups: def.NetworkConfiguration.AwsvpcConfiguration.SecurityGroups,
		TaskDef:        *def.TaskDefinition,
	}, nil
}

func (s *Service) describeTask(client *ecs.Client, taskArn *string) taskDef {
	resp, _ := client.DescribeTaskDefinition(context.TODO(), &ecs.DescribeTaskDefinitionInput{TaskDefinition: taskArn})

	return taskDef{
		Name: *resp.TaskDefinition.ContainerDefinitions[0].Name,
	}
}

func (s *Service) Execute(cmd []string, wait bool, dockerImageTag string) {
	cfg, err := s.initCfg()
	if err != nil {
		log.Fatalln(err)
	}

	svc := ecs.NewFromConfig(cfg)

	sdef, err := s.describeService(svc)
	if err != nil {
		log.Fatalf("An error occurred while loading the service %s in the cluster %s: %v", s.Service, s.Cluster, err)
	}

	tdef := s.describeTask(svc, &sdef.TaskDef)

	var taskDef string

	if dockerImageTag != "" {
		taskDef, err = s.cloneTaskDef(dockerImageTag, svc)
		if err != nil {
			log.Fatalln(err)
		}
		log.Printf("New task definition %s is created", taskDef)
	} else {
		taskDef = sdef.TaskDef
		log.Printf("The task definition %s is used", taskDef)
	}

	output, err := svc.RunTask(context.TODO(), &ecs.RunTaskInput{
		Cluster:        &s.Cluster,
		TaskDefinition: &taskDef,
		LaunchType:     "FARGATE",
		NetworkConfiguration: &types.NetworkConfiguration{
			AwsvpcConfiguration: &types.AwsVpcConfiguration{
				Subnets:        sdef.Subnets,
				SecurityGroups: sdef.SecurityGroups,
				AssignPublicIp: "ENABLED",
			},
		},
		Overrides: &types.TaskOverride{
			ContainerOverrides: []types.ContainerOverride{{
				Name:    &tdef.Name,
				Command: cmd,
			}},
		},
	})

	if err != nil {
		log.Fatalln(err)
	}

	log.Printf("task %s executed", *output.Tasks[0].TaskArn)
	if wait {
		for {
			success, err := s.wait(svc, *output.Tasks[0].TaskArn)
			if err != nil {
				log.Fatal(err)
			}

			if success {
				break
			}

			time.Sleep(6 * time.Second)
		}

		log.Printf("task %s finished", *output.Tasks[0].TaskArn)
	}
}

func (s *Service) wait(client *ecs.Client, task string) (bool, error) {
	output, err := client.DescribeTasks(context.TODO(), &ecs.DescribeTasksInput{
		Cluster: &s.Cluster,
		Tasks:   []string{task},
	})

	if err != nil {
		return false, err
	}

	// if output.Tasks[0].Containers[0].ExitCode != 0 {
	// 	return false, fmt.Errorf("task %s failed", output.Tasks[0].TaskArn)
	// }

	// log.Printf("task %s status: %d", *output.Tasks[0].TaskArn, output.Tasks[0].Containers[0].ExitCode)

	return *output.Tasks[0].LastStatus == "STOPPED", nil
}
