---
layout: default
title: Step Reference
nav_order: 4
---

# Step Reference

This document lists all available Gherkin steps organized by resource type.

> **Note:** This documentation is auto-generated from the source code. Run `tomato docs` to regenerate.


---

## HTTP Client

Steps for making HTTP requests and validating responses


### Request Setup

| Step | Description |
|------|-------------|
| `"api" header "Content-Type" is "application/json"` | Set a header |
| `"api" headers are:` | Set multiple headers from table |
| `"api" query param "page" is "1"` | Set a query parameter |
| `"api" body is:` | Set raw request body (docstring) |
| `"api" json body is:` | Set JSON body + Content-Type header |
| `"api" form body is:` | Set form-encoded body from table |


### Request Execution

| Step | Description |
|------|-------------|
| `"api" sends "GET" to "/users"` | Send HTTP request |
| `"api" sends "POST" to "/users" with body:` | Send with raw body |
| `"api" sends "POST" to "/users" with json:` | Send with JSON body |


### Response Status

| Step | Description |
|------|-------------|
| `"api" response status is "200"` | Assert exact status code |
| `"api" response status is success` | Assert status class (2xx, 3xx, 4xx, 5xx) |


### Response Headers

| Step | Description |
|------|-------------|
| `"api" response header "Content-Type" is "application/json"` | Assert exact header value |
| `"api" response header "Content-Type" contains "json"` | Assert header contains substring |
| `"api" response header "X-Request-Id" exists` | Assert header exists |


### Response Body

| Step | Description |
|------|-------------|
| `"api" response body is:` | Assert exact body match |
| `"api" response body contains "success"` | Assert body contains substring |
| `"api" response body does not contain "error"` | Assert body doesn't contain substring |
| `"api" response body is empty` | Assert empty body |


### Response JSON

| Step | Description |
|------|-------------|
| `"api" response json "data.id" is "123"` | Assert JSON path value |
| `"api" response json "data.id" exists` | Assert JSON path exists |
| `"api" response json "data.deleted" does not exist` | Assert JSON path doesn't exist |
| `"api" response json matches:` | Assert exact JSON structure with matchers: @string, @number, @boolean, @array, @object, @any, @null, @notnull, @empty, @notempty, @regex:pattern, @contains:text, @startswith:text, @endswith:text, @gt:n, @gte:n, @lt:n, @lte:n, @len:n |
| `"api" response json contains:` | Assert JSON contains specified fields (ignores extra fields). Supports same matchers as 'matches' |


### Response Timing

| Step | Description |
|------|-------------|
| `"api" response time is less than "500ms"` | Assert response time |



---

## Redis

Steps for interacting with Redis key-value store


### String Operations

| Step | Description |
|------|-------------|
| `"cache" key "user:1" is "John"` | Set a string value |
| `"cache" key "session:abc" is "data" with TTL "1h"` | Set a string value with expiration |
| `"cache" key "user:1" is:` | Set a JSON/multiline value |
| `"cache" key "user:1" is deleted` | Delete a key |
| `"cache" key "counter" is incremented` | Increment integer value by 1 |
| `"cache" key "counter" is incremented by "5"` | Increment integer value by N |
| `"cache" key "counter" is decremented` | Decrement integer value by 1 |


### String Assertions

| Step | Description |
|------|-------------|
| `"cache" key "user:1" exists` | Assert key exists |
| `"cache" key "user:1" does not exist` | Assert key doesn't exist |
| `"cache" key "user:1" has value "John"` | Assert exact value |
| `"cache" key "user:1" contains "John"` | Assert value contains substring |
| `"cache" key "session:abc" has TTL greater than "3600" seconds` | Assert TTL greater than N seconds |


### Hash Operations

| Step | Description |
|------|-------------|
| `"cache" hash "user:1" has fields:` | Set hash fields from table |
| `"cache" hash "user:1" field "name" is "John"` | Assert hash field value |
| `"cache" hash "user:1" contains:` | Assert hash contains fields |


### List Operations

| Step | Description |
|------|-------------|
| `"cache" list "queue" has "item1"` | Push value to list |
| `"cache" list "queue" has values:` | Push multiple values to list |
| `"cache" list "queue" has "3" items` | Assert list length |
| `"cache" list "queue" contains "item1"` | Assert list contains value |


### Set Operations

| Step | Description |
|------|-------------|
| `"cache" set "tags" has "tag1"` | Add member to set |
| `"cache" set "tags" has members:` | Add multiple members to set |
| `"cache" set "tags" contains "tag1"` | Assert set contains member |
| `"cache" set "tags" has "3" members` | Assert set size |


### Database

| Step | Description |
|------|-------------|
| `"cache" has "5" keys` | Assert total key count |
| `"cache" is empty` | Assert database is empty |



---

## PostgreSQL

Steps for interacting with PostgreSQL databases


### Data Setup

| Step | Description |
|------|-------------|
| `"db" table "users" has values:` | Insert rows from table |
| `"db" executes:` | Execute raw SQL |
| `"db" executes file "fixtures/seed.sql"` | Execute SQL from file |


### Assertions

