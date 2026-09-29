Feature: AWS identity
  The aws resource gives the app IRSA: a role and a web identity token, which the app's SDK
  exchanges at the STS the resource serves. tomato's own AWS credentials stay out of the
  app's environment.

  Scenario: The app is given the IRSA environment
    When "api" sends "GET" to "/env?name=AWS_ROLE_ARN"
    Then "api" response status is "200"
    And "api" response json "value" is "arn:aws:iam::000000000000:role/tomato-tests"
    When "api" sends "GET" to "/env?name=AWS_ENDPOINT_URL_STS"
    Then "api" response json "value" is "http://127.0.0.1:9996"
    When "api" sends "GET" to "/env?name=AWS_REGION"
    Then "api" response json "value" is "eu-central-1"
    When "api" sends "GET" to "/env?name=AWS_ACCESS_KEY_ID"
    Then "api" response json "set" is "false"

  Scenario: A web identity is exchanged for the role's session
    Given "sts" form body is:
      | field            | value                                        |
      | Action           | AssumeRoleWithWebIdentity                    |
      | Version          | 2011-06-15                                   |
      | RoleArn          | arn:aws:iam::000000000000:role/tomato-tests  |
      | RoleSessionName  | tests                                        |
      | WebIdentityToken | a-token                                      |
    When "sts" sends "POST" to "/"
    Then "sts" response status is "200"
    And "sts" response body contains "<AccessKeyId>TOMATO"
    And "sts" response body contains "assumed-role/tomato-tests/tests"
    And "aws" role "arn:aws:iam::000000000000:role/tomato-tests" was assumed
    And "aws" role "arn:aws:iam::000000000000:role/never-used" was not assumed

  Scenario: An unsupported STS action is refused like STS refuses it
    Given "sts" form body is:
      | field  | value                      |
      | Action | DecodeAuthorizationMessage |
    When "sts" sends "POST" to "/"
    Then "sts" response status is "400"
    And "sts" response body contains "<Code>InvalidAction</Code>"
