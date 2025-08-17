# AWS Authentication Configuration

RunECS supports multiple methods for AWS authentication:

## Using Environment Variables

RunECS also supports credential configuration through environment variables as documented in the [AWS CLI Environment Variables guide](https://docs.aws.amazon.com/cli/v1/userguide/cli-configure-envvars.html). This approach integrates seamlessly with tools like [direnv](https://direnv.net/). You can maintain distinct account configurations, regions, and other settings on a per-directory or per-project basis.

## Using AWS Credential Profiles

Specify credential profiles using the global `--profile` parameter to easily switch between different accounts or roles. Configure profiles in the credentials file as documented in the [AWS CLI Configuration Files guide](https://docs.aws.amazon.com/cli/v1/userguide/cli-configure-files.html).

```bash
# Use a specific AWS profile
runecs list --profile production

# Deploy using a different profile
runecs deploy --service myapp/web -i latest --profile staging
```
