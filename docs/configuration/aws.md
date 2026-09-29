# AWS configuration

The `aws` resource gives the application under test the AWS identity it gets on
AWS through IRSA (IAM roles for service accounts): a role and a web identity
token, which the application's AWS SDK exchanges at STS for session
credentials. tomato serves that STS itself, so the application gets its
credentials the way it does in production, and a test fails when that path
breaks.

```yaml
resources:
  aws:
    type: aws
    options:
      region: eu-central-1
      role_arn: arn:aws:iam::000000000000:role/my-service
      ambient_identity: true
```

## Options

| Option | Default | Description |
|--------|---------|-------------|
| `region` | `us-east-1` | Region the application is told it runs in |
| `role_arn` | `arn:aws:iam::000000000000:role/tomato` | The IRSA role |
| `session_name` | `tomato` | Role session name |
| `ambient_identity` | `false` | Also leave a second, long-term identity where the SDK's credential chain looks after the role (see below) |
| `port` | random | Port of the STS endpoint |

## What the application gets

The resource starts before the application and adds to its environment:

| Variable | Value |
|----------|-------|
| `AWS_ROLE_ARN`, `AWS_ROLE_SESSION_NAME` | the role |
| `AWS_WEB_IDENTITY_TOKEN_FILE` | a token file tomato writes |
| `AWS_ENDPOINT_URL_STS` | tomato's STS |
| `AWS_REGION`, `AWS_DEFAULT_REGION` | `region` |
| `AWS_SHARED_CREDENTIALS_FILE`, `AWS_CONFIG_FILE` | files tomato writes, so your own `~/.aws` is never read |
| `AWS_EC2_METADATA_DISABLED` | `true` |

It also removes `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`,
`AWS_SESSION_TOKEN`, `AWS_PROFILE` and the container-credential variables from
what the application inherits from your shell or CI, so credentials you happen
to have never stand in for the role. The application's own `env` still wins.
This applies to an application run as a local process (`app.command`).

The STS answers `AssumeRoleWithWebIdentity`, `AssumeRole` and
`GetCallerIdentity`. It does not verify tokens or signatures. The credentials it
issues are stable per role, so a [kafka preset](kafka.md#msk-iam-authentication)
with `auth: aws_msk_iam` knows to let exactly those in.

## The ambient identity

On EKS, when an SDK cannot get the role's credentials, its credential chain
keeps looking and can end up with another identity, such as the node's role.
The service then authenticates as the wrong principal and is refused.
`ambient_identity: true` puts such an identity in the shared credentials file,
so that fallback happens in the test too, and ends the same way: an
`AWS_MSK_IAM` listener that only admits the role's sessions answers
`Access denied`.

## Steps

The journal of assumed roles covers the whole run and is not reset between
scenarios, because applications assume their role when they start. See
[AWS steps](../resources/aws.md).

```gherkin
Then "aws" role "arn:aws:iam::000000000000:role/my-service" was assumed
```
