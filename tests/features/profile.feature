@profile
Feature: Profiling the app per scenario
  tomato measures the app while the scenarios run, from outside: its CPU and
  memory, the errors it logs, and the dependencies it waits on. These run
  tomato against tests/testdata/profile, whose app spends a known amount of
  CPU and memory or logs an error on request, and check what it reports.

  Scenario: Each scenario's profile is in the event stream
    When "shell" runs:
      """
      sh tests/testdata/cli/tomato.sh run -c tests/testdata/profile/tomato.yml --format tomato | python3 tests/testdata/profile/check.py
      """
    Then "shell" succeeds
    And "shell" stdout contains "profile ok"

  Scenario: The terminal shows the heaviest scenarios and the errors the app logged
    When "shell" runs:
      """
      sh tests/testdata/cli/tomato.sh run -c tests/testdata/profile/tomato.yml
      """
    Then "shell" succeeds
    And "shell" stdout contains "Profile"
    And "shell" stdout contains "burns CPU"
    And "shell" stdout contains "The app logged errors in 1 scenario:"
    And "shell" stdout contains "logs an error: 1 error"
    And "shell" stdout contains "profile.json"

  Scenario: settings.profile false turns it off
    When "shell" runs:
      """
      { printf 'settings:\n  profile: false\n'; cat tests/testdata/profile/tomato.yml; } > tests/testdata/profile/tomato.off.yml
      sh tests/testdata/cli/tomato.sh run -c tests/testdata/profile/tomato.off.yml
      status=$?
      rm -f tests/testdata/profile/tomato.off.yml
      exit $status
      """
    Then "shell" succeeds
    And "shell" stdout does not contain "Profile"
