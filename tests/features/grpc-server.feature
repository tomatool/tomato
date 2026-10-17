@grpc-server
Feature: gRPC Server
  The app's user directory is an HTTP API in front of a gRPC service,
  tomato.test.v1.UserService. tomato stands in for that service: "users"
  serves it from its descriptor set and answers with what each scenario stubs.
  The app reaches it at USERS_ADDR, which app.env sets to {{.users.address}}.

  Scenario: The app shows what the service returns
    Given "users" stub "tomato.test.v1.UserService/GetUser" returns json:
      """
      {"id": "42", "name": "Alice", "role": "ROLE_ADMIN", "tags": ["beta"], "createdAt": "2026-10-17T10:00:00Z"}
      """
    When "api" sends "GET" to "/directory/users/42"
    Then "api" response status is "200"
    And "api" response json contains:
      """
      {"id": "42", "name": "Alice", "role": "ROLE_ADMIN", "tags": ["beta"], "createdAt": "2026-10-17T10:00:00Z"}
      """
    And "users" received "tomato.test.v1.UserService/GetUser"
    And "users" last "tomato.test.v1.UserService/GetUser" request json contains:
      """
      {"id": "42"}
      """

  Scenario: The app forwards the caller's token as metadata
    Given "users" stub "tomato.test.v1.UserService/GetUser" with metadata "authorization" "Bearer alice-token" returns json:
      """
      {"id": "42", "name": "Alice"}
      """
    And "api" header "Authorization" is "Bearer alice-token"
    And "api" header "X-Request-Id" is "req-7"
    When "api" sends "GET" to "/directory/users/42"
    Then "api" response status is "200"
    And "users" received "tomato.test.v1.UserService/GetUser" with metadata "authorization" "Bearer alice-token"
    And "users" last "tomato.test.v1.UserService/GetUser" request metadata "authorization" is "Bearer alice-token"
    And "users" last "tomato.test.v1.UserService/GetUser" request metadata "x-request-id" saved as "{{request_id}}"
    And "users" last "tomato.test.v1.UserService/GetUser" request json "id" saved as "{{user_id}}"
    When "api" sends "GET" to "/directory/users/{{user_id}}"
    Then "api" response json "name" is "Alice"
    And "users" received "tomato.test.v1.UserService/GetUser" "2" times
    And "users" received "tomato.test.v1.UserService/GetUser" with metadata "x-request-id" "{{request_id}}"

  Scenario: A call no stub matches is answered UNIMPLEMENTED, naming the method
    Given "users" stub "tomato.test.v1.UserService/GetUser" with metadata "authorization" "Bearer alice-token" returns json:
      """
      {"id": "42", "name": "Alice"}
      """
    When "api" sends "GET" to "/directory/users/42"
    Then "api" response status is "502"
    And "api" response json "code" is "Unimplemented"
    And "api" response body contains "no stub for tomato.test.v1.UserService/GetUser matches this call"

  Scenario: A failure for one request, a user for the others
    Given "users" stub "tomato.test.v1.UserService/GetUser" returns json:
      """
      {"id": "1", "name": "Bob"}
      """
    And "users" stub "tomato.test.v1.UserService/GetUser" with request json "id" "404" returns status "NOT_FOUND" with message "no user 404"
    And "users" stub "tomato.test.v1.UserService/GetUser" with request json "id" "@startswith:secret-" returns status "PERMISSION_DENIED"
    When "api" sends "GET" to "/directory/users/404"
    Then "api" response status is "404"
    And "api" response json "error" is "no user 404"
    When "api" sends "GET" to "/directory/users/secret-7"
    Then "api" response status is "403"
    When "api" sends "GET" to "/directory/users/1"
    Then "api" response status is "200"
    And "api" response json "name" is "Bob"

  Scenario: A dependency that is starting, then slow, then fine
    # The app gives each call one second; the second answer comes too late.
    Given "users" stub "tomato.test.v1.UserService/GetUser" returns in sequence:
      | status      | message    | json                          | delay |
      | UNAVAILABLE | warming up |                               |       |
      | OK          |            | {"id": "42", "name": "Slow"}  | 3s    |
      | OK          |            | {"id": "42", "name": "Alice"} |       |
    When "api" sends "GET" to "/directory/users/42"
    Then "api" response status is "503"
    And "api" response json "error" is "warming up"
    When "api" sends "GET" to "/directory/users/42"
    Then "api" response status is "504"
    When "api" sends "GET" to "/directory/users/42"
    Then "api" response status is "200"
    And "api" response json "name" is "Alice"
    When "api" sends "GET" to "/directory/users/42"
    Then "api" response json "name" is "Alice"
    And "users" received "tomato.test.v1.UserService/GetUser" "4" times

  Scenario: The service is down, then back
    Given "users" stub "tomato.test.v1.UserService/GetUser" returns json:
      """
      {"id": "42", "name": "Alice"}
      """
    And "users" refuses connections
    When "api" sends "GET" to "/directory/users/42"
    Then "api" response status is "503"
    And "users" did not receive "tomato.test.v1.UserService/GetUser"
    When "users" accepts connections
    And "api" sends "GET" to "/directory/users/42"
    Then "api" response status is "200"
    And "api" response json "name" is "Alice"

  Scenario: A scenario that leaves the service down
    Given "users" refuses connections

  Scenario: The next scenario finds it accepting again, with no stubs
    When "api" sends "GET" to "/directory/users/42"
    Then "api" response status is "502"
    And "api" response json "code" is "Unimplemented"
    And "users" received "1" calls

  Scenario: What the app asked for
    Given "users" stub "tomato.test.v1.UserService/ListUsers" returns json:
      """
      {"users": [{"id": "1", "name": "Alice"}, {"id": "2", "name": "Bob"}], "nextPageToken": "page-2"}
      """
    When "api" sends "GET" to "/directory/users?page_size=50"
    Then "api" response status is "200"
    And "api" response json "users[1].name" is "Bob"
    And "api" response json "nextPageToken" is "page-2"
    And "users" last "tomato.test.v1.UserService/ListUsers" request json contains:
      """
      {"pageSize": "@gte:50", "pageToken": ""}
      """
    And "users" received "1" calls
    And "users" did not receive "tomato.test.v1.UserService/GetUser"

  Scenario: A server stream, message by message
    Given "users" stub "tomato.test.v1.UserService/WatchUsers" with request json "team" "payments" streams:
      | json                         | delay |
      | {"id": "1", "name": "Alice"} |       |
      | {"id": "2", "name": "Bob"}   | 50ms  |
    When "api" sends "GET" to "/directory/watch?team=payments"
    Then "api" response status is "200"
    And "api" response json "status" is "OK"
    And "api" response json "users[0].name" is "Alice"
    And "api" response json "users[1].name" is "Bob"
    And "users" last "tomato.test.v1.UserService/WatchUsers" request json contains:
      """
      {"team": "payments"}
      """

  Scenario: A server stream that breaks off
    Given "users" stub "tomato.test.v1.UserService/WatchUsers" streams:
      | json                         | status      | message          |
      | {"id": "1", "name": "Alice"} |             |                  |
      |                              | UNAVAILABLE | connection reset |
    When "api" sends "GET" to "/directory/watch?team=payments"
    Then "api" response json "users[0].name" is "Alice"
    And "api" response json "users[1]" does not exist
    And "api" response json "status" is "Unavailable"
    And "api" response json "message" is "connection reset"

  Scenario: The stub serves reflection, so a grpc client can call it too
    Given "users" stub "tomato.test.v1.UserService/GetUser" returns json:
      """
      {"id": "7", "name": "Grace"}
      """
    And "users" address is stored in "USERS_ADDRESS"
    And "users-grpc" metadata "x-upstream" is "{{USERS_ADDRESS}}"
    When "users-grpc" calls "tomato.test.v1.UserService/GetUser" with:
      """
      {"id": "7"}
      """
    Then "users-grpc" call succeeds
    And "users-grpc" response json "name" is "Grace"
    And "users-grpc" response json "role" is "ROLE_UNSPECIFIED"
    And "users-grpc" exposes service "tomato.test.v1.UserService"
    And "users-grpc" service "tomato.test.v1.UserService" has method "WatchUsers"
    And "users" last "tomato.test.v1.UserService/GetUser" request metadata "x-upstream" is "localhost:9995"

  Scenario: A method nobody stubbed
    When "users-grpc" calls "tomato.test.v1.UserService/ListUsers" with:
      """
      {}
      """
    Then "users-grpc" response status is "UNIMPLEMENTED"
    And "users-grpc" response error contains "has no stub for tomato.test.v1.UserService/ListUsers"
