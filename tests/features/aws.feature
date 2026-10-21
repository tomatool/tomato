Feature: AWS identity
  The aws resource gives the app IRSA: a role and a web identity token, which the app's SDK
  exchanges at the STS the resource serves. tomato's own AWS credentials stay out of the
  app's environment.

  Scenario: The app is given the IRSA environment
    When "app-client" sends "GET" to "/env?name=AWS_ROLE_ARN"
    Then "app-client" response status is "200"
    And "app-client" response json "value" is "arn:aws:iam::000000000000:role/tomato-tests"
    When "app-client" sends "GET" to "/env?name=AWS_ENDPOINT_URL_STS"
    Then "app-client" response json "value" is "http://127.0.0.1:9996"
    When "app-client" sends "GET" to "/env?name=AWS_REGION"
    Then "app-client" response json "value" is "eu-central-1"
    When "app-client" sends "GET" to "/env?name=AWS_ACCESS_KEY_ID"
    Then "app-client" response json "set" is "false"

  Scenario: A web identity is exchanged for the role's session
    Given "sts-client" form body is:
      | field            | value                                        |
      | Action           | AssumeRoleWithWebIdentity                    |
      | Version          | 2011-06-15                                   |
      | RoleArn          | arn:aws:iam::000000000000:role/tomato-tests  |
      | RoleSessionName  | tests                                        |
      | WebIdentityToken | a-token                                      |
    When "sts-client" sends "POST" to "/"
    Then "sts-client" response status is "200"
    And "sts-client" response body contains "<AccessKeyId>TOMATO"
    And "sts-client" response body contains "assumed-role/tomato-tests/tests"
    And "aws" role "arn:aws:iam::000000000000:role/tomato-tests" was assumed
    And "aws" role "arn:aws:iam::000000000000:role/never-used" was not assumed

  Scenario: An unsupported STS action is refused like STS refuses it
    Given "sts-client" form body is:
      | field  | value                      |
      | Action | DecodeAuthorizationMessage |
    When "sts-client" sends "POST" to "/"
    Then "sts-client" response status is "400"
    And "sts-client" response body contains "<Code>InvalidAction</Code>"
