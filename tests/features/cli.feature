Feature: tomato CLI
  Runs the tomato binary against a shell-only fixture project
  (tests/testdata/cli) through tests/testdata/cli/tomato.sh, which strips
  colors. TOMATO_BIN lets `make coverage` point it at the coverage-
  instrumented binary; it defaults to ./bin/tomato (`make build`).

  Scenario: "not" in a tag expression skips the tagged scenarios
    When "shell" runs:
      """
      sh tests/testdata/cli/tomato.sh run -c tests/testdata/cli/tomato.yml --tags "not @slow"
      """
    Then "shell" succeeds
    And "shell" stdout contains "2 scenarios (2 passed)"
    And "shell" stdout contains "fast only"
    And "shell" stdout contains "untagged"
    And "shell" stdout does not contain "slow only"

  Scenario: "and" and "not" combine
    When "shell" runs:
      """
      sh tests/testdata/cli/tomato.sh run -c tests/testdata/cli/tomato.yml --tags "@fast and not @slow"
      """
    Then "shell" succeeds
    And "shell" stdout contains "1 scenarios (1 passed)"
    And "shell" stdout contains "fast only"

  Scenario: "or" matches either tag
    When "shell" runs:
      """
      sh tests/testdata/cli/tomato.sh run -c tests/testdata/cli/tomato.yml --tags "@fast or @slow"
      """
    Then "shell" succeeds
    And "shell" stdout contains "3 scenarios (3 passed)"
    And "shell" stdout does not contain "untagged"

  Scenario: parentheses group a negated expression
    When "shell" runs:
      """
      sh tests/testdata/cli/tomato.sh run -c tests/testdata/cli/tomato.yml --tags "not (@fast or @slow)"
      """
    Then "shell" succeeds
    And "shell" stdout contains "1 scenarios (1 passed)"
    And "shell" stdout contains "untagged"

  Scenario: godog's native tag syntax still works
    When "shell" runs:
      """
      sh tests/testdata/cli/tomato.sh run -c tests/testdata/cli/tomato.yml --tags "@fast && ~@slow"
      """
    Then "shell" succeeds
    And "shell" stdout contains "1 scenarios (1 passed)"

  Scenario: an invalid tag expression fails with an error message
    When "shell" runs:
      """
      sh tests/testdata/cli/tomato.sh run -c tests/testdata/cli/tomato.yml --tags "@fast and (@slow"
      """
    Then "shell" fails
    And "shell" stderr contains "Error:"

  Scenario: validate prints its report when stdout is not a terminal
    When "shell" runs:
      """
      sh tests/testdata/cli/tomato.sh validate -c tests/testdata/cli/tomato.yml
      """
    Then "shell" succeeds
    And "shell" stdout contains "Validation passed!"
