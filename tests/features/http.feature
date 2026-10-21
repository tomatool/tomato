@http
Feature: HTTP Handler
  Test all HTTP handler steps

  Scenario: Send GET request and check status
    When "app-client" sends "GET" to "/health"
    Then "app-client" response status is "200"
    And "app-client" response status is success

  Scenario: Send GET request and check JSON response
    When "app-client" sends "GET" to "/health"
    Then "app-client" response status is "200"
    And "app-client" response json "status" is "ok"
    And "app-client" response json "status" exists

  Scenario: Send POST request with JSON body
    Given "app-client" header "Content-Type" is "application/json"
    When "app-client" sends "POST" to "/users" with json:
      """
      {"name": "Alice", "email": "alice@test.com"}
      """
    Then "app-client" response status is "201"
    And "app-client" response json "id" exists
    And "app-client" response json "name" is "Alice"

  Scenario: Set headers and query params
    Given "app-client" header "X-Custom-Header" is "test-value"
    And "app-client" query param "key" is "test-key"
    When "app-client" sends "GET" to "/echo"
    Then "app-client" response status is "200"
    And "app-client" response body contains "test-value"
    And "app-client" response body contains "test-key"

  Scenario: Check response headers
    When "app-client" sends "GET" to "/health"
    Then "app-client" response header "Content-Type" contains "json"
    And "app-client" response header "Content-Type" exists

  Scenario: Send request with JSON body shorthand
    Given "app-client" json body is:
      """
      {"name": "Bob", "email": "bob@test.com"}
      """
    When "app-client" sends "POST" to "/users"
    Then "app-client" response status is "201"

  Scenario: Check response body
    When "app-client" sends "GET" to "/health"
    Then "app-client" response body contains "ok"
    And "app-client" response body does not contain "error"

  Scenario: Check 404 response
    When "app-client" sends "GET" to "/users/99999"
    Then "app-client" response status is "404"
    And "app-client" response status is client error

  Scenario: Response time check
    When "app-client" sends "GET" to "/health"
    Then "app-client" response time is less than "5s"
