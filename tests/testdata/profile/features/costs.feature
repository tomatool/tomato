Feature: scenarios that cost something known

  Scenario: burns CPU
    When "app-client" sends "GET" to "/cpu?ms=400"
    Then "app-client" response status is "200"

  Scenario: holds memory
    When "app-client" sends "GET" to "/memory?mb=96"
    Then "app-client" response status is "200"

  Scenario: logs an error
    When "app-client" sends "GET" to "/log?level=ERROR"
    And "app-client" sends "GET" to "/log?level=WARN"
    Then "app-client" response status is "200"

  Scenario: does nothing much
    When "app-client" sends "GET" to "/health"
    Then "app-client" response status is "200"
