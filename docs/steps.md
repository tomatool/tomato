---
layout: default
title: Step Reference
nav_order: 4
---

# Tomato Step Reference

This document lists all available Gherkin steps in Tomato.

{: .note }
This documentation is auto-generated from the source code. Run `tomato docs` to regenerate.


## HTTP

Steps for making HTTP requests and validating responses


### Sets a header for the next HTTP request

**Pattern:** `^I set "<resource>" header "([^"]*)" to "([^"]*)"$`

**Example:**
```gherkin
I set "api" header "Content-Type" to "application/json"
```


### Sets multiple headers for the next HTTP request using a table

**Pattern:** `^I set "<resource>" headers:$`

**Example:**
```gherkin
I set "api" headers:
  | header       | value            |
  | Content-Type | application/json |
```


### Sets a query parameter for the next HTTP request

**Pattern:** `^I set "<resource>" query param "([^"]*)" to "([^"]*)"$`

**Example:**
```gherkin
I set "api" query param "page" to "1"
```


### Sets the raw request body for the next HTTP request

**Pattern:** `^I set "<resource>" request body:$`

**Example:**
```gherkin
I set "api" request body:
  """
  raw body content
  """
```


### Sets a JSON request body and automatically sets Content-Type header

**Pattern:** `^I set "<resource>" JSON body:$`

**Example:**
```gherkin
I set "api" JSON body:
  """
  {"name": "test"}
  """
```


### Sets form-encoded body from a table and sets Content-Type header

**Pattern:** `^I set "<resource>" form body:$`

**Example:**
```gherkin
I set "api" form body:
  | field | value |
  | name  | test  |
```


### Sends an HTTP request with the specified method to the given path

**Pattern:** `^I send "([^"]*)" request to "<resource>" "([^"]*)"$`

**Example:**
```gherkin
I send "GET" request to "api" "/api/users"
```


### Sends an HTTP request with a raw body

**Pattern:** `^I send "([^"]*)" request to "<resource>" "([^"]*)" with body:$`

**Example:**
```gherkin
I send "POST" request to "api" "/api/users" with body:
  """
  raw body
  """
```


### Sends an HTTP request with a JSON body

**Pattern:** `^I send "([^"]*)" request to "<resource>" "([^"]*)" with JSON:$`

**Example:**
```gherkin
I send "POST" request to "api" "/api/users" with JSON:
  """
  {"name": "John"}
  """
```


### Asserts the response has the exact HTTP status code

**Pattern:** `^"<resource>" response status should be "(\d+)"$`

**Example:**
```gherkin
"api" response status should be "200"
```


### Asserts the response status is in the given class (2xx, 3xx, 4xx, 5xx)

**Pattern:** `^"<resource>" response status should be (success|redirect|client error|server error)$`

**Example:**
```gherkin
"api" response status should be success
```


### Asserts a response header has the exact value

**Pattern:** `^"<resource>" response header "([^"]*)" should be "([^"]*)"$`

**Example:**
```gherkin
"api" response header "Content-Type" should be "application/json"
```


### Asserts a response header contains a substring

**Pattern:** `^"<resource>" response header "([^"]*)" should contain "([^"]*)"$`

**Example:**
```gherkin
"api" response header "Content-Type" should contain "json"
```


### Asserts a response header exists

**Pattern:** `^"<resource>" response header "([^"]*)" should exist$`

**Example:**
```gherkin
"api" response header "X-Request-Id" should exist
```


### Asserts the response body matches exactly

**Pattern:** `^"<resource>" response body should be:$`

**Example:**
```gherkin
"api" response body should be:
  """
  expected body
  """
```


### Asserts the response body contains a substring

**Pattern:** `^"<resource>" response body should contain "([^"]*)"$`

**Example:**
```gherkin
"api" response body should contain "success"
```


### Asserts the response body does not contain a substring

**Pattern:** `^"<resource>" response body should not contain "([^"]*)"$`

**Example:**
```gherkin
"api" response body should not contain "error"
```


### Asserts the response body is empty

**Pattern:** `^"<resource>" response body should be empty$`

**Example:**
```gherkin
"api" response body should be empty
```


### Asserts a JSON path in the response has the expected value

**Pattern:** `^"<resource>" response JSON "([^"]*)" should be "([^"]*)"$`

**Example:**
```gherkin
"api" response JSON "data.id" should be "123"
```


### Asserts a JSON path exists in the response

**Pattern:** `^"<resource>" response JSON "([^"]*)" should exist$`

**Example:**
```gherkin
"api" response JSON "data.id" should exist
```


### Asserts a JSON path does not exist in the response

