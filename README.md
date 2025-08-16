<div align="center">

# run(ECS)

<p align="center">Effortlessly run tasks and manage your services on AWS ECS.</p>

[![GitHub release (**latest** by date)](https://img.shields.io/github/v/release/meap/runecs?logo=GitHub)](https://github.com/meap/runecs/releases)
[![GitHub all releases](https://img.shields.io/github/downloads/meap/runecs/total?label=all%20time%20downloads)](https://github.com/meap/runecs/releases/)
[![Docker Pulls](https://img.shields.io/docker/pulls/preichl/runecs?logo=docker)](https://hub.docker.com/r/preichl/runecs)

</div>

<p>
    <img src="./images/demo.gif" width="100%" alt="RunECS - Simplified AWS ECS run task demonstration">
</p>

---

## Installation

RunECS is a cross-platform tool available for macOS, Linux, and Windows.

```bash
# Install via Homebrew (macOS/Linux)
brew install meap/runecs/runecs

# Or install from source
go install github.com/meap/runecs@latest
```

Pre-compiled binaries for all platforms are available on our [releases page](https://github.com/meap/runecs/releases).

### Docker

RunECS is also available as a Docker image for containerized execution:

```bash
docker run \
  -e AWS_ACCESS_KEY_ID=$AWS_ACCESS_KEY_ID \
  -e AWS_SECRET_ACCESS_KEY=$AWS_SECRET_ACCESS_KEY \
  -e AWS_REGION=$AWS_REGION \
  preichl/runecs list
```

## Configuration

### AWS Credentials

RunECS supports multiple methods for AWS authentication. See [AWS Authentication](docs/aws-authentication.md) for detailed configuration options.

## Key Features

A complete list of available commands can be obtained by running `runecs --help`. Common use cases are demonstrated below.

### Deploy a Specific Docker Image Tag

A specific Docker image tag or commit SHA can be deployed to an ECS service. This functionality is particularly useful when rollbacks to known-good versions are required or when specific builds need to be deployed:

```bash
runecs deploy --service mycanvas-ecs-staging-cluster/web -i 9cd43549f03faf9bbc0ddc3eba8585f00098b240
```

### Run One-Off Commands in ECS

One-off commands can be executed directly in the ECS environment, making it ideal for database migrations, maintenance tasks, or debugging within configured VPC and security groups. Commands are executed with the same network access, environment variables, and IAM permissions as the services:

```bash
runecs run "echo \"HELLO WORLD\"" -w --service mycanvas-ecs-staging-cluster/web
```

Both AWS Fargate and EC2 capacity providers are supported, with the appropriate launch type being automatically selected based on service configuration. When the `-w` flag is used, task completion is awaited and full output is streamed to the terminal, making it ideal for interactive debugging and migration scripts.

### Restart ECS Services

ECS services can be gracefully restarted without downtime, or immediate task termination can be forced when required:

```bash
runecs restart --service mycanvas-ecs-staging-cluster/addrp
```

By default, a rolling restart is performed, with tasks being replaced one by one to maintain service availability. For situations where immediate task termination is required (such as clearing stuck processes or forcing configuration reloads), the `--kill` flag can be used to terminate all tasks at once, allowing replacements to be spawned according to the service's desired count.

## FAQ

#### How does this differ from AWS CLI?

While AWS CLI offers comprehensive control over ECS clusters, its extensive feature set can introduce significant complexity for everyday tasks. RunECS streamlines common ECS operations by focusing on the workflows developers use most frequently, providing an intuitive and efficient command-line experience without sacrificing functionality.
