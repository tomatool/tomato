@lifecycle
Feature: Stopping and starting the app and its containers
  The lifecycle resource takes the app or a dependency down mid-scenario, the
  way production fails, and brings everything back before the next scenario.

  Scenario: The app restarts with a changed environment
    When "lifecycle" restarts the app with env:
      | name             | value |
      | TOMATO_LIFECYCLE | on    |
    And "app-client" sends "GET" to "/env?name=TOMATO_LIFECYCLE"
    Then "app-client" response json "value" is "on"
    When "lifecycle" restarts the app
    And "app-client" sends "GET" to "/env?name=TOMATO_LIFECYCLE"
    Then "app-client" response json "value" is "on"

  Scenario: Every scenario starts with the app's configured environment
    When "app-client" sends "GET" to "/env?name=TOMATO_LIFECYCLE"
    Then "app-client" response json "set" is "false"

  Scenario: The app goes down and comes back
    When "lifecycle" stops the app
    Then "shell" runs "curl -s -o /dev/null http://localhost:8080/health"
    And "shell" exit code is "7"
    When "lifecycle" starts the app
    And "app-client" sends "GET" to "/health"
    Then "app-client" response status is "200"

  Scenario: A dependency goes down and comes back on the same address
    When "lifecycle" stops container "redis"
    And "app-client" sends "GET" to "/cache?key=missing"
    Then "app-client" response status is "500"
    When "lifecycle" starts container "redis"
    And "app-client" sends "GET" to "/cache?key=missing"
    Then "app-client" response status is "404"

  Scenario: A database restarts under a connected app
    Given "app-client" sends "POST" to "/users" with json:
      """
      {"name": "Before", "email": "before@test.com"}
      """
    And "app-client" response status is "201"
    When "lifecycle" restarts container "postgres"
    # Postgres opens its port before it accepts queries, so a port wait passes
    # first. Wait for the database itself, then check the app writes again.
    And "app-db" query "SELECT 1 AS up" returns within "30s":
      | up |
      | 1  |
    And "app-client" sends "POST" to "/users" with json:
      """
      {"name": "After", "email": "after@test.com"}
      """
    Then "app-client" response status is "201"
    And "app-db" table "users" has "2" rows

  Scenario: A hung dependency times the app out, then recovers
    When "lifecycle" pauses container "redis"
    And "app-client" sends "GET" to "/cache?key=missing"
    Then "app-client" response status is "500"
    When "lifecycle" unpauses container "redis"
    And "app-client" sends "GET" to "/cache?key=missing"
    Then "app-client" response status is "404"

  Scenario: Every scenario starts with its containers up
    When "app-client" sends "GET" to "/cache?key=missing"
    Then "app-client" response status is "404"
