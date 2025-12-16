@http
Feature: HTTP Handler
  Test all HTTP handler steps

  Scenario: Send GET request and check status
    When "api" sends "GET" to "/health"
    Then "api" response status is "200"
    And "api" response status is success

  Scenario: Send GET request and check JSON response
    When "api" sends "GET" to "/health"
    Then "api" response status is "200"
    And "api" response json "status" is "ok"
    And "api" response json "status" exists

  Scenario: Send POST request with JSON body
    Given "api" header "Content-Type" is "application/json"
    When "api" sends "POST" to "/users" with json:
      """
      {"name": "Alice", "email": "alice@test.com"}
      """
    Then "api" response status is "201"
    And "api" response json "id" exists
    And "api" response json "name" is "Alice"

  Scenario: Set headers and query params
    Given "api" header "X-Custom-Header" is "test-value"
    And "api" query param "key" is "test-key"
    When "api" sends "GET" to "/echo"
    Then "api" response status is "200"
    And "api" response body contains "test-value"
    And "api" response body contains "test-key"

  Scenario: Check response headers
    When "api" sends "GET" to "/health"
    Then "api" response header "Content-Type" contains "json"
    And "api" response header "Content-Type" exists

  Scenario: Send request with JSON body shorthand
    Given "api" json body is:
      """
      {"name": "Bob", "email": "bob@test.com"}
      """
    When "api" sends "POST" to "/users"
    Then "api" response status is "201"

  Scenario: Check response body
    When "api" sends "GET" to "/health"
    Then "api" response body contains "ok"
    And "api" response body does not contain "error"

  Scenario: Check 404 response
    When "api" sends "GET" to "/users/99999"
    Then "api" response status is "404"
    And "api" response status is client error

  Scenario: Response time check
    When "api" sends "GET" to "/health"
    Then "api" response time is less than "5s"
