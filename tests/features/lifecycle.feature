@lifecycle
Feature: Stopping and starting the app and its containers
  The lifecycle resource takes the app or a dependency down mid-scenario, the
  way production fails, and brings everything back before the next scenario.

  Scenario: The app restarts with a changed environment
    When "lifecycle" restarts the app with env:
      | name             | value |
      | TOMATO_LIFECYCLE | on    |
    And "api" sends "GET" to "/env?name=TOMATO_LIFECYCLE"
    Then "api" response json "value" is "on"
    When "lifecycle" restarts the app
    And "api" sends "GET" to "/env?name=TOMATO_LIFECYCLE"
    Then "api" response json "value" is "on"

  Scenario: Every scenario starts with the app's configured environment
    When "api" sends "GET" to "/env?name=TOMATO_LIFECYCLE"
    Then "api" response json "set" is "false"

  Scenario: The app goes down and comes back
    When "lifecycle" stops the app
    Then "shell" runs "curl -s -o /dev/null http://localhost:8080/health"
    And "shell" exit code is "7"
    When "lifecycle" starts the app
    And "api" sends "GET" to "/health"
    Then "api" response status is "200"

  Scenario: A dependency goes down and comes back on the same address
    When "lifecycle" stops container "redis"
    And "api" sends "GET" to "/cache?key=missing"
    Then "api" response status is "500"
    When "lifecycle" starts container "redis"
    And "api" sends "GET" to "/cache?key=missing"
    Then "api" response status is "404"

  Scenario: A database restarts under a connected app
    Given "api" sends "POST" to "/users" with json:
      """
      {"name": "Before", "email": "before@test.com"}
      """
    And "api" response status is "201"
    When "lifecycle" restarts container "postgres"
    And "api" sends "POST" to "/users" with json:
      """
      {"name": "After", "email": "after@test.com"}
      """
    Then "api" response status is "201"
    And "db" table "users" has "2" rows

  Scenario: A hung dependency times the app out, then recovers
    When "lifecycle" pauses container "redis"
    And "api" sends "GET" to "/cache?key=missing"
    Then "api" response status is "500"
    When "lifecycle" unpauses container "redis"
    And "api" sends "GET" to "/cache?key=missing"
    Then "api" response status is "404"

  Scenario: Every scenario starts with its containers up
    When "api" sends "GET" to "/cache?key=missing"
    Then "api" response status is "404"
