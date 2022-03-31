## Overview

RunECS is a tool for running one-off processes in an ECS cluster. The tool was created as a simple solution for occasional running of processes in the ECS cluster - e.g. various data migrations. Currently only the FARGATE launch type is supported.

The process can be started asynchronously (does not wait for finish) or synchronously with the `-w` parameter (waits for task finish).

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

### Execute command

Executing a one-off process:

```shell
runecs run rake db:migrate
runecs run rake db:migrate --profile myservice
```

### Deregister task definition

Deregisters all task definitions of all available families in the cluster, and keeps only the latest. See [AWS documentation](https://docs.aws.amazon.com/AmazonECS/latest/APIReference/API_DeregisterTaskDefinition.html).

```shell
runecs deregister
runecs deregister --profile myservice
```

## Build

```shell
git clone git@github.com:meap/runecs.git

cd runecs
make

./bin/runecs --help
```

