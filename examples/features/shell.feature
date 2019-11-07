Feature: shell features example

  Scenario: Create file and check if file exist
    Given "shell-cli" execute "touch helloworld"
    Given "shell-cli" exit code equal to 0
    Given "shell-cli" execute "rm some-not-exist-file"
    Given "shell-cli" exit code not equal to 0
    Then "shell-cli" execute "ls"
    Then "shell-cli" stdout should contains "helloworld"
    Then "shell-cli" execute "rm helloworld"
    Then "ls" execute "."
    Then "ls" stdout should not contains "helloworld"
