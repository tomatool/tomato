# 🍅 tomato - behavioral testing tools
![CircleCI](https://circleci.com/gh/alileza/tomato/tree/master.svg?style=shield)
[![Go Report Card](https://goreportcard.com/badge/github.com/alileza/tomato)](https://goreportcard.com/report/github.com/alileza/tomato)
[![GoDoc](https://godoc.org/github.com/alileza/tomato?status.svg)](https://godoc.org/github.com/alileza/tomato)
[![codecov.io](https://codecov.io/github/alileza/tomato/branch/master/graph/badge.svg)](https://codecov.io/github/alileza/tomato)
 
Behavior Driven Development tools, built on top of (https://github.com/DATA-DOG/godog). To simplify adding BDD to your application without writing any code.

tomato uses yaml config file to specify [resources](#resources)

[Documentation](https://alileza.github.io/tomato/)

# Getting started

Install tomato by untar latest stable [release](https://github.com/alileza/tomato/releases/latest), or get from latest master by
```
go get -u github.com/alileza/tomato/cmd/tomato
```

Prepare your `tomato.yml` with your resources. For example :

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

[List of available resources](http://alileza.github.io/tomato/resources)

Once you're ready with all your resources you needed, you're good to start writing your features. You can find some of examples [here](https://github.com/alileza/tomato/tree/0.1.0/examples/features)

`example.feature`
```gherkin
Feature: behavior test example

  Scenario: Delete customer on DELETE http request
    Given listen message from "my-awesome-queue" target "customers:deleted"
    Given set "my-awesome-postgres-db" table "customers" list of content
        | name    | country |
        | cembri  | id      |
    Given "httpcli" send request to "DELETE /api/v1/customers" with body
        """
            {
                "name":"cembri"
            }
        """
    Then "my-awesome-postgres-db" table "customers" should look like
        | customer_id | name    |
    Then message from "my-awesome-queue" target "customers:deleted" count should be 1
    Then message from "my-awesome-queue" target "customers:deleted" should look like
        """
            {
                "country":"id",
                "name":"cembri",
                "reason":"http-post-request"
            }
        """
```

## Executing your test

You need to make sure that all your resources and application you're going to test is ready before executing tomato.

```sh
tomato -c tomato.yml -f example.feature
```

Have fun! 🍅
