<div align="center">

<img width="106px" alt="elastic container service logo" src="images/amazon_ecs-icon.svg">

# RunECS CLI - Simplified AWS ECS Run Task Wrapper

[![GitHub release (latest by date)](https://img.shields.io/github/v/release/meap/runecs?logo=GitHub)](https://github.com/meap/runecs/releases)
[![GitHub all releases](https://img.shields.io/github/downloads/meap/runecs/total?label=all%20time%20downloads)](https://github.com/meap/runecs/releases/)

RunECS: Effortlessly Execute One-Off Tasks and Database Migrations in Your ECS Cluster. A developer-friendly wrapper for the AWS ECS run task command.

</div>

<p>
    <img src="./images/demo.gif" width="100%" alt="RunECS - Simplified AWS ECS run task demonstration">
</p>

## What is RunECS?

RunECS is a command-line tool that simplifies running one-off tasks in Amazon ECS (Elastic Container Service). It wraps the standard `aws ecs run-task` command functionality with a more developer-friendly interface, providing intuitive commands for common ECS operations while leveraging AWS's underlying infrastructure.

# Install

## 🍺 Homebrew

```shell
brew tap meap/runecs
brew install runecs
```

## Using Docker

The easiest way to get started with runecs using Docker is by running this command.

```
docker run \
  -e AWS_ACCESS_KEY_ID=$AWS_ACCESS_KEY_ID \
  -e AWS_SECRET_ACCESS_KEY=$AWS_SECRET_ACCESS_KEY \
  -e AWS_REGION=$AWS_REGION \
  preichl/runecs list
```

> Note: You have to pass the environment variables with AWS credentials. I recommend using [direnv](https://direnv.net/) which I mentioned in the [introduction post](https://dev.to/preichl/streamline-your-ecs-workflow-easy-database-migrations-with-a-runecs-cli-tool-5d2h).

## 📦 Other way

Download the binary file for your platform, [see releases](https://github.com/meap/runecs/releases).


# How to Use

RunECS simplifies the process of executing commands using AWS ECS run task functionality. The service must be specified in `cluster/service` format. Further, you must specify the environment variables that determine access to AWS.

```
export AWS_ACCESS_KEY_ID=xxxxxx
export AWS_SECRET_ACCESS_KEY=xxxxxxx
export AWS_REGION=eu-east1

runecs rake orders:upload[14021] --service my-cluster/my-service -w
```

## Commands

### Run (AWS ECS Run Task Made Simple)

The `run` command is a streamlined wrapper around the AWS ECS run task API. It executes the process using the last available task definition. You can pick a specific docker tag by using the `--image-tag` argument. In this case, it changes the task definition and inserts the specified docker tag. 

The task is run asynchronously by default. Using the `--wait` argument, the task starts synchronously and returns the EXIT code of the container.

Executing a one-off process asynchronously:

```shell
runecs run rake db:migrate \
  --service my-cluster/my-service \
  --image-tag latest
```

Executing a one-off process synchronously:

```shell
runecs run rake db:migrate \
  --service my-cluster/my-service \
  --image-tag latest \
  --wait
```

### List

The main parameter is the name of the ECS service within which the command is to be executed. The parameter value consists of the cluster name and its service. To make it easier, we have introduced a command **list** that lists all these services in the specified region.

```shell
runecs list
```

To include the tasks of each service in the output, use the `--all` parameter. This will display the tasks currently running in each service.

```shell
runecs list --all
```

### Restart

Restarts all running tasks in the service.

```shell
runecs restart --service my-cluster/myservice
```

The services restart without downtime. The command initiates new tasks using the last definition. After reaching the running state, ECS automatically shuts down old tasks to achieve the desired number of running tasks.

Another option is to use the `--kill` parameter, which shuts down running tasks in the service. If the service has health checks set up properly, ECS automatically starts new tasks to ensure that the desired number of tasks are running.

```shell
runecs restart --service my-cluster/myservice --kill
```

### Prune

[Deregisters](https://docs.aws.amazon.com/AmazonECS/latest/APIReference/API_DeregisterTaskDefinition.html) old task definitions.

Use `--keep-last` and `--keep-days` to ensure that a certain number of definitions are always available.

```shell
runecs prune \
  --service my-cluster/my-service \
  --keep-last 10 --keep-days 5
```

### Deploy

Creates a new task definition with the specified image tag and updates the service to use the created task definition for new tasks.

```shell
runecs deploy \
  --service my-cluster/my-service \
  --image-tag latest
```

### Revisions

Prints a list of revisions of the task definition. Sorted from newest to oldest. Displays the Docker image URI used in the revision.

```shell
runecs revisions \
  --service my-cluster/my-service \
  --last 10
```

## Limitations

* Only FARGATE launch type is supported.
* Sidecar containers are not supported

# License

RunECS is distributed under [Apache-2.0 license](https://github.com/meap/runecs/blob/main/LICENSE)
