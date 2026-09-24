Feature: Tagged scenarios for tag-filter tests

  @fast
  Scenario: fast only
    When "shell" runs "echo fast-only"
    Then "shell" succeeds

  @slow
  Scenario: slow only
    When "shell" runs "echo slow-only"
    Then "shell" succeeds

  @fast @slow
  Scenario: fast and slow
    When "shell" runs "echo fast-and-slow"
    Then "shell" succeeds

  Scenario: untagged
    When "shell" runs "echo untagged"
    Then "shell" succeeds
