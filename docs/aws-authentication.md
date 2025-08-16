# AWS Authentication Configuration

RunECS supports multiple methods for AWS authentication:

## Using Environment Variables

AWS credential configuration through environment variables is also supported as documented in the [AWS CLI Environment Variables guide](https://docs.aws.amazon.com/cli/v1/userguide/cli-configure-envvars.html). This approach integrates seamlessly with tools like [direnv](https://direnv.net/), enabling distinct AWS account configurations, regions, and other settings to be maintained on a per-directory or per-project basis.

## Using AWS Credential Profiles

AWS credential profiles can be specified using the global `--profile` parameter, allowing you to easily switch between different AWS accounts or roles. Profiles are configured in the AWS credentials file as documented in the [AWS CLI Configuration Files guide](https://docs.aws.amazon.com/cli/v1/userguide/cli-configure-files.html).

```bash
# Use a specific AWS profile
runecs list --profile production

# Deploy using a different profile
runecs deploy --service myapp/web -i latest --profile staging
```

The credentials file is typically located at `~/.aws/credentials`.