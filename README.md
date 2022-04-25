# Overview

RunECS is a tool for running one-off processes in an ECS cluster. The tool was created as a simple solution for occasional running of processes in the ECS cluster - e.g. various data migrations. 

## Limitations

* Only FARGATE launch type is supported.
* One container in the task.

## How to Use

RunECS works in a cluster with the service. You must define `--cluster` and `--service`. In this case, AWS authorization can be addressed by environment variables:

```
export AWS_ACCESS_KEY_ID=
export AWS_SECRET_ACCESS_KEY=
export AWS_REGION=
```

### Profiles

You can have individual service settings stored in profiles. Profile is a file with the following structure:

```yaml
AwsProfile: myprofile
AwsRegion: eu-west-1
Cluster: mycluster
Service: myservice
```

`AwsProfile` says that the AWS [named profile](https://docs.aws.amazon.com/cli/latest/userguide/cli-configure-profiles.html) is used.

The profile name determines the name of the environment settings file.The file must be placed at `~/.runecs/profiles/myservice.yml`. In this case, the profile is named `myservice`.

Usage:

```shell
runecs revisions --profile myservice --last 10
```

## Commands

### Run

The process runs using the last task definition. Using the `--image-tag` parameter creates a new task definition with which to run the process.

The task is run asynchronously by default. Using the `--wait` argument, the task starts synchronously and returns the EXIT code of the container.

Executing a one-off process asynchronously:

```shell
runecs run rake db:migrate \
  --cluster mycluster --service myservice \
  --image-tag latest
````

Executing a one-off process synchronously:

```shell
runecs run rake db:migrate \
  --cluster mycluster --service myservice \
  --image-tag latest \
  --wait
```

### Prune

[Deregisters](https://docs.aws.amazon.com/AmazonECS/latest/APIReference/API_DeregisterTaskDefinition.html) old task definitions.

Use `--keep-last` and `--keep-days` to ensure that a certain number of definitions are always available.

```shell
runecs prune \
  --cluster mycluster --service myservice \
  --keep-last 10 --keep-days 5
```

### Deploy

Creates a new task definition with the specified image tag and updates the service to use the created task definition for new tasks.

```shell
runecs deploy \
  --cluster mycluster --service myservice \
  --image-tag latest
```

### Revisions

Prints a list of revisions of the task definition. Sorted from newest to oldest. Displays the Docker image URI used in the revision.

```shell
runecs revisions \
  --cluster mycluster --service myservice \
  --last 10
```

# Build

```shell
git clone git@github.com:meap/runecs.git

cd runecs
make

./bin/runecs --help
```
