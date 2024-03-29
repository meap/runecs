# Overview

RunECS is a tool for executing one-off processes in an ECS cluster.

## Limitations

* Only FARGATE launch type is supported.
* Sidecar containers are not supported

# Install

## Manual

Download the binary file for your platform, [see releases](https://github.com/meap/runecs/releases).

## Homebrew

```shell
brew tap meap/runecs
brew install runecs
```

# How to Use

RunECS executes the command using the specified service. The service must be specified in `cluster/service` format. Further, you must specify the environment variables that determine access to AWS.

```
export AWS_ACCESS_KEY_ID=xxxxxx
export AWS_SECRET_ACCESS_KEY=xxxxxxx
export AWS_REGION=eu-east1

runecs rake orders:upload[14021] --service my-cluster/my-service -w
```

## Commands

### Run

Executes the process using the last available task definition. You can pick a specific docker tag by using the `--image-tag` argument. In this case, it changes the task definition and inserts the specified docker tag. 

The task is run asynchronously by default. Using the `--wait` argument, the task starts synchronously and returns the EXIT code of the container.

Executing a one-off process asynchronously:

```shell
runecs run rake db:migrate \
  --service my-cluster/my-service \
  --image-tag latest
````

Executing a one-off process synchronously:

```shell
runecs run rake db:migrate \
  --service my-cluster/my-service \
  --image-tag latest \
  --wait
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
