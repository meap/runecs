## Overview

RunECS is a tool for running one-off processes in an ECS cluster. The tool was created as a simple solution for occasional running of processes in the ECS cluster - e.g. various data migrations. Currently only the FARGATE launch type is supported.

## How to Use

The ECS cluster settings are located in the `~/.runecs.yml` file, which is located in the user's home directory. The default profile is called `Default` and is automatically used unless explicitly specified otherwise.

```yaml
Profiles:
  Default:
    AwsProfile: myprofile
    AwsRegion: eu-west-1
    Cluster: mycluster
    Service: myservice
```

Authorization in AWS is done using [named profiles](https://docs.aws.amazon.com/cli/latest/userguide/cli-configure-profiles.html).

Launching a one-off process:

```shell
runecs rake db:migrate
runecs rake db:migrate --profile Default
```

## Build

```shell
git clone git@github.com:meap/runecs.git
make

./bin/runecs --help
```

