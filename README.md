## Overview

RunECS is a tool for running one-off processes in an ECS cluster. The tool was created as a simple solution for occasional running of processes in the ECS cluster - e.g. various data migrations. Currently only the FARGATE launch type is supported.

The process can be started asynchronously (does not wait for finish) or synchronously with the `-w` parameter (waits for task finish).

## How to Use

The ECS cluster settings are located in the `~/.runecs.yml` file, which is located in the user's home directory. The default profile is called `default` and is automatically used unless explicitly specified otherwise.

```yaml
Profiles:
  default:
    AwsProfile: myprofile
    AwsRegion: eu-west-1
    Cluster: mycluster
    Service: myservice
```

Authorization in AWS is done using [named profiles](https://docs.aws.amazon.com/cli/latest/userguide/cli-configure-profiles.html).

### Execute command

Executing a one-off process:

```shell
runecs run rake db:migrate
runecs run rake db:migrate --profile default
```

### Deregister task definition

Deregisters all task definitions of all available families in the cluster, and keeps only the latest. See [AWS documentation](https://docs.aws.amazon.com/AmazonECS/latest/APIReference/API_DeregisterTaskDefinition.html).

```shell
runecs deregister
runecs deregister --profile default
```

## Build

```shell
git clone git@github.com:meap/runecs.git

cd runecs
make

./bin/runecs --help
```

