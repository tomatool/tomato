Feature: shell features example

  Scenario: Create file and check if file exist
    Given "shell-cli" execute "touch helloworld"
    Then "shell-cli" execute "ls"
    Then "shell-cli" stdout should contains "helloworld"
    Then "shell-cli" execute "rm helloworld"
    Then "shell-cli" execute "ls"
    Then "shell-cli" stdout should not contains "helloworld"