**Pattern:** `^"<resource>" response JSON "([^"]*)" should not exist$`

**Example:**
```gherkin
"api" response JSON "data.deleted" should not exist
```


### Asserts the response JSON matches the expected structure. Use @string, @number, @boolean, @array, @object, @any, @null, @notnull as type matchers

**Pattern:** `^"<resource>" response JSON should match:$`

**Example:**
```gherkin
"api" response JSON should match:
  """
  {"id": "@number", "name": "@string"}
  """
```


### Asserts the response was received within the given duration

**Pattern:** `^"<resource>" response time should be less than "([^"]*)"$`

**Example:**
```gherkin
"api" response time should be less than "500ms"
```



## Redis

Steps for interacting with Redis key-value store


### Sets a string value for a key

**Pattern:** `^I set "<resource>" key "([^"]*)" with value "([^"]*)"$`

**Example:**
```gherkin
I set "api" key "user:1" with value "John"
```


### Sets a string value with expiration time

**Pattern:** `^I set "<resource>" key "([^"]*)" with value "([^"]*)" and TTL "([^"]*)"$`

**Example:**
```gherkin
I set "api" key "session:abc" with value "data" and TTL "1h"
```


### Sets a JSON value for a key

**Pattern:** `^I set "<resource>" key "([^"]*)" with JSON:$`

**Example:**
```gherkin
I set "api" key "user:1" with JSON:
  """
  {"name": "John"}
  """
```


### Deletes a key

**Pattern:** `^I delete "<resource>" key "([^"]*)"$`

**Example:**
```gherkin
I delete "api" key "user:1"
```


### Asserts that a key exists

**Pattern:** `^"<resource>" key "([^"]*)" should exist$`

**Example:**
```gherkin
"api" key "user:1" should exist
```


### Asserts that a key does not exist

**Pattern:** `^"<resource>" key "([^"]*)" should not exist$`

**Example:**
```gherkin
"api" key "user:1" should not exist
```


### Asserts a key has the exact value

**Pattern:** `^"<resource>" key "([^"]*)" should have value "([^"]*)"$`

**Example:**
```gherkin
"api" key "user:1" should have value "John"
```


### Asserts a key's value contains a substring

**Pattern:** `^"<resource>" key "([^"]*)" should contain "([^"]*)"$`

**Example:**
```gherkin
"api" key "user:1" should contain "John"
```


### Asserts a key has TTL greater than specified seconds

**Pattern:** `^"<resource>" key "([^"]*)" should have TTL greater than "(\d+)" seconds$`

**Example:**
```gherkin
"api" key "session:abc" should have TTL greater than "3600" seconds
```


### Asserts the database has exactly N keys

**Pattern:** `^"<resource>" should have "(\d+)" keys$`

**Example:**
```gherkin
"api" should have "5" keys
```


### Asserts the database has no keys

**Pattern:** `^"<resource>" should be empty$`

**Example:**
```gherkin
"api" should be empty
```


### Sets multiple fields in a hash

**Pattern:** `^I set "<resource>" hash "([^"]*)" with fields:$`

**Example:**
```gherkin
I set "api" hash "user:1" with fields:
  | field | value |
  | name  | John  |
  | age   | 30    |
```


### Asserts a hash field has the expected value

**Pattern:** `^"<resource>" hash "([^"]*)" field "([^"]*)" should be "([^"]*)"$`

**Example:**
```gherkin
"api" hash "user:1" field "name" should be "John"
```


### Asserts a hash contains the specified field-value pairs

**Pattern:** `^"<resource>" hash "([^"]*)" should contain:$`

**Example:**
```gherkin
"api" hash "user:1" should contain:
  | field | value |
  | name  | John  |
```


### Pushes a value to the end of a list

**Pattern:** `^I push "([^"]*)" to "<resource>" list "([^"]*)"$`

**Example:**
```gherkin
I push "item1" to "api" list "queue"
```


### Pushes multiple values to a list

**Pattern:** `^I push values to "<resource>" list "([^"]*)":$`

**Example:**
```gherkin
I push values to "api" list "queue":
  | item1 |
  | item2 |
```


### Asserts a list has exactly N items

**Pattern:** `^"<resource>" list "([^"]*)" should have "(\d+)" items$`

**Example:**
```gherkin
"api" list "queue" should have "3" items
```


### Asserts a list contains a value

**Pattern:** `^"<resource>" list "([^"]*)" should contain "([^"]*)"$`

**Example:**
```gherkin
"api" list "queue" should contain "item1"
```


### Adds a member to a set

**Pattern:** `^I add "([^"]*)" to "<resource>" set "([^"]*)"$`

