@redis
Feature: Redis Handler
  Test all Redis handler steps

  Scenario: Set and get string keys
    Given "app-cache" key "greeting" is "hello world"
    Then "app-cache" key "greeting" exists
    And "app-cache" key "greeting" has value "hello world"
    And "app-cache" key "greeting" contains "hello"

  Scenario: Key with TTL
    Given "app-cache" key "temp" is "expires soon" with TTL "1h"
    Then "app-cache" key "temp" exists
    And "app-cache" key "temp" has TTL greater than "3500" seconds

  Scenario: Delete key
    Given "app-cache" key "to-delete" is "bye"
    When "app-cache" key "to-delete" is deleted
    Then "app-cache" key "to-delete" does not exist

  Scenario: Set JSON value
    Given "app-cache" key "user:json" is:
      """
      {"id": 1, "name": "Test User"}
      """
    Then "app-cache" key "user:json" exists
    And "app-cache" key "user:json" contains "Test User"

  Scenario: Keys, values, fields and members substitute saved variables
    Given "app-client" sends "POST" to "/login"
    And "app-client" response header "Location" query param "session" saved as "{{session_id}}"
    And "app-cache" key "session:{{session_id}}" is "{{session_id}}"
    And "app-cache" hash "user:{{session_id}}" has fields:
      | field   | value          |
      | session | {{session_id}} |
    And "app-cache" list "sessions" has "{{session_id}}"
    And "app-cache" set "active" has "{{session_id}}"
    # Stored under the substituted names, not the placeholders
    Then "app-cache" key "session:sess-42" has value "sess-42"
    And "app-cache" key "session:{{session_id}}" exists
    And "app-cache" key "session:{{session_id}}" contains "{{session_id}}"
    And "app-cache" hash "user:sess-42" field "session" is "{{session_id}}"
    And "app-cache" list "sessions" contains "sess-42"
    And "app-cache" set "active" contains "{{session_id}}"

  Scenario: A substituted key that was never written does not exist
    Given "app-client" sends "POST" to "/login"
    And "app-client" response header "Location" query param "session" saved as "{{session_id}}"
    And "app-cache" key "{{session_id}}" is "written"
    # The placeholder isn't looked up literally, so this checks the real key
    Then "app-cache" key "session:{{session_id}}" does not exist
    # And the key written was the substituted one
    And "app-cache" key "sess-42" has value "written"

  Scenario: Hash operations
    Given "app-cache" hash "user:100" has fields:
      | field | value           |
      | name  | Grace           |
      | email | grace@test.com  |
    Then "app-cache" hash "user:100" field "name" is "Grace"
    And "app-cache" hash "user:100" contains:
      | field | value           |
      | name  | Grace           |
      | email | grace@test.com  |

  Scenario: List operations
    Given "app-cache" list "queue" has "first"
    And "app-cache" list "queue" has "second"
    And "app-cache" list "queue" has "third"
    Then "app-cache" list "queue" has "3" items
    And "app-cache" list "queue" contains "second"

  Scenario: Push multiple to list
    Given "app-cache" list "items" has values:
      | item-a |
      | item-b |
      | item-c |
    Then "app-cache" list "items" has "3" items

  Scenario: Set operations
    Given "app-cache" set "tags" has "tag1"
    And "app-cache" set "tags" has "tag2"
    Then "app-cache" set "tags" contains "tag1"
    And "app-cache" set "tags" has "2" members

  Scenario: Add multiple to set
    Given "app-cache" set "colors" has members:
      | red   |
      | green |
      | blue  |
    Then "app-cache" set "colors" has "3" members

  Scenario: Increment and decrement
    Given "app-cache" key "counter" is "10"
    When "app-cache" key "counter" is incremented
    Then "app-cache" key "counter" has value "11"
    When "app-cache" key "counter" is incremented by "5"
    Then "app-cache" key "counter" has value "16"
    When "app-cache" key "counter" is decremented
    Then "app-cache" key "counter" has value "15"

  Scenario: Check database state
    Given "app-cache" key "a" is "1"
    And "app-cache" key "b" is "2"
    Then "app-cache" has "2" keys

  Scenario: Check database is empty
    Then "app-cache" is empty
