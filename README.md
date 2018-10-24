# 🍅 tomato - behavioral testing tools
![CircleCI](https://circleci.com/gh/tomatool/tomato/tree/master.svg?style=shield)
[![Go Report Card](https://goreportcard.com/badge/github.com/tomatool/tomato)](https://goreportcard.com/report/github.com/tomatool/tomato)
[![GoDoc](https://godoc.org/github.com/tomatool/tomato?status.svg)](https://godoc.org/github.com/tomatool/tomato)
[![codecov.io](https://codecov.io/github/tomatool/tomato/branch/master/graph/badge.svg)](https://codecov.io/github/tomatool/tomato)

Integration testing tools, built on top of (https://github.com/DATA-DOG/godog). To simplify adding Integration Test to your application without writing any code.

tomato uses YAML config file to specify [resources](#resources)

[Documentation](https://alileza.github.io/tomato/)

# Getting started with docker-compose.yml
Create `docker-compose.yml` file:
```yml
version: '3'
services:
  tomato:
    image: alileza/tomato:latest
    environment:
      APP_BASE_URL: http://my-application:9000
      PSQL_DATASOURCE: "postgres://tomato:potato@postgres:5432/tomato?sslmode=disable"
    volumes:
      - ./tomato.yml:/config.yml
      - ./features/:/features/

  my-application:
    build: .
    expose:
      - "9000"

  postgres:
    image: postgres:9.5
    expose:
      - "5432"
    environment:
            POSTGRES_USER: tomato
            POSTGRES_PASSWORD: potato
            POSTGRES_DB: tomato
    volumes:
      - ./sqldump/:/docker-entrypoint-initdb.d/

```

Create `tomato.yml` file:
```yml
---

resources:
    - name: psql
      type: db/sql
      params:
        driver: postgres
        datasource: {{ .PSQL_DATASOURCE }}

    - name: app-client
      type: http/client
      params:
        base_url: {{ .APP_BASE_URL }}

```

And you can start writing your first feature, try with `check-status.feature` put it inside `features` folder:
```gherkin
Feature: check status endpoint

  Scenario: When everything is fine
    Given "app-client" send request to "GET /status"
    Then "app-client" response code should be 200

```

Executing your test
```sh
docker-compose up --exit-code-from tomato
```

And that's it !!! 🙌

[List of available resources](http://alileza.github.io/tomato/resources)

You can find some of examples [here](https://github.com/tomatool/tomato/tree/0.1.0/examples/features)