**Example:**
```gherkin
I add "tag1" to "api" set "tags"
```


### Adds multiple members to a set

**Pattern:** `^I add members to "<resource>" set "([^"]*)":$`

**Example:**
```gherkin
I add members to "api" set "tags":
  | tag1 |
  | tag2 |
```


### Asserts a set contains a member

**Pattern:** `^"<resource>" set "([^"]*)" should contain "([^"]*)"$`

**Example:**
```gherkin
"api" set "tags" should contain "tag1"
```


### Asserts a set has exactly N members

**Pattern:** `^"<resource>" set "([^"]*)" should have "(\d+)" members$`

**Example:**
```gherkin
"api" set "tags" should have "3" members
```


### Increments a key's integer value by 1

**Pattern:** `^I increment "<resource>" key "([^"]*)"$`

**Example:**
```gherkin
I increment "api" key "counter"
```


### Increments a key's integer value by N

**Pattern:** `^I increment "<resource>" key "([^"]*)" by "(\d+)"$`

**Example:**
```gherkin
I increment "api" key "counter" by "5"
```


### Decrements a key's integer value by 1

**Pattern:** `^I decrement "<resource>" key "([^"]*)"$`

**Example:**
```gherkin
I decrement "api" key "counter"
```



## Postgres

Steps for interacting with PostgreSQL databases


### Inserts rows into a table from a data table

**Pattern:** `^I set "<resource>" table "([^"]*)" with values:$`

**Example:**
```gherkin
I set "api" table "users" with values:
  | id | name  | email           |
  | 1  | John  | john@test.com   |
```


### Asserts a table contains the expected rows

**Pattern:** `^"<resource>" table "([^"]*)" should contain:$`

**Example:**
```gherkin
"api" table "users" should contain:
  | id | name  |
  | 1  | John  |
```


### Asserts a table has no rows

**Pattern:** `^"<resource>" table "([^"]*)" should be empty$`

**Example:**
```gherkin
"api" table "users" should be empty
```


### Asserts a table has exactly N rows

**Pattern:** `^"<resource>" table "([^"]*)" should have "(\d+)" rows$`

**Example:**
```gherkin
"api" table "users" should have "5" rows
```


### Executes raw SQL query

**Pattern:** `^I execute SQL on "<resource>":$`

**Example:**
```gherkin
I execute SQL on "api":
  """
  UPDATE users SET active = true WHERE id = 1
  """
```


### Executes SQL from a file

**Pattern:** `^I execute SQL file "([^"]*)" on "<resource>"$`

**Example:**
```gherkin
I execute SQL file "fixtures/seed.sql" on "api"
```



## Kafka

Steps for interacting with Apache Kafka message broker


### Asserts a Kafka topic exists

**Pattern:** `^kafka topic "([^"]*)" exists on "<resource>"$`

**Example:**
```gherkin
kafka topic "events" exists on "api"
```


### Creates a Kafka topic with 1 partition

**Pattern:** `^I create kafka topic "([^"]*)" on "<resource>"$`

**Example:**
```gherkin
I create kafka topic "events" on "api"
```


### Creates a Kafka topic with specified partitions

**Pattern:** `^I create kafka topic "([^"]*)" on "<resource>" with "(\d+)" partitions$`

**Example:**
```gherkin
I create kafka topic "events" on "api" with "3" partitions
```


### Publishes a message to a topic

**Pattern:** `^I publish message to "<resource>" topic "([^"]*)":$`

**Example:**
```gherkin
I publish message to "api" topic "events":
  """
  Hello World
  """
```


### Publishes a message with a key to a topic

**Pattern:** `^I publish message to "<resource>" topic "([^"]*)" with key "([^"]*)":$`

**Example:**
```gherkin
I publish message to "api" topic "events" with key "user-123":
  """
  Hello World
  """
```


### Publishes a JSON message to a topic

**Pattern:** `^I publish JSON to "<resource>" topic "([^"]*)":$`

**Example:**
```gherkin
I publish JSON to "api" topic "events":
  """
  {"type": "user_created"}
  """
```


### Publishes a JSON message with a key

**Pattern:** `^I publish JSON to "<resource>" topic "([^"]*)" with key "([^"]*)":$`

**Example:**
```gherkin
I publish JSON to "api" topic "events" with key "user-123":
  """
  {"type": "user_created"}
  """
```


### Publishes multiple messages from a table

**Pattern:** `^I publish messages to "<resource>" topic "([^"]*)":$`

**Example:**
```gherkin
I publish messages to "api" topic "events":
  | key      | value           |
  | user-1   | {"id": 1}     |
```


