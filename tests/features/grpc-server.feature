@grpc-server
Feature: gRPC Server
  The app's user directory is an HTTP API in front of a gRPC service,
  tomato.test.v1.UserService. tomato stands in for that service: "users-stub"
  serves it from its descriptor set and answers with what each scenario stubs.
  The app reaches it at USERS_ADDR, which app.env sets to {{.users-stub.address}}.

  Scenario: The app shows what the service returns
    Given "users-stub" stub "tomato.test.v1.UserService/GetUser" returns json:
      """
      {"id": "42", "name": "Alice", "role": "ROLE_ADMIN", "tags": ["beta"], "createdAt": "2026-10-17T10:00:00Z"}
      """
    When "app-client" sends "GET" to "/directory/users/42"
    Then "app-client" response status is "200"
    And "app-client" response json contains:
      """
      {"id": "42", "name": "Alice", "role": "ROLE_ADMIN", "tags": ["beta"], "createdAt": "2026-10-17T10:00:00Z"}
      """
    And "users-stub" received "tomato.test.v1.UserService/GetUser"
    And "users-stub" last "tomato.test.v1.UserService/GetUser" request json contains:
      """
      {"id": "42"}
      """

  Scenario: The app forwards the caller's token as metadata
    Given "users-stub" stub "tomato.test.v1.UserService/GetUser" with metadata "authorization" "Bearer alice-token" returns json:
      """
      {"id": "42", "name": "Alice"}
      """
    And "app-client" header "Authorization" is "Bearer alice-token"
    And "app-client" header "X-Request-Id" is "req-7"
    When "app-client" sends "GET" to "/directory/users/42"
    Then "app-client" response status is "200"
    And "users-stub" received "tomato.test.v1.UserService/GetUser" with metadata "authorization" "Bearer alice-token"
    And "users-stub" last "tomato.test.v1.UserService/GetUser" request metadata "authorization" is "Bearer alice-token"
    And "users-stub" last "tomato.test.v1.UserService/GetUser" request metadata "x-request-id" saved as "{{request_id}}"
    And "users-stub" last "tomato.test.v1.UserService/GetUser" request json "id" saved as "{{user_id}}"
    When "app-client" sends "GET" to "/directory/users/{{user_id}}"
    Then "app-client" response json "name" is "Alice"
    And "users-stub" received "tomato.test.v1.UserService/GetUser" "2" times
    And "users-stub" received "tomato.test.v1.UserService/GetUser" with metadata "x-request-id" "{{request_id}}"

  Scenario: A call no stub matches is answered UNIMPLEMENTED, naming the method
    Given "users-stub" stub "tomato.test.v1.UserService/GetUser" with metadata "authorization" "Bearer alice-token" returns json:
      """
      {"id": "42", "name": "Alice"}
      """
    When "app-client" sends "GET" to "/directory/users/42"
    Then "app-client" response status is "502"
    And "app-client" response json "code" is "Unimplemented"
    And "app-client" response body contains "no stub for tomato.test.v1.UserService/GetUser matches this call"

  Scenario: A failure for one request, a user for the others
    Given "users-stub" stub "tomato.test.v1.UserService/GetUser" returns json:
      """
      {"id": "1", "name": "Bob"}
      """
    And "users-stub" stub "tomato.test.v1.UserService/GetUser" with request json "id" "404" returns status "NOT_FOUND" with message "no user 404"
    And "users-stub" stub "tomato.test.v1.UserService/GetUser" with request json "id" "@startswith:secret-" returns status "PERMISSION_DENIED"
    When "app-client" sends "GET" to "/directory/users/404"
    Then "app-client" response status is "404"
    And "app-client" response json "error" is "no user 404"
    When "app-client" sends "GET" to "/directory/users/secret-7"
    Then "app-client" response status is "403"
    When "app-client" sends "GET" to "/directory/users/1"
    Then "app-client" response status is "200"
    And "app-client" response json "name" is "Bob"

  Scenario: A dependency that is starting, then slow, then fine
    # The app gives each call one second; the second answer comes too late.
    Given "users-stub" stub "tomato.test.v1.UserService/GetUser" returns in sequence:
      | status      | message    | json                          | delay |
      | UNAVAILABLE | warming up |                               |       |
      | OK          |            | {"id": "42", "name": "Slow"}  | 3s    |
      | OK          |            | {"id": "42", "name": "Alice"} |       |
    When "app-client" sends "GET" to "/directory/users/42"
    Then "app-client" response status is "503"
    And "app-client" response json "error" is "warming up"
    When "app-client" sends "GET" to "/directory/users/42"
    Then "app-client" response status is "504"
    When "app-client" sends "GET" to "/directory/users/42"
    Then "app-client" response status is "200"
    And "app-client" response json "name" is "Alice"
    When "app-client" sends "GET" to "/directory/users/42"
    Then "app-client" response json "name" is "Alice"
    And "users-stub" received "tomato.test.v1.UserService/GetUser" "4" times

  Scenario: The service is down, then back
    Given "users-stub" stub "tomato.test.v1.UserService/GetUser" returns json:
      """
      {"id": "42", "name": "Alice"}
      """
    And "users-stub" refuses connections
    When "app-client" sends "GET" to "/directory/users/42"
    Then "app-client" response status is "503"
    And "users-stub" did not receive "tomato.test.v1.UserService/GetUser"
    When "users-stub" accepts connections
    And "app-client" sends "GET" to "/directory/users/42"
    Then "app-client" response status is "200"
    And "app-client" response json "name" is "Alice"

  Scenario: A scenario that leaves the service down
    Given "users-stub" refuses connections

  Scenario: The next scenario finds it accepting again, with no stubs
    When "app-client" sends "GET" to "/directory/users/42"
    Then "app-client" response status is "502"
    And "app-client" response json "code" is "Unimplemented"
    And "users-stub" received "1" calls

  Scenario: What the app asked for
    Given "users-stub" stub "tomato.test.v1.UserService/ListUsers" returns json:
      """
      {"users": [{"id": "1", "name": "Alice"}, {"id": "2", "name": "Bob"}], "nextPageToken": "page-2"}
      """
    When "app-client" sends "GET" to "/directory/users?page_size=50"
    Then "app-client" response status is "200"
    And "app-client" response json "users[1].name" is "Bob"
    And "app-client" response json "nextPageToken" is "page-2"
    And "users-stub" last "tomato.test.v1.UserService/ListUsers" request json contains:
      """
      {"pageSize": "@gte:50", "pageToken": ""}
      """
    And "users-stub" received "1" calls
    And "users-stub" did not receive "tomato.test.v1.UserService/GetUser"

  Scenario: A server stream, message by message
    Given "users-stub" stub "tomato.test.v1.UserService/WatchUsers" with request json "team" "payments" streams:
      | json                         | delay |
      | {"id": "1", "name": "Alice"} |       |
      | {"id": "2", "name": "Bob"}   | 50ms  |
    When "app-client" sends "GET" to "/directory/watch?team=payments"
    Then "app-client" response status is "200"
    And "app-client" response json "status" is "OK"
    And "app-client" response json "users[0].name" is "Alice"
    And "app-client" response json "users[1].name" is "Bob"
    And "users-stub" last "tomato.test.v1.UserService/WatchUsers" request json contains:
      """
      {"team": "payments"}
      """

  Scenario: A server stream that breaks off
    Given "users-stub" stub "tomato.test.v1.UserService/WatchUsers" streams:
      | json                         | status      | message          |
      | {"id": "1", "name": "Alice"} |             |                  |
      |                              | UNAVAILABLE | connection reset |
    When "app-client" sends "GET" to "/directory/watch?team=payments"
    Then "app-client" response json "users[0].name" is "Alice"
    And "app-client" response json "users[1]" does not exist
    And "app-client" response json "status" is "Unavailable"
    And "app-client" response json "message" is "connection reset"

  Scenario: The stub serves reflection, so a grpc client can call it too
    Given "users-stub" stub "tomato.test.v1.UserService/GetUser" returns json:
      """
      {"id": "7", "name": "Grace"}
      """
    And "users-stub" address is stored in "USERS_ADDRESS"
    And "users-client" metadata "x-upstream" is "{{USERS_ADDRESS}}"
    When "users-client" calls "tomato.test.v1.UserService/GetUser" with:
      """
      {"id": "7"}
      """
    Then "users-client" call succeeds
    And "users-client" response json "name" is "Grace"
    And "users-client" response json "role" is "ROLE_UNSPECIFIED"
    And "users-client" exposes service "tomato.test.v1.UserService"
    And "users-client" service "tomato.test.v1.UserService" has method "WatchUsers"
    And "users-stub" last "tomato.test.v1.UserService/GetUser" request metadata "x-upstream" is "localhost:9995"

  Scenario: A method nobody stubbed
    When "users-client" calls "tomato.test.v1.UserService/ListUsers" with:
      """
      {}
      """
    Then "users-client" response status is "UNIMPLEMENTED"
    And "users-client" response error contains "has no stub for tomato.test.v1.UserService/ListUsers"
