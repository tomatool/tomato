Feature: HTTP API Testing
  As a developer
  I want to test HTTP API endpoints
  So that I can verify my application's REST API works correctly

  Scenario: Simple GET request
    When I send "GET" request to "api" "/health"
    Then "api" response status should be "200"
    And "api" response body should contain "ok"

  Scenario: GET request with query parameters
    Given I set "api" query param "page" to "1"
    And I set "api" query param "limit" to "10"
    When I send "GET" request to "api" "/users"
    Then "api" response status should be "200"
    And "api" response status should be success

  Scenario: POST request with JSON body
    Given I set "api" header "Content-Type" to "application/json"
    When I send "POST" request to "api" "/users" with JSON:
      """
      {
        "name": "Alice",
        "email": "alice@test.com"
      }
      """
    Then "api" response status should be "201"
    And "api" response JSON "id" should exist
    And "api" response JSON "name" should be "Alice"
    And "api" response JSON "email" should be "alice@test.com"

  Scenario: Setting headers for request
    Given I set "api" headers:
      | header        | value            |
      | Authorization | Bearer token123  |
      | X-Request-ID  | req-456          |
    When I send "GET" request to "api" "/protected"
    Then "api" response status should be "200"

  Scenario: Form data submission
    Given I set "api" form body:
      | field    | value           |
      | username | alice           |
      | password | secret123       |
    When I send "POST" request to "api" "/login"
    Then "api" response status should be "200"
    And "api" response header "Set-Cookie" should exist

  Scenario: Response header assertions
    When I send "GET" request to "api" "/users"
    Then "api" response status should be "200"
    And "api" response header "Content-Type" should contain "application/json"
    And "api" response header "X-Request-ID" should exist

  Scenario: JSON path assertions
    When I send "GET" request to "api" "/users/1"
    Then "api" response status should be "200"
    And "api" response JSON "id" should be "1"
    And "api" response JSON "profile.name" should exist
    And "api" response JSON "roles[0]" should be "admin"

  Scenario: JSON structure matching with matchers
    When I send "GET" request to "api" "/users/1"
    Then "api" response status should be "200"
    And "api" response JSON should match:
      """
      {
        "id": "@number",
        "name": "@string",
        "email": "@string",
        "created_at": "@string",
        "active": "@boolean",
        "profile": "@object",
        "roles": "@array"
      }
      """

  Scenario: PUT request to update resource
    When I send "PUT" request to "api" "/users/1" with JSON:
      """
      {
        "name": "Alice Updated",
        "email": "alice.updated@test.com"
      }
      """
    Then "api" response status should be "200"
    And "api" response JSON "name" should be "Alice Updated"

  Scenario: DELETE request
    When I send "DELETE" request to "api" "/users/1"
    Then "api" response status should be "204"
    And "api" response body should be empty

  Scenario: Client error handling
    When I send "GET" request to "api" "/nonexistent"
    Then "api" response status should be "404"
    And "api" response status should be client error

  Scenario: Response time assertion
    When I send "GET" request to "api" "/health"
    Then "api" response status should be "200"
    And "api" response time should be less than "1s"

  Scenario: Negative body assertion
    When I send "GET" request to "api" "/public"
    Then "api" response status should be "200"
    And "api" response body should not contain "password"
    And "api" response body should not contain "secret"
