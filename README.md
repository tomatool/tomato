# 🍅 tomato - behavioral testing tool kit
![CircleCI](https://circleci.com/gh/alileza/tomato/tree/master.svg?style=shield)
[![Go Report Card](https://goreportcard.com/badge/github.com/alileza/tomato)](https://goreportcard.com/report/github.com/alileza/tomato)
[![GoDoc](https://godoc.org/github.com/alileza/tomato?status.svg)](https://godoc.org/github.com/alileza/tomato)
[![codecov.io](https://codecov.io/github/alileza/tomato/branch/master/graph/badge.svg)](https://codecov.io/github/alileza/tomato)

Tomato is a language agnostic testing tool kit that simplifies the acceptance testing workflow of your application and its dependencies.

Using [godog](https://github.com/DATA-DOG/godog) and [Gherkin](https://docs.cucumber.io/gherkin/), tomato makes behavioral tests easier to understand, maintain, and write for developers, QA, and product owners.

- [Documentation](https://alileza.github.io/tomato/)
- [Examples](https://github.com/alileza/tomato/tree/0.1.0/examples/features)

## Features
- Cucumber [Gherkin](https://docs.cucumber.io/gherkin/) feature syntax
- Support for MySQL, MariaDB, and PostgreSQL
- Support for messaging queues (RabbitMQ, NSQ)
- Support for mocking HTTP API responses
- Additional resources [resources](https://alileza.github.io/tomato/resources)

## Getting Started

### Set up your tomato configuration
Tomato integrates your app and its test dependencies using a simple configuration file `tomato.yml`.

Create a `tomato.yml` file with your application's required test [resources](https://alileza.github.io/tomato/resources):
```yml
---

resources:
    - name: psql
      type: db/sql
      params:
        driver: postgres
        datasource: {{ .PSQL_DATASOURCE }}

    - name: your-application-client
      type: http/client
      params:
        base_url: {{ .APP_BASE_URL }}
```

### Write your first feature test
Write your own [Gherkin](https://docs.cucumber.io/gherkin/) feature (or customize the check-status.feature example below) and place it inside ./features/check-status.feature:

```gherkin
Feature: Check my application's status endpoint

  Scenario: My application is running and active
    Given "your-application-client" send request to "GET /status"
    Then "your-application-client" response code should be 200
```

### Run tomato
#### Using docker-compose

Now that you have your resources configured, you can use docker-compose to run tomato in any Docker environment (great for CI and other build pipelines).

Create a `docker-compose.yml` file, or add tomato and your test dependencies to an existing one:
```yml
version: '3'
services:
  tomato:
    image: quay.io/alileza/tomato:latest
    environment:
      APP_BASE_URL: http://my-application:9000
      PSQL_DATASOURCE: "postgres://user:password@postgres:5432/test-database?sslmode=disable"
    volumes:
      - ./tomato.yml:/config.yml # location of your tomato.yml
      - ./features/:/features/   # location of all of your features

  my-application:
    build: .
    expose:
      - "9000"

  postgres:
    image: postgres:9.5
    expose:
      - "5432"
    environment:
            POSTGRES_USER: user
            POSTGRES_PASSWORD: password
            POSTGRES_DB: test-database
    volumes:
      - ./sqldump/:/docker-entrypoint-initdb.d/ # schema or migrations sql
```

Execute your tests
```sh
docker-compose up --abort-on-container-exit
```

#### Using the binary

Install tomato by grabbing the latest stable [release](https://github.com/alileza/tomato/releases/latest) and placing it in your path, or by using go get
```
go get -u github.com/alileza/tomato/cmd/tomato
```

Now run tomato:
```sh
tomato -c tomato.yml -f check-status.feature
```

---

## Resources

Resource are the objects that are going to be used for steps in the scenario. They are listed under the resources key in the pipeline configuration.

### List of available resources

* [Database SQL](https://github.com/alileza/tomato/blob/master/docs/resources/database-sql.md) - to manage (insert/check) database state changes
  * [MySQL](https://github.com/alileza/tomato/blob/master/docs/resources/database-sql.md)
  * [PostgreSQL](https://github.com/alileza/tomato/blob/master/docs/resources/database-sql.md)
* [HTTP Client](https://github.com/alileza/tomato/blob/master/docs/resources/http-client.md) - to send HTTP Request
* [HTTP Server](https://github.com/alileza/tomato/blob/master/docs/resources/http-server.md) - to create fake external service dependency that serve HTTP API
* [Message Queue](https://github.com/alileza/tomato/blob/master/docs/resources/message-queue.md) - to manage (publish/consume/check) message queue
  * [RabbitMQ](https://github.com/alileza/tomato/blob/master/docs/resources/message-queue.md)
  * [NSQ](https://github.com/alileza/tomato/blob/master/docs/resources/message-queue.md)
* [Shell](https://github.com/alileza/tomato/blob/master/docs/resources/shell.md) -  to execute unix shell command-line interface
