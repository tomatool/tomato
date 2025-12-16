@shell
Feature: Shell Handler
  Test all Shell handler steps

  Scenario: Run simple command
    When "shell" runs "echo 'Hello World'"
    Then "shell" succeeds
    And "shell" exit code is "0"
    And "shell" stdout contains "Hello World"

  Scenario: Run command with docstring
    When "shell" runs:
      """
      echo "Line 1"
      echo "Line 2"
      """
    Then "shell" succeeds
    And "shell" stdout contains "Line 1"
    And "shell" stdout contains "Line 2"

  Scenario: Check command failure
    When "shell" runs "exit 1"
    Then "shell" fails
    And "shell" exit code is "1"

  Scenario: Check stdout exactly
    When "shell" runs "printf 'exact'"
    Then "shell" stdout is:
      """
      exact
      """

  Scenario: Check empty stdout
    When "shell" runs "true"
    Then "shell" stdout is empty

  Scenario: Check stderr
    When "shell" runs "echo 'error message' >&2"
    Then "shell" stderr contains "error message"

  Scenario: Set environment variable
    Given "shell" env "MY_VAR" is "my-value"
    When "shell" runs "echo $MY_VAR"
    Then "shell" stdout contains "my-value"

  Scenario: Set working directory
    Given "shell" workdir is "/tmp"
    When "shell" runs "pwd"
    Then "shell" stdout contains "/tmp"

  Scenario: File assertions
    When "shell" runs "echo 'test content' > /tmp/tomato-test-file.txt"
    Then "shell" file "/tmp/tomato-test-file.txt" exists
    And "shell" file "/tmp/tomato-test-file.txt" contains "test content"
    When "shell" runs "rm /tmp/tomato-test-file.txt"
    Then "shell" file "/tmp/tomato-test-file.txt" does not exist

  Scenario: Command with timeout
    When "shell" runs with timeout "5s":
      """
      sleep 1 && echo "done"
      """
    Then "shell" succeeds
    And "shell" stdout contains "done"

  Scenario: Check stdout does not contain
    When "shell" runs "echo 'success'"
    Then "shell" stdout does not contain "error"
    And "shell" stdout does not contain "failure"
