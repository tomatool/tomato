Feature: gRPC resource
  Calls unary gRPC methods on the test app's grpc.health.v1.Health service,
  resolved entirely through server reflection — no .proto file and no
  generated stubs are checked in anywhere in this repository.

  Scenario: Call a unary method and assert on the response
    When "app-grpc-client" calls "grpc.health.v1.Health/Check" with:
      """
      {"service": "tomato"}
      """
    Then "app-grpc-client" call succeeds
    And "app-grpc-client" response status is "OK"
    And "app-grpc-client" response json "status" is "SERVING"
    And "app-grpc-client" response contains "SERVING"

  Scenario: Set the request separately from the call
    Given "app-grpc-client" request is:
      """
      {"service": "degraded"}
      """
    When "app-grpc-client" calls "grpc.health.v1.Health/Check"
    Then "app-grpc-client" call succeeds
    And "app-grpc-client" response json "status" is "NOT_SERVING"

  Scenario: A non-OK status is an assertable outcome, not a crash
    When "app-grpc-client" calls "grpc.health.v1.Health/Check" with:
      """
      {"service": "does-not-exist"}
      """
    Then "app-grpc-client" call fails
    And "app-grpc-client" response status is "NotFound"

  Scenario: Assert the exact response structure
    When "app-grpc-client" calls "grpc.health.v1.Health/Check" with:
      """
      {"service": "tomato"}
      """
    Then "app-grpc-client" response json matches:
      """
      {"status": "SERVING"}
      """

  Scenario: Partial matching ignores fields we do not care about
    When "app-grpc-client" calls "grpc.health.v1.Health/Check" with:
      """
      {"service": "tomato"}
      """
    Then "app-grpc-client" response json contains:
      """
      {"status": "@notempty"}
      """

  Scenario: Metadata is sent with the call
    Given "app-grpc-client" metadata "x-tomato" is "hello"
    And "app-grpc-client" metadata are:
      | key         | value  |
      | x-request-id | abc-123 |
    When "app-grpc-client" calls "grpc.health.v1.Health/Check" with:
      """
      {"service": "tomato"}
      """
    Then "app-grpc-client" call succeeds

  Scenario: Discover what the server exposes
    Then "app-grpc-client" exposes service "grpc.health.v1.Health"
    And "app-grpc-client" service "grpc.health.v1.Health" has method "Check"

  Scenario: Calls complete within a time budget
    When "app-grpc-client" calls "grpc.health.v1.Health/Check" with:
      """
      {"service": "tomato"}
      """
    Then "app-grpc-client" response time is less than "5s"

  Scenario: Assert what the response does and does not contain
    When "app-grpc-client" calls "grpc.health.v1.Health/Check" with:
      """
      {"service": "tomato"}
      """
    Then "app-grpc-client" response does not contain "NOT_SERVING"
    And "app-grpc-client" response json "status" exists
    And "app-grpc-client" response json "unknownField" does not exist

  Scenario: Assert on the error message of a failed call
    When "app-grpc-client" calls "grpc.health.v1.Health/Check" with:
      """
      {"service": "does-not-exist"}
      """
    Then "app-grpc-client" response status is "NotFound"
    And "app-grpc-client" response error contains "unknown service"