### Starts consuming messages from a topic

**Pattern:** `^I start consuming from "<resource>" topic "([^"]*)"$`

**Example:**
```gherkin
I start consuming from "api" topic "events"
```


### Waits for a message from a topic within timeout

**Pattern:** `^I consume message from "<resource>" topic "([^"]*)" within "([^"]*)"$`

**Example:**
```gherkin
I consume message from "api" topic "events" within "5s"
```


### Asserts a specific message is received within timeout

**Pattern:** `^I should receive message from "<resource>" topic "([^"]*)" within "([^"]*)":$`

**Example:**
```gherkin
I should receive message from "api" topic "events" within "5s":
  """
  Hello World
  """
```


### Asserts a message with specific key is received

**Pattern:** `^I should receive message from "<resource>" topic "([^"]*)" with key "([^"]*)" within "([^"]*)"$`

**Example:**
```gherkin
I should receive message from "api" topic "events" with key "user-123" within "5s"
```


### Asserts topic has exactly N messages consumed

**Pattern:** `^"<resource>" topic "([^"]*)" should have "(\d+)" messages$`

**Example:**
```gherkin
"api" topic "events" should have "3" messages
```


### Asserts no messages have been consumed from topic

**Pattern:** `^"<resource>" topic "([^"]*)" should be empty$`

**Example:**
```gherkin
"api" topic "events" should be empty
```


### Asserts the last consumed message contains content

**Pattern:** `^the last message from "<resource>" should contain:$`

**Example:**
```gherkin
the last message from "api" should contain:
  """
  user_created
  """
```


### Asserts the last consumed message has specific key

**Pattern:** `^the last message from "<resource>" should have key "([^"]*)"$`

**Example:**
```gherkin
the last message from "api" should have key "user-123"
```


### Asserts the last message has a header with value

**Pattern:** `^the last message from "<resource>" should have header "([^"]*)" with value "([^"]*)"$`

**Example:**
```gherkin
the last message from "api" should have header "content-type" with value "application/json"
```


### Asserts messages are received in specified order

**Pattern:** `^I should receive messages from "<resource>" topic "([^"]*)" in order:$`

**Example:**
```gherkin
I should receive messages from "api" topic "events" in order:
  | key    | value  |
  | key1   | msg1   |
```



## WebSocket

Steps for interacting with WebSocket connections


### Connects to the WebSocket endpoint

**Pattern:** `^I connect to websocket "<resource>"$`

**Example:**
```gherkin
I connect to websocket "api"
```


### Connects with custom headers

**Pattern:** `^I connect to websocket "<resource>" with headers:$`

**Example:**
```gherkin
I connect to websocket "api" with headers:
  | header        | value       |
  | Authorization | Bearer xyz  |
```


### Disconnects from the WebSocket

**Pattern:** `^I disconnect from websocket "<resource>"$`

**Example:**
```gherkin
I disconnect from websocket "api"
```


### Asserts the WebSocket is connected

**Pattern:** `^websocket "<resource>" should be connected$`

**Example:**
```gherkin
websocket "api" should be connected
```


### Asserts the WebSocket is disconnected

**Pattern:** `^websocket "<resource>" should be disconnected$`

**Example:**
```gherkin
websocket "api" should be disconnected
```


### Sends a text message

**Pattern:** `^I send message to websocket "<resource>":$`

**Example:**
```gherkin
I send message to websocket "api":
  """
  Hello Server
  """
```


### Sends a short text message

**Pattern:** `^I send text "([^"]*)" to websocket "<resource>"$`

**Example:**
```gherkin
I send text "ping" to websocket "api"
```


### Sends a JSON message

**Pattern:** `^I send JSON to websocket "<resource>":$`

**Example:**
```gherkin
I send JSON to websocket "api":
  """
  {"action": "subscribe"}
  """
```


### Asserts a specific message is received within timeout

**Pattern:** `^I should receive message from websocket "<resource>" within "([^"]*)":$`

**Example:**
```gherkin
I should receive message from websocket "api" within "5s":
  """
  Hello Client
  """
```


### Asserts a message containing substring is received

**Pattern:** `^I should receive message from websocket "<resource>" within "([^"]*)" containing "([^"]*)"$`

**Example:**
```gherkin
I should receive message from websocket "api" within "5s" containing "success"
```


### Asserts a JSON message matching structure is received

**Pattern:** `^I should receive JSON from websocket "<resource>" within "([^"]*)" matching:$`

**Example:**
```gherkin
I should receive JSON from websocket "api" within "5s" matching:
  """
  {"status": "ok"}
  """
```


