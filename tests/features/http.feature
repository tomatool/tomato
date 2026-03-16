@http
Feature: HTTP Handler
  Test all HTTP handler steps

  # ============================================================================
  # Request Setup
  # ============================================================================

  Scenario: Set single header
    Given "api" header "X-Custom-Header" is "test-value"
    When "api" sends "GET" to "/echo"
    Then "api" response status is "200"
    And "api" response body contains "X-Custom-Header"
    And "api" response body contains "test-value"

  Scenario: Set multiple headers from table
    Given "api" headers are:
      | header          | value           |
      | X-First-Header  | first-value     |
      | X-Second-Header | second-value    |
      | Authorization   | Bearer token123 |
    When "api" sends "GET" to "/echo"
    Then "api" response status is "200"
    And "api" response body contains "first-value"
    And "api" response body contains "second-value"
    And "api" response body contains "Bearer token123"

  Scenario: Set query parameter
    Given "api" query param "search" is "hello"
    And "api" query param "page" is "1"
    When "api" sends "GET" to "/echo"
    Then "api" response status is "200"
    And "api" response body contains "search"
    And "api" response body contains "hello"
    And "api" response body contains "page"

  Scenario: Set raw body
    Given "api" header "Content-Type" is "text/plain"
    And "api" body is:
      """
      This is raw text body content
      """
    When "api" sends "POST" to "/echo"
    Then "api" response status is "200"

  Scenario: Set JSON body shorthand
    Given "api" json body is:
      """
      {"name": "TestUser", "email": "test@example.com"}
      """
    When "api" sends "POST" to "/users"
    Then "api" response status is "201"
    And "api" response json "name" is "TestUser"

  Scenario: Set form-encoded body
    Given "api" form body is:
      | field    | value          |
      | username | testuser       |
      | password | secret123      |
    When "api" sends "POST" to "/form"
    Then "api" response status is "200"
    And "api" response body contains "testuser"
    And "api" response body contains "secret123"

  # ============================================================================
  # Request Execution
  # ============================================================================

  Scenario: Send GET request
    When "api" sends "GET" to "/health"
    Then "api" response status is "200"

  Scenario: Send POST request with JSON body inline
    Given "api" header "Content-Type" is "application/json"
    When "api" sends "POST" to "/users" with json:
      """
      {"name": "Alice", "email": "alice@test.com"}
      """
    Then "api" response status is "201"
    And "api" response json "name" is "Alice"

  Scenario: Send POST request with raw body inline
    Given "api" header "Content-Type" is "text/plain"
    When "api" sends "POST" to "/echo" with body:
      """
      raw body content here
      """
    Then "api" response status is "200"

  Scenario: Send DELETE request
    Given "db" table "users" has values:
      | id  | name        | email            |
      | 500 | DeleteMe    | delete@test.com  |
    When "api" sends "DELETE" to "/users/500"
    Then "api" response status is "204"

  # ============================================================================
  # Response Status
  # ============================================================================

  Scenario: Assert exact status code
    When "api" sends "GET" to "/health"
    Then "api" response status is "200"

  Scenario: Assert success status class (2xx)
    When "api" sends "GET" to "/health"
    Then "api" response status is success

  Scenario: Assert client error status class (4xx)
    When "api" sends "GET" to "/users/99999"
    Then "api" response status is "404"
    And "api" response status is client error

  Scenario: Assert server error status class (5xx)
    When "api" sends "GET" to "/error"
    Then "api" response status is "500"
    And "api" response status is server error

  # ============================================================================
  # Response Headers
  # ============================================================================

  Scenario: Assert exact header value
    When "api" sends "GET" to "/complex"
    Then "api" response header "X-Custom-Header" is "custom-value"

  Scenario: Assert header contains substring
    When "api" sends "GET" to "/health"
    Then "api" response header "Content-Type" contains "json"

  Scenario: Assert header exists
    When "api" sends "GET" to "/complex"
    Then "api" response header "X-Request-Id" exists
    And "api" response header "X-Custom-Header" exists

  # ============================================================================
  # Response Body
  # ============================================================================

  Scenario: Assert exact body match
    When "api" sends "GET" to "/health"
    Then "api" response body is:
      """
      {"status":"ok"}
      """

  Scenario: Assert body contains substring
    When "api" sends "GET" to "/health"
    Then "api" response body contains "status"
    And "api" response body contains "ok"

  Scenario: Assert body does not contain substring
    When "api" sends "GET" to "/health"
    Then "api" response body does not contain "error"
    And "api" response body does not contain "failed"

  Scenario: Assert empty body
    When "api" sends "GET" to "/empty"
    Then "api" response status is "204"
    And "api" response body is empty

  # ============================================================================
  # Response JSON - Basic
  # ============================================================================

  Scenario: Assert JSON path value
    When "api" sends "GET" to "/complex"
    Then "api" response json "id" is "550e8400-e29b-41d4-a716-446655440000"
    And "api" response json "email" is "test@example.com"
    And "api" response json "count" is "42"
    And "api" response json "active" is "true"

  Scenario: Assert JSON path exists
    When "api" sends "GET" to "/complex"
    Then "api" response json "id" exists
    And "api" response json "metadata" exists
    And "api" response json "items" exists

  Scenario: Assert JSON path does not exist
    When "api" sends "GET" to "/complex"
    Then "api" response json "nonexistent" does not exist
    And "api" response json "data.missing" does not exist

  Scenario: Assert nested JSON path
    When "api" sends "GET" to "/complex"
    Then "api" response json "metadata.version" is "1.0"
    And "api" response json "metadata.nested.deep" is "value"

  Scenario: Assert array element JSON path
    When "api" sends "GET" to "/complex"
    Then "api" response json "tags[0]" is "api"
    And "api" response json "tags[1]" is "test"
    And "api" response json "items[0].name" is "item1"
    And "api" response json "items[1].qty" is "20"

  # ============================================================================
  # Response JSON - Pattern Matching
  # ============================================================================

  Scenario: Assert JSON path matches regex pattern
    When "api" sends "GET" to "/complex"
    Then "api" response json "id" matches pattern "^[0-9a-f-]{36}$"
    And "api" response json "email" matches pattern "^[a-z]+@[a-z]+\.[a-z]+$"

  Scenario: Assert JSON path is UUID
    When "api" sends "GET" to "/complex"
    Then "api" response json "id" is uuid

  Scenario: Assert JSON path is email
    When "api" sends "GET" to "/complex"
    Then "api" response json "email" is email

  Scenario: Assert JSON path is ISO timestamp
    When "api" sends "GET" to "/complex"
    Then "api" response json "created_at" is iso-timestamp

  # ============================================================================
  # Response JSON - Matchers (using contains for partial matching)
  # ============================================================================

  Scenario: JSON contains with type matchers
    When "api" sends "GET" to "/complex"
    Then "api" response json contains:
      """
      {
        "id": "@string",
        "email": "@string",
        "count": "@number",
        "price": "@number",
        "active": "@boolean",
        "tags": "@array",
        "metadata": "@object",
        "items": "@array",
        "empty_string": "@string",
        "empty_array": "@array",
        "empty_object": "@object",
        "null_value": "@null"
      }
      """

  Scenario: JSON contains with value matchers
    When "api" sends "GET" to "/complex"
    Then "api" response json contains:
      """
      {
        "id": "@regex:^[0-9a-f-]{36}$",
        "email": "@contains:@example",
        "created_at": "@startswith:2024",
        "count": "@gt:40",
        "price": "@lt:100",
        "tags": "@len:3",
        "metadata": "@notempty",
        "items": "@notempty"
      }
      """

  Scenario: JSON contains with comparison matchers
    When "api" sends "GET" to "/complex"
    Then "api" response json contains:
      """
      {
        "count": "@gte:42",
        "price": "@lte:19.99"
      }
      """

  Scenario: JSON contains with empty/notempty matchers
    When "api" sends "GET" to "/complex"
    Then "api" response json contains:
      """
      {
        "empty_string": "@empty",
        "empty_array": "@empty",
        "empty_object": "@empty",
        "metadata": "@notempty",
        "tags": "@notempty"
      }
      """

  Scenario: JSON contains with @any matcher
    When "api" sends "GET" to "/complex"
    Then "api" response json contains:
      """
      {
        "id": "@any",
        "email": "@any",
        "count": "@any",
        "active": "@any"
      }
      """

  Scenario: JSON contains with @notnull matcher
    When "api" sends "GET" to "/complex"
    Then "api" response json contains:
      """
      {
        "id": "@notnull",
        "email": "@notnull",
        "metadata": "@notnull"
      }
      """

  # ============================================================================
  # Response JSON - Exact Match (response json matches)
  # ============================================================================

  Scenario: JSON matches exact structure with @endswith matcher
    When "api" sends "GET" to "/health"
    Then "api" response json matches:
      """
      {
        "status": "@endswith:ok"
      }
      """

  # ============================================================================
  # Response JSON - Contains (partial matching)
  # ============================================================================

  Scenario: JSON contains subset of fields
    When "api" sends "GET" to "/complex"
    Then "api" response json contains:
      """
      {
        "id": "550e8400-e29b-41d4-a716-446655440000",
        "email": "test@example.com"
      }
      """

  Scenario: JSON contains with matchers
    When "api" sends "GET" to "/complex"
    Then "api" response json contains:
      """
      {
        "id": "@string",
        "count": "@gt:0",
        "active": "@boolean"
      }
      """

  Scenario: JSON contains nested object
    When "api" sends "GET" to "/complex"
    Then "api" response json contains:
      """
      {
        "metadata": {
          "version": "1.0"
        }
      }
      """

  # ============================================================================
  # Response Timing
  # ============================================================================

  Scenario: Assert response time is fast
    When "api" sends "GET" to "/health"
    Then "api" response time is less than "5s"

  Scenario: Assert response time with milliseconds
    When "api" sends "GET" to "/health"
    Then "api" response time is less than "5000ms"

  # ============================================================================
  # Variable Capture and Usage
  # ============================================================================

  Scenario: Save JSON path to variable and use in next request
    Given "api" header "Content-Type" is "application/json"
    When "api" sends "POST" to "/users" with json:
      """
      {"name": "VariableUser", "email": "varuser@test.com"}
      """
    Then "api" response status is "201"
    And "api" response json "id" saved as "{{user_id}}"
    When "api" sends "GET" to "/users/{{user_id}}"
    Then "api" response status is "200"
    And "api" response json "name" is "VariableUser"

  Scenario: Save header to variable
    When "api" sends "GET" to "/complex"
    Then "api" response status is "200"
    And "api" response header "X-Request-Id" saved as "{{request_id}}"
    Given "api" header "X-Correlation-Id" is "{{request_id}}"
    When "api" sends "GET" to "/echo"
    Then "api" response body contains "550e8400-e29b-41d4-a716-446655440000"

  # ============================================================================
  # Combined/Complex Scenarios
  # ============================================================================

  Scenario: Full CRUD workflow
    # Create
    Given "api" header "Content-Type" is "application/json"
    When "api" sends "POST" to "/users" with json:
      """
      {"name": "CRUDUser", "email": "crud@test.com"}
      """
    Then "api" response status is "201"
    And "api" response json "id" exists
    And "api" response json "id" saved as "{{crud_user_id}}"
    # Read
    When "api" sends "GET" to "/users/{{crud_user_id}}"
    Then "api" response status is "200"
    And "api" response json "name" is "CRUDUser"
    And "api" response json "email" is "crud@test.com"
    # Delete
    When "api" sends "DELETE" to "/users/{{crud_user_id}}"
    Then "api" response status is "204"
    And "api" response body is empty
    # Verify deleted
    When "api" sends "GET" to "/users/{{crud_user_id}}"
    Then "api" response status is "404"

  Scenario: Request with multiple setup steps
    Given "api" header "Content-Type" is "application/json"
    And "api" header "Accept" is "application/json"
    And "api" header "X-API-Key" is "test-api-key"
    And "api" query param "include" is "metadata"
    And "api" query param "format" is "full"
    When "api" sends "GET" to "/echo"
    Then "api" response status is "200"
    And "api" response body contains "X-Api-Key"
    And "api" response body contains "include"
    And "api" response body contains "format"
