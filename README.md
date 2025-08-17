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

You can also use RunECS as a Docker image for containerized execution:

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

Run `runecs --help` to see all available commands. The examples below demonstrate common use cases.

### Deploy a Specific Docker Image Tag

Deploy a specific Docker image tag or commit SHA to an ECS service. Use this feature for rollbacks to known-good versions or deploying specific builds:

```bash
runecs deploy --service mycanvas-ecs-staging-cluster/web -i 9cd43549f03faf9bbc0ddc3eba8585f00098b240
```

### Run One-Off Commands in ECS

Execute one-off commands directly in the ECS environment. This makes database migrations, maintenance tasks, and debugging ideal within configured VPC and security groups. Commands execute with the same network access, environment variables, and IAM permissions as the services:

```bash
runecs run "echo \"HELLO WORLD\"" -w --service mycanvas-ecs-staging-cluster/web
```

**RunECS supports both AWS Fargate and EC2 capacity providers.** The tool automatically selects the appropriate launch type based on service configuration. When you use the `-w` flag, RunECS waits for task completion and streams full output to the terminal. This approach works well for interactive debugging and migration scripts.

### Scale ECS Services

Adjust the desired count of tasks for an ECS service instantly:

```bash
runecs scale 5 --service mycanvas-ecs-staging-cluster/web
```

This command directly modifies the service's desired count using `UpdateService`, providing immediate scaling without creating task sets or managing deployment configurations.

### Restart ECS Services

Restart ECS services gracefully without downtime, or force immediate task termination when required:

```bash
runecs restart --service mycanvas-ecs-staging-cluster/addrp
```

By default, RunECS performs a rolling restart. Tasks get replaced one by one to maintain service availability. For immediate task termination (such as clearing stuck processes or forcing configuration reloads), use the `--kill` flag to terminate all tasks at once. The service then spawns replacements according to the desired count.

## FAQ

#### How does this differ from AWS CLI?

While AWS CLI offers comprehensive control over ECS clusters, its extensive feature set can introduce significant complexity for everyday tasks. RunECS streamlines common ECS operations by focusing on the workflows developers use most frequently, providing an intuitive and efficient command-line experience without sacrificing functionality.