### Asserts N messages are received within timeout

**Pattern:** `^I should receive "(\d+)" messages from websocket "<resource>" within "([^"]*)"$`

**Example:**
```gherkin
I should receive "3" messages from websocket "api" within "10s"
```


### Asserts no message is received within timeout

**Pattern:** `^I should not receive message from websocket "<resource>" within "([^"]*)"$`

**Example:**
```gherkin
I should not receive message from websocket "api" within "2s"
```


### Asserts the last message matches exactly

**Pattern:** `^the last message from websocket "<resource>" should be:$`

**Example:**
```gherkin
the last message from websocket "api" should be:
  """
  pong
  """
```


### Asserts the last message contains substring

**Pattern:** `^the last message from websocket "<resource>" should contain "([^"]*)"$`

**Example:**
```gherkin
the last message from websocket "api" should contain "success"
```


### Asserts the last message is JSON matching structure

**Pattern:** `^the last message from websocket "<resource>" should be JSON matching:$`

**Example:**
```gherkin
the last message from websocket "api" should be JSON matching:
  """
  {"type": "response"}
  """
```


### Asserts total messages received count

**Pattern:** `^websocket "<resource>" should have received "(\d+)" messages$`

**Example:**
```gherkin
websocket "api" should have received "5" messages
```



## Shell

Steps for executing shell commands and scripts


### Sets an environment variable for commands

**Pattern:** `^I set "<resource>" environment variable "([^"]*)" to "([^"]*)"$`

**Example:**
```gherkin
I set "api" environment variable "API_KEY" to "secret"
```


### Sets the working directory for commands

**Pattern:** `^I set "<resource>" working directory to "([^"]*)"$`

**Example:**
```gherkin
I set "api" working directory to "/tmp/test"
```


### Runs a shell command

**Pattern:** `^I run command on "<resource>":$`

**Example:**
```gherkin
I run command on "api":
  """
  echo "Hello World"
  """
```


### Runs a short inline command

**Pattern:** `^I run "([^"]*)" on "<resource>"$`

**Example:**
```gherkin
I run "ls -la" on "api"
```


### Runs a script file

**Pattern:** `^I run script "([^"]*)" on "<resource>"$`

**Example:**
```gherkin
I run script "scripts/setup.sh" on "api"
```


### Runs a command with custom timeout

**Pattern:** `^I run command on "<resource>" with timeout "([^"]*)":$`

**Example:**
```gherkin
I run command on "api" with timeout "60s":
  """
  ./long-running-task
  """
```


### Asserts the command exit code

**Pattern:** `^"<resource>" exit code should be "(\d+)"$`

**Example:**
```gherkin
"api" exit code should be "0"
```


### Asserts the command exited with code 0

**Pattern:** `^"<resource>" should succeed$`

**Example:**
```gherkin
"api" should succeed
```


### Asserts the command exited with non-zero code

**Pattern:** `^"<resource>" should fail$`

**Example:**
```gherkin
"api" should fail
```


### Asserts stdout contains substring

**Pattern:** `^"<resource>" stdout should contain "([^"]*)"$`

**Example:**
```gherkin
"api" stdout should contain "success"
```


### Asserts stdout does not contain substring

**Pattern:** `^"<resource>" stdout should not contain "([^"]*)"$`

**Example:**
```gherkin
"api" stdout should not contain "error"
```


### Asserts stdout matches exactly

**Pattern:** `^"<resource>" stdout should be:$`

**Example:**
```gherkin
"api" stdout should be:
  """
  Hello World
  """
```


### Asserts stdout is empty

**Pattern:** `^"<resource>" stdout should be empty$`

**Example:**
```gherkin
"api" stdout should be empty
```


### Asserts stderr contains substring

**Pattern:** `^"<resource>" stderr should contain "([^"]*)"$`

**Example:**
```gherkin
"api" stderr should contain "warning"
```


### Asserts stderr is empty

**Pattern:** `^"<resource>" stderr should be empty$`

**Example:**
```gherkin
"api" stderr should be empty
```


### Asserts a file exists

**Pattern:** `^"<resource>" file "([^"]*)" should exist$`

**Example:**
```gherkin
"api" file "output.txt" should exist
```


### Asserts a file does not exist

**Pattern:** `^"<resource>" file "([^"]*)" should not exist$`

**Example:**
```gherkin
"api" file "temp.txt" should not exist
```


### Asserts a file contains substring

**Pattern:** `^"<resource>" file "([^"]*)" should contain "([^"]*)"$`

**Example:**
```gherkin
"api" file "config.json" should contain "database"
```



