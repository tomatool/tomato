Feature: gRPC resource
  Calls unary gRPC methods on the test app's grpc.health.v1.Health service,
  resolved entirely through server reflection — no .proto file and no
  generated stubs are checked in anywhere in this repository.

  Scenario: Call a unary method and assert on the response
    When "grpc" calls "grpc.health.v1.Health/Check" with:
      """
      {"service": "tomato"}
      """
    Then "grpc" call succeeds
    And "grpc" response status is "OK"
    And "grpc" response json "status" is "SERVING"
    And "grpc" response contains "SERVING"

  Scenario: Set the request separately from the call
    Given "grpc" request is:
      """
      {"service": "degraded"}
      """
    When "grpc" calls "grpc.health.v1.Health/Check"
    Then "grpc" call succeeds
    And "grpc" response json "status" is "NOT_SERVING"

  Scenario: A non-OK status is an assertable outcome, not a crash
    When "grpc" calls "grpc.health.v1.Health/Check" with:
      """
      {"service": "does-not-exist"}
      """
    Then "grpc" call fails
    And "grpc" response status is "NotFound"

  Scenario: Assert the exact response structure
    When "grpc" calls "grpc.health.v1.Health/Check" with:
      """
      {"service": "tomato"}
      """
    Then "grpc" response json matches:
      """
      {"status": "SERVING"}
      """

  Scenario: Partial matching ignores fields we do not care about
    When "grpc" calls "grpc.health.v1.Health/Check" with:
      """
      {"service": "tomato"}
      """
    Then "grpc" response json contains:
      """
      {"status": "@notempty"}
      """

  Scenario: Metadata is sent with the call
    Given "grpc" metadata "x-tomato" is "hello"
    And "grpc" metadata are:
      | key         | value  |
      | x-request-id | abc-123 |
    When "grpc" calls "grpc.health.v1.Health/Check" with:
      """
      {"service": "tomato"}
      """
    Then "grpc" call succeeds

  Scenario: Discover what the server exposes
    Then "grpc" exposes service "grpc.health.v1.Health"
    And "grpc" service "grpc.health.v1.Health" has method "Check"

  Scenario: Calls complete within a time budget
    When "grpc" calls "grpc.health.v1.Health/Check" with:
      """
      {"service": "tomato"}
      """
    Then "grpc" response time is less than "5s"

  Scenario: Assert what the response does and does not contain
    When "grpc" calls "grpc.health.v1.Health/Check" with:
      """
      {"service": "tomato"}
      """
    Then "grpc" response does not contain "NOT_SERVING"
    And "grpc" response json "status" exists
    And "grpc" response json "unknownField" does not exist

  Scenario: Assert on the error message of a failed call
    When "grpc" calls "grpc.health.v1.Health/Check" with:
      """
      {"service": "does-not-exist"}
      """
    Then "grpc" response status is "NotFound"
    And "grpc" response error contains "unknown service"
