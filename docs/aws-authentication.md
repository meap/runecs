# AWS Authentication Configuration

RunECS supports multiple methods for AWS authentication:

## Using Environment Variables

RunECS also supports credential configuration through environment variables as documented in the [AWS CLI Environment Variables guide](https://docs.aws.amazon.com/cli/v1/userguide/cli-configure-envvars.html). This approach integrates seamlessly with tools like [direnv](https://direnv.net/). You can maintain distinct account configurations, regions, and other settings on a per-directory or per-project basis.

### Using direnv

direnv automatically loads environment variables from `.envrc` files when entering directories, enabling project-specific AWS configurations.

#### Basic AWS Credentials

```bash
# .envrc
export AWS_ACCESS_KEY_ID="AKIA..."
export AWS_SECRET_ACCESS_KEY="..."
export AWS_DEFAULT_REGION="us-east-1"
```

#### Multi-Account Setup

For different environments in separate directories:

```bash
# development/.envrc
export AWS_ACCESS_KEY_ID="AKIA...DEV"
export AWS_SECRET_ACCESS_KEY="...dev"
export AWS_DEFAULT_REGION="us-west-2"

# staging/.envrc  
export AWS_ACCESS_KEY_ID="AKIA...STAGING"
export AWS_SECRET_ACCESS_KEY="...staging"
export AWS_DEFAULT_REGION="us-east-1"

# production/.envrc
export AWS_ACCESS_KEY_ID="AKIA...PROD"
export AWS_SECRET_ACCESS_KEY="...prod"
export AWS_DEFAULT_REGION="eu-west-1"
```

#### AWS SSO with Session Tokens

```bash
# .envrc
export AWS_ACCESS_KEY_ID="ASIA..."
export AWS_SECRET_ACCESS_KEY="..."
export AWS_SESSION_TOKEN="IQo..."
export AWS_DEFAULT_REGION="us-east-1"
```

#### Profile-Based Configuration

```bash
# .envrc
export AWS_PROFILE="my-project-dev"
export AWS_DEFAULT_REGION="us-west-2"
```

Once configured, RunECS commands automatically use the active environment:

```bash
# Automatically uses credentials from .envrc
runecs list
runecs deploy --service myapp/web -i latest
```

## Using AWS Credential Profiles

Specify credential profiles using the global `--profile` parameter to easily switch between different accounts or roles. Configure profiles in the credentials file as documented in the [AWS CLI Configuration Files guide](https://docs.aws.amazon.com/cli/v1/userguide/cli-configure-files.html).

```bash
# Use a specific AWS profile
runecs list --profile production

# Deploy using a different profile
runecs deploy --service myapp/web -i latest --profile staging

# If profile doesn't have region defined, specify it with AWS_REGION
AWS_REGION=eu-central-1 runecs list --profile staging
```
