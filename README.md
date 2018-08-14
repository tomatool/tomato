# 🍅 tomato - behavioral testing tool suite
![CircleCI](https://circleci.com/gh/alileza/tomato/tree/master.svg?style=shield)
[![Go Report Card](https://goreportcard.com/badge/github.com/alileza/tomato)](https://goreportcard.com/report/github.com/alileza/tomato)
[![GoDoc](https://godoc.org/github.com/alileza/tomato?status.svg)](https://godoc.org/github.com/alileza/tomato)
[![codecov.io](https://codecov.io/github/alileza/tomato/branch/master/graph/badge.svg)](https://codecov.io/github/alileza/tomato)

Built on top of [godog](https://github.com/DATA-DOG/godog), tomato is a language agnostic tool suite that simplifies adding behavioral tests to your application without writing any additional code.

# Overview

Tomato uses a yaml file to specify [resources](#resources). Resources are then used in the [gherkin](https://docs.cucumber.io/gherkin/reference/) documents that both you and your team write to test the behavior of your application against real dependencies.

[Documentation](https://alileza.github.io/tomato/)

# Installation

Install the latest stable release from [here](https://github.com/alileza/tomato/releases/latest), or from master using go get
```
go get -u github.com/alileza/tomato/cmd/tomato
```

# Quickstart

Prepare your `tomato.yml` in your project by configuring your required resources ([supported resources](http://alileza.github.io/tomato/resources)). For example:

```yaml
---

resources:
  - name: my-awesome-postgres-db
    type: db/sql
    params:
      driver: postgres
      datasource: postgres://user:pass@localhost:5432/customers?sslmode=disable
  - name: my-awesome-queue
    type: queue
    params:
      driver: rabbitmq
      datasource: amqp://guest:guest@localhost:5672
```

Once your tomato.yml is set up with all the resources your application requires, you can begin writing features. Features are pre-defined behavior tests that are run against your resources to determine if the application performs as expected. You can find some examples [here](https://github.com/alileza/tomato/tree/0.1.0/examples/features)

`example.feature`
```gherkin
Feature: Customer removal via an HTTP API request

  # Set the stage for the test
  Background:
    Given the table "customers" in "my-awesome-postgres-db" with content
      | name    | country |
      | cembri  | id      |

  Scenario: Remove cembri when a rabbitmq delete message is received
    Given a message received from "my-awesome-queue" target "customers:deleted"
    When the "httpcli" sends a DELETE request to "/api/v1/customers" with body
        """
            {
                "name":"cembri"
            }
        """
    Then the table "customers" in "my-awesome-postgres-db" should not have a "name" cembri
    And the number of messages in "my-awesome-queue" target "customers:deleted" should be 1
    And the message from "my-awesome-queue" target "customers:deleted" should look like
        """
            {
                "country":"id",
                "name":"cembri",
                "reason":"http-post-request"
            }
        """
```

## Executing your test

Before beginning, ensure all resources your application is going to test against are ready before executing tomato.

```sh
tomato -c tomato.yml -f example.feature
```

Have fun! 🍅

## Integrating with your continous integration

[Documentation CI Integration](https://alileza.github.io/tomato/ci-integration)
