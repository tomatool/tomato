# 🍅 tomato - behavioral testing tool kit
![CircleCI](https://circleci.com/gh/tomatool/tomato/tree/master.svg?style=shield)
[![Go Report Card](https://goreportcard.com/badge/github.com/tomatool/tomato)](https://goreportcard.com/report/github.com/tomatool/tomato)
[![GoDoc](https://godoc.org/github.com/tomatool/tomato?status.svg)](https://godoc.org/github.com/tomatool/tomato)
[![codecov.io](https://codecov.io/github/tomatool/tomato/branch/master/graph/badge.svg)](https://codecov.io/github/tomatool/tomato)

Tomato is a language agnostic testing tool kit that simplifies the acceptance testing workflow of your application and its dependencies.

Using [godog](https://github.com/DATA-DOG/godog) and [Gherkin](https://docs.cucumber.io/gherkin/), tomato makes behavioral tests easier to understand, maintain, and write for developers, QA, and product owners.

- [Background](https://medium.com/@alileza/functional-testing-using-af78a868a1f1)
- [Documentation](https://github.com/tomatool/tomato/tree/master/docs)
- [Examples](https://github.com/tomatool/tomato/tree/0.1.0/examples/features)

## Features
- Cucumber [Gherkin](https://docs.cucumber.io/gherkin/) feature syntax
- Support for MySQL, MariaDB, and PostgreSQL
- Support for messaging queues (RabbitMQ, NSQ)
- Support for mocking HTTP API responses
- Additional resources [resources](https://tomatool.github.io/tomato/resources)

## Getting Started

### Set up your tomato configuration
Tomato integrates your app and its test dependencies using a simple configuration file `tomato.yml`.

Create a `tomato.yml` file with your application's required test [resources](https://github.com/tomatool/tomato/blob/master/docs/resources.md):
```yml
---

# Randomize scenario execution order
randomize: true

# Stops on the first failure
stop_on_failure: false

# All feature file paths
features_path:
    - ./features
    - check-status.feature

# List of resources for application dependencies
resources:
    - name: psql
      type: postgres
      options:
        datasource: {{ .PSQL_DATASOURCE }}

    - name: your-application-client
      type: httpclient
      options:
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
    image: quay.io/tomatool/tomato:latest
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

#### Install latest stable version

```
curl https://raw.githubusercontent.com/tomatool/tomato/master/install.sh | sh
```

#### Install latest master

Install tomato by grabbing the latest stable [release](https://github.com/tomatool/tomato/releases/latest) and placing it in your path, or by using go get
```
go get -u github.com/tomatool/tomato
```

#### Executing tomato

Now run tomato:
```sh
tomato tomato.yml
```

---

## Resources

Resource are the objects that are going to be used for steps in the scenario. They are listed under the resources key in the pipeline configuration.

### List of available resources

* [Database SQL](https://github.com/tomatool/tomato/blob/master/docs/resources.md#database-sql) - to manage (insert/check) database state changes
  * [MySQL](https://github.com/tomatool/tomato/blob/master/docs/resources.md#database-sql)
  * [PostgreSQL](https://github.com/tomatool/tomato/blob/master/docs/resources.md#database-sql)
* [HTTP Client](https://github.com/tomatool/tomato/blob/master/docs/resources.md#http-client) - to send HTTP Request
* [HTTP Server](https://github.com/tomatool/tomato/blob/master/docs/resources.md#http-server) - to mock external HTTP Server
  * [Wiremock](https://github.com/tomatool/tomato/blob/master/docs/resources.md#http-server)
* [Message Queue](https://github.com/tomatool/tomato/blob/master/docs/resources.md#queue) - to manage (publish/consume/check) message queue
  * [RabbitMQ](https://github.com/tomatool/tomato/blob/master/docs/resources.md#queue)
  * [NSQ](https://github.com/tomatool/tomato/blob/master/docs/resources.md#queue)
* [Shell](https://github.com/tomatool/tomato/blob/master/docs/resources.md#shell) -  to execute unix shell command-line interface
* [Cache](https://github.com/tomatool/tomato/blob/master/docs/resources.md#cache)
  * [Redis](https://github.com/tomatool/tomato/blob/master/docs/resources.md#cache)
