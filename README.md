# Overview

RunECS is a tool for running one-off processes in an ECS cluster. The tool was created as a simple solution for occasional running of processes in the ECS cluster - e.g. various data migrations. Currently only the FARGATE launch type is supported.

The process can be started asynchronously or synchronously with the `-w` parameter.

## How to Use

You need to define a profile to specify the target environment in which to run the task.

The profile values can be defined using environment variables or saved in yaml format in the `~/.runecs/profiles` folder under the name the profile will use.

Save the target environment specification to a file `~/.runecs/profiles/myservice.yml`

```yaml
AwsProfile: myprofile
AwsRegion: eu-west-1
Cluster: mycluster
Service: myservice
```

or use environment variables with profile 

```shell
export AWS_PROFILE=
export AWS_REGION=
export CLUSTER=
export SERVICE=
```

or without profile

```shell
export AWS_ACCESS_KEY_ID=
export AWS_SECRET_ACCESS_KEY=
export AWS_REGION=
export CLUSTER=
export SERVICE=
```

### Profiles vs Environment Variables

The target environment specified by the profile must use the [named profile](https://docs.aws.amazon.com/cli/latest/userguide/cli-configure-profiles.html) for authorization. Environment variables support using variables `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY` and `AWS_DEFAULT_REGION`.

## Commands

### RUN

Executing a one-off process:

```shell
runecs run rake db:migrate
runecs run rake db:migrate --profile myservice
```

### DEREGISTER

Deregisters all task definitions of all available families in the cluster, and keeps only the latest. See [AWS documentation](https://docs.aws.amazon.com/AmazonECS/latest/APIReference/API_DeregisterTaskDefinition.html).

```shell
runecs deregister
runecs deregister --profile myservice
```

# Build

```shell
git clone git@github.com:meap/runecs.git

cd runecs
make

./bin/runecs --help
```

