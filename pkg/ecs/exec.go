package ecs

import (
	"context"
	"fmt"
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
	Name            string
	LogGroup        string
	LogStreamPrefix string
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
	fmt.Printf("Service '%s' loaded. \n", *def.ServiceName)

	// Fetch the latest task definition. Keep in mind that the active service may
	// have a different task definition that is available, see. *def.TaskDefinition
	//
	// TODO: Define by CLI input parameter?
	taskDef, err := s.latestTaskDefinition(client)
	if err != nil {
		return serviceDef{}, err
	}

	return serviceDef{
		Subnets:        def.NetworkConfiguration.AwsvpcConfiguration.Subnets,
		SecurityGroups: def.NetworkConfiguration.AwsvpcConfiguration.SecurityGroups,
		TaskDef:        taskDef,
	}, nil
}

func (s *Service) describeTask(client *ecs.Client, taskArn *string) (taskDef, error) {
	resp, _ := client.DescribeTaskDefinition(context.TODO(), &ecs.DescribeTaskDefinitionInput{TaskDefinition: taskArn})

	if len(resp.TaskDefinition.ContainerDefinitions) == 0 {
		return taskDef{}, fmt.Errorf("no container definitions found in task definition %s", *taskArn)
	}

	output := taskDef{}
	output.Name = *resp.TaskDefinition.ContainerDefinitions[0].Name

	logConfig := resp.TaskDefinition.ContainerDefinitions[0].LogConfiguration
	if logConfig != nil && logConfig.LogDriver == types.LogDriverAwslogs {
		output.LogGroup = logConfig.Options["awslogs-group"]
		output.LogStreamPrefix = logConfig.Options["awslogs-stream-prefix"]
	}

	return output, nil
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

	tdef, err := s.describeTask(svc, &sdef.TaskDef)
	if err != nil {
		log.Fatalf("An error occurred while loading the task definition %s: %v", sdef.TaskDef, err)
	}

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

	executedTask := output.Tasks[0]

	log.Printf("Task %s executed", *executedTask.TaskArn)
	if wait {
		for {
			success, err := s.wait(svc, *executedTask.TaskArn)
			if err != nil {
				logsErr := s.printProcessLogs(tdef.LogGroup, tdef.LogStreamPrefix, *executedTask.TaskArn, tdef.Name)
				if logsErr != nil {
					log.Printf("Failed to fetch events from CloudWatch %v", logsErr)
				}
				log.Fatalln(err)
			}

			if success {
				break
			}

			time.Sleep(6 * time.Second)
		}

		log.Printf("task %s finished", *executedTask.TaskArn)
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

	if *output.Tasks[0].LastStatus == "STOPPED" {
		if exitCode := *output.Tasks[0].Containers[0].ExitCode; exitCode != 0 {
			return true, fmt.Errorf("task %s failed with exit code %d", *output.Tasks[0].TaskArn, exitCode)
		}
	}

	return *output.Tasks[0].LastStatus == "STOPPED", nil
}
