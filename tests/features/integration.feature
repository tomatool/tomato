@integration
Feature: Integration Tests
  Test scenarios that combine multiple handlers to verify end-to-end behavior
  Note: Database tables are automatically truncated before each scenario via reset

  Scenario: HTTP creates user, Postgres verifies, HTTP reads back
    # Create user via HTTP API
    Given "api" header "Content-Type" is "application/json"
    When "api" sends "POST" to "/users" with json:
      """
      {"name": "Integration User", "email": "integration@test.com"}
      """
    Then "api" response status is "201"
    And "api" response json "id" exists

    # Verify user exists in database
    Then "db" table "users" has "1" rows
    And "db" table "users" contains:
      | name             | email                  |
      | Integration User | integration@test.com   |

    # Read user back via HTTP API
    When "api" sends "GET" to "/users"
    Then "api" response status is "200"
    And "api" response body contains "Integration User"
    And "api" response body contains "integration@test.com"

  Scenario: Postgres inserts data, HTTP reads it
    # Insert user directly into database
    Given "db" table "users" has values:
      | id  | name         | email              |
      | 100 | Direct User  | direct@test.com    |

    # Verify via HTTP API
    When "api" sends "GET" to "/users/100"
    Then "api" response status is "200"
    And "api" response json "name" is "Direct User"
    And "api" response json "email" is "direct@test.com"

  Scenario: HTTP sets cache, Redis verifies
    # Set cache via HTTP API
    Given "api" header "Content-Type" is "application/json"
    And "api" query param "key" is "http-cache-key"
    When "api" sends "POST" to "/cache" with json:
      """
      {"value": "cached-via-http", "ttl": 3600}
      """
    Then "api" response status is "201"

    # Verify in Redis directly
    Then "cache" key "http-cache-key" exists
    And "cache" key "http-cache-key" has value "cached-via-http"

  Scenario: Redis sets data, HTTP reads it
    # Set cache directly in Redis
    Given "cache" key "redis-direct-key" is "set-via-redis"

    # Read via HTTP API
    Given "api" query param "key" is "redis-direct-key"
    When "api" sends "GET" to "/cache"
    Then "api" response status is "200"
    And "api" response body contains "set-via-redis"

  Scenario: Shell creates file, verifies with file assertion
    # Create a file with shell
    When "shell" runs "echo 'test data for integration' > /tmp/integration-test.txt"
    Then "shell" succeeds

    # Verify file exists and has content
    Then "shell" file "/tmp/integration-test.txt" exists
    And "shell" file "/tmp/integration-test.txt" contains "test data for integration"

    # Clean up
    When "shell" runs "rm /tmp/integration-test.txt"
    Then "shell" file "/tmp/integration-test.txt" does not exist

@skip-dynamic-port
  Scenario: Shell runs curl to test HTTP endpoint
    # Note: This test is skipped when app runs on dynamic port
    # Use shell to make HTTP request via curl
    When "shell" runs "curl -s http://localhost:8080/health"
    Then "shell" succeeds
    And "shell" stdout contains "ok"
    And "shell" stdout contains "status"

  Scenario: Multiple users workflow - create, list, delete
    # Create multiple users
    Given "api" header "Content-Type" is "application/json"

    When "api" sends "POST" to "/users" with json:
      """
      {"name": "User One", "email": "one@test.com"}
      """
    Then "api" response status is "201"

    When "api" sends "POST" to "/users" with json:
      """
      {"name": "User Two", "email": "two@test.com"}
      """
    Then "api" response status is "201"

    # Verify count in database
    Then "db" table "users" has "2" rows

    # List via API
    When "api" sends "GET" to "/users"
    Then "api" response status is "200"
    And "api" response body contains "User One"
    And "api" response body contains "User Two"

  Scenario: WebSocket receives echo while HTTP health check passes
    # Connect to WebSocket
    Given "ws" connects
    Then "ws" is connected

    # Verify HTTP still works
    When "api" sends "GET" to "/health"
    Then "api" response status is "200"

    # Send message via WebSocket
    When "ws" sends "concurrent test"
    Then "ws" receives within "5s":
      """
      concurrent test
      """

    # Disconnect
    When "ws" disconnects
    Then "ws" is disconnected

  Scenario: Database transaction with HTTP verification
    # Insert via SQL
    Given "db" executes:
      """
      INSERT INTO users (id, name, email) VALUES (200, 'SQL User', 'sql@test.com')
      """

    # Update via SQL
    When "db" executes:
      """
      UPDATE users SET name = 'Updated SQL User' WHERE id = 200
      """

    # Verify via HTTP
    When "api" sends "GET" to "/users/200"
    Then "api" response status is "200"
    And "api" response json "name" is "Updated SQL User"

    # Verify in DB
    And "db" table "users" contains:
      | id  | name             |
      | 200 | Updated SQL User |

  Scenario: Environment variable affects shell command
    # Set environment variable
    Given "shell" env "TEST_VAR" is "integration-value"

    # Run command that uses the variable
    When "shell" runs "echo Value is: $TEST_VAR"
    Then "shell" succeeds
    And "shell" stdout contains "Value is: integration-value"

  Scenario: Redis cache with TTL verification
    # Set key with TTL
    Given "cache" key "ttl-test" is "expires" with TTL "1h"

    # Verify TTL
    Then "cache" key "ttl-test" has TTL greater than "3500" seconds

    # Verify value via HTTP
    Given "api" query param "key" is "ttl-test"
    When "api" sends "GET" to "/cache"
    Then "api" response status is "200"
    And "api" response body contains "expires"
