# Getting Started 

## Set up your tomato configuration
Tomato integrates your app and its test dependencies using a simple configuration file `tomato.yml`. 

Create a `tomato.yml` file with your application's required test [resources](#resources):
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

## Write your first feature test
Write your own [Gherkin](https://docs.cucumber.io/gherkin/) feature (or customize the check-status.feature example below) and place it inside ./features/check-status.feature: 

```gherkin
Feature: Check my application's status endpoint

  Scenario: My application is running and active 
    Given "your-application-client" sends a "GET" HTTP request to "/status"
    Then "your-application-client" response code should be 200
```

## Run tomato 
### Using docker-compose 

Now that you have your resources configured, you can use docker-compose to run tomato in any Docker environment (great for CI and other build pipelines).

Create a `docker-compose.yml` file, or add tomato and your test dependencies to an existing one:
```yml
version: '3'
services:
  tomato:
    image: alileza/tomato:latest
    environment:
      APP_BASE_URL: http://your-application:9000
      PSQL_DATASOURCE: "postgres://user:passport@postgres:5432/test-database?sslmode=disable"
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
docker-compose up --exit-code-from tomato
```

### Using the binary

Install tomato by grabbing the latest stable [release](https://github.com/alileza/tomato/releases/latest) and placing it in your path, or by using go get 
```
go get -u github.com/alileza/tomato/cmd/tomato
```

Now run tomato:
```sh
tomato -c tomato.yml -f check-status.feature
```