| Step | Description |
|------|-------------|
| `"db" table "users" contains:` | Assert table contains rows |
| `"db" table "users" is empty` | Assert table is empty |
| `"db" table "users" has "5" rows` | Assert row count |



---

## Kafka

Steps for interacting with Apache Kafka message broker


### Topic Management

| Step | Description |
|------|-------------|
| `"{resource}" topic "events" exists` | Asserts a Kafka topic exists |
| `"{resource}" creates topic "events"` | Creates a Kafka topic with 1 partition |
| `"{resource}" creates topic "events" with "3" partitions` | Creates a Kafka topic with specified partitions |


### Publishing

| Step | Description |
|------|-------------|
| `"{resource}" publishes to "events":
  """
  Hello World
  """` | Publishes a message to a topic |
| `"{resource}" publishes to "events" with key "user-123":
  """
  Hello World
  """` | Publishes a message with a key to a topic |
| `"{resource}" publishes json to "events":
  """
  {"type": "user_created"}
  """` | Publishes a JSON message to a topic |
| `"{resource}" publishes json to "events" with key "user-123":
  """
  {"type": "user_created"}
  """` | Publishes a JSON message with a key |
| `"{resource}" publishes messages to "events":
  | key      | value           |
  | user-1   | {"id": 1}     |` | Publishes multiple messages from a table |


### Consuming

| Step | Description |
|------|-------------|
| `"{resource}" consumes from "events"` | Starts consuming messages from a topic |
| `"{resource}" receives from "events" within "5s"` | Waits for a message from a topic within timeout |
| `"{resource}" receives from "events" within "5s":
  """
  Hello World
  """` | Asserts a specific message is received within timeout |
| `"{resource}" receives from "events" with key "user-123" within "5s"` | Asserts a message with specific key is received |


### Assertions

| Step | Description |
|------|-------------|
| `"{resource}" topic "events" has "3" messages` | Asserts topic has exactly N messages consumed |
| `"{resource}" topic "events" is empty` | Asserts no messages have been consumed from topic |
| `"{resource}" last message contains:
  """
  user_created
  """` | Asserts the last consumed message contains content |
| `"{resource}" last message has key "user-123"` | Asserts the last consumed message has specific key |
| `"{resource}" last message has header "content-type" with value "application/json"` | Asserts the last message has a header with value |
| `"{resource}" receives messages from "events" in order:
  | key    | value  |
  | key1   | msg1   |` | Asserts messages are received in specified order |



---

## WebSocket Client

Steps for connecting to WebSocket servers


### Connection

| Step | Description |
|------|-------------|
| `"ws" connects` | Connect to WebSocket endpoint |
| `"ws" connects with headers:` | Connect with custom headers |
| `"ws" disconnects` | Disconnect from WebSocket |
| `"ws" is connected` | Assert connected |
| `"ws" is disconnected` | Assert disconnected |


### Sending

| Step | Description |
|------|-------------|
| `"ws" sends:` | Send text message (docstring) |
| `"ws" sends "ping"` | Send short text message |
| `"ws" sends json:` | Send JSON message |


### Receiving

| Step | Description |
|------|-------------|
| `"ws" receives within "5s":` | Assert message received within timeout |
| `"ws" receives within "5s" containing "success"` | Assert message containing substring received |
| `"ws" receives json within "5s" matching:` | Assert JSON message matching structure |
| `"ws" receives "3" messages within "10s"` | Assert N messages received within timeout |
| `"ws" does not receive within "2s"` | Assert no message received |


### Assertions

| Step | Description |
|------|-------------|
| `"ws" last message is:` | Assert last message matches exactly |
| `"ws" last message contains "success"` | Assert last message contains substring |
| `"ws" last message is json matching:` | Assert last message is JSON matching structure |
| `"ws" received "5" messages` | Assert total message count |



---

## Shell

Steps for executing shell commands and scripts


### Setup

| Step | Description |
|------|-------------|
| `"shell" env "API_KEY" is "secret"` | Set environment variable |
| `"shell" workdir is "/tmp/test"` | Set working directory |


### Execution

| Step | Description |
|------|-------------|
| `"shell" runs:` | Run command (docstring) |
| `"shell" runs "ls -la"` | Run inline command |
| `"shell" runs script "scripts/setup.sh"` | Run script file |
| `"shell" runs with timeout "60s":` | Run with custom timeout |


### Exit Code

| Step | Description |
|------|-------------|
| `"shell" exit code is "0"` | Assert exit code |
| `"shell" succeeds` | Assert exit code 0 |
| `"shell" fails` | Assert non-zero exit code |


### Output

| Step | Description |
|------|-------------|
| `"shell" stdout contains "success"` | Assert stdout contains substring |
| `"shell" stdout does not contain "error"` | Assert stdout doesn't contain |
| `"shell" stdout is:` | Assert exact stdout |
| `"shell" stdout is empty` | Assert stdout empty |
| `"shell" stderr contains "warning"` | Assert stderr contains substring |
| `"shell" stderr is empty` | Assert stderr empty |


### Files

| Step | Description |
|------|-------------|
| `"shell" file "output.txt" exists` | Assert file exists |
| `"shell" file "temp.txt" does not exist` | Assert file doesn't exist |
| `"shell" file "config.json" contains "database"` | Assert file contains substring |



