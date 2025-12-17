@redis
Feature: Redis Handler
  Test all Redis handler steps

  Scenario: Set and get string keys
    Given "cache" key "greeting" is "hello world"
    Then "cache" key "greeting" exists
    And "cache" key "greeting" has value "hello world"
    And "cache" key "greeting" contains "hello"

  Scenario: Key with TTL
    Given "cache" key "temp" is "expires soon" with TTL "1h"
    Then "cache" key "temp" exists
    And "cache" key "temp" has TTL greater than "3500" seconds

  Scenario: Delete key
    Given "cache" key "to-delete" is "bye"
    When "cache" key "to-delete" is deleted
    Then "cache" key "to-delete" does not exist

  Scenario: Set JSON value
    Given "cache" key "user:json" is:
      """
      {"id": 1, "name": "Test User"}
      """
    Then "cache" key "user:json" exists
    And "cache" key "user:json" contains "Test User"

  Scenario: Hash operations
    Given "cache" hash "user:100" has fields:
      | field | value           |
      | name  | Grace           |
      | email | grace@test.com  |
    Then "cache" hash "user:100" field "name" is "Grace"
    And "cache" hash "user:100" contains:
      | field | value           |
      | name  | Grace           |
      | email | grace@test.com  |

  Scenario: List operations
    Given "cache" list "queue" has "first"
    And "cache" list "queue" has "second"
    And "cache" list "queue" has "third"
    Then "cache" list "queue" has "3" items
    And "cache" list "queue" contains "second"

  Scenario: Push multiple to list
    Given "cache" list "items" has values:
      | item-a |
      | item-b |
      | item-c |
    Then "cache" list "items" has "3" items

  Scenario: Set operations
    Given "cache" set "tags" has "tag1"
    And "cache" set "tags" has "tag2"
    Then "cache" set "tags" contains "tag1"
    And "cache" set "tags" has "2" members

  Scenario: Add multiple to set
    Given "cache" set "colors" has members:
      | red   |
      | green |
      | blue  |
    Then "cache" set "colors" has "3" members

  Scenario: Increment and decrement
    Given "cache" key "counter" is "10"
    When "cache" key "counter" is incremented
    Then "cache" key "counter" has value "11"
    When "cache" key "counter" is incremented by "5"
    Then "cache" key "counter" has value "16"
    When "cache" key "counter" is decremented
    Then "cache" key "counter" has value "15"

  Scenario: Check database state
    Given "cache" key "a" is "1"
    And "cache" key "b" is "2"
    Then "cache" has "2" keys

  Scenario: Check database is empty
    Then "cache" is empty
