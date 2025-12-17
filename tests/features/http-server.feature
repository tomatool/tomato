@http-server
Feature: HTTP Server Handler
  Test all HTTP server handler steps for stubbing external services

  # Stub Setup - Status only
  Scenario: Stub returns status code
    Given "mock" stub "GET" "/health" returns "200"
    When "api" sends "GET" to "http://localhost:9999/health"
    Then "api" response status is "200"

  # Stub Setup - With body
  Scenario: Stub returns body
    Given "mock" stub "GET" "/message" returns "200" with body:
      """
      Hello World
      """
    When "api" sends "GET" to "http://localhost:9999/message"
    Then "api" response status is "200"
    And "api" response body contains "Hello World"

  # Stub Setup - With JSON
  Scenario: Stub returns JSON
    Given "mock" stub "GET" "/users" returns "200" with json:
      """
      [{"id": 1, "name": "Alice"}, {"id": 2, "name": "Bob"}]
      """
    When "api" sends "GET" to "http://localhost:9999/users"
    Then "api" response status is "200"
    And "api" response header "Content-Type" contains "application/json"
    And "api" response json "[0].name" is "Alice"

  # Stub Setup - With headers
  Scenario: Stub returns with custom headers
    Given "mock" stub "GET" "/custom" returns "200" with headers:
      | header       | value           |
      | X-Custom     | custom-value    |
      | X-Request-Id | req-123         |
    When "api" sends "GET" to "http://localhost:9999/custom"
    Then "api" response status is "200"
    And "api" response header "X-Custom" contains "custom-value"
    And "api" response header "X-Request-Id" contains "req-123"

  # Stub Setup - Different methods
  Scenario: Stub POST endpoint
    Given "mock" stub "POST" "/items" returns "201" with json:
      """
      {"id": 100, "created": true}
      """
    Given "api" header "Content-Type" is "application/json"
    When "api" sends "POST" to "http://localhost:9999/items" with json:
      """
      {"name": "New Item"}
      """
    Then "api" response status is "201"
    And "api" response json "created" is "true"

  # Verification - Received request
  Scenario: Verify request was received
    Given "mock" stub "GET" "/check" returns "200"
    When "api" sends "GET" to "http://localhost:9999/check"
    Then "mock" received "GET" "/check"

  # Verification - Received N times
  Scenario: Verify request count
    Given "mock" stub "GET" "/counter" returns "200"
    When "api" sends "GET" to "http://localhost:9999/counter"
    And "api" sends "GET" to "http://localhost:9999/counter"
    And "api" sends "GET" to "http://localhost:9999/counter"
    Then "mock" received "GET" "/counter" "3" times

  # Verification - Did not receive
  Scenario: Verify request was not received
    Given "mock" stub "GET" "/exists" returns "200"
    When "api" sends "GET" to "http://localhost:9999/exists"
    Then "mock" did not receive "DELETE" "/exists"
    And "mock" did not receive "GET" "/other"

  # Verification - Header check
  Scenario: Verify request header
    Given "mock" stub "GET" "/auth" returns "200"
    Given "api" header "Authorization" is "Bearer token123"
    When "api" sends "GET" to "http://localhost:9999/auth"
    Then "mock" received request with header "Authorization" containing "Bearer"

  # Verification - Body check
  Scenario: Verify request body
    Given "mock" stub "POST" "/data" returns "200"
    Given "api" header "Content-Type" is "application/json"
    When "api" sends "POST" to "http://localhost:9999/data" with json:
      """
      {"action": "create", "value": 42}
      """
    Then "mock" received request with body containing "action"
    And "mock" received request with body containing "create"

  # Verification - Total requests
  Scenario: Verify total request count
    Given "mock" stub "GET" "/a" returns "200"
    And "mock" stub "GET" "/b" returns "200"
    And "mock" stub "POST" "/c" returns "201"
    When "api" sends "GET" to "http://localhost:9999/a"
    And "api" sends "GET" to "http://localhost:9999/b"
    Given "api" header "Content-Type" is "application/json"
    When "api" sends "POST" to "http://localhost:9999/c" with json:
      """
      {}
      """
    Then "mock" received "3" requests

  # No stub match returns 404
  Scenario: Unmatched request returns 404
    When "api" sends "GET" to "http://localhost:9999/not-stubbed"
    Then "api" response status is "404"

  # Multiple mock servers
  Scenario: Multiple mock servers work independently
    Given "mock" stub "GET" "/service-a" returns "200" with json:
      """
      {"service": "A"}
      """
    And "mock2" stub "GET" "/service-b" returns "200" with json:
      """
      {"service": "B"}
      """
    When "api" sends "GET" to "http://localhost:9999/service-a"
    Then "api" response status is "200"
    And "api" response json "service" is "A"
    When "api" sends "GET" to "http://localhost:9998/service-b"
    Then "api" response status is "200"
    And "api" response json "service" is "B"
    # Verify isolation - each mock only has its own stubs
    When "api" sends "GET" to "http://localhost:9999/service-b"
    Then "api" response status is "404"
    When "api" sends "GET" to "http://localhost:9998/service-a"
    Then "api" response status is "404"
