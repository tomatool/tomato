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

  # Each step reads and writes through the app as well as through tomato, or
  # with literal names on one side: a key that is {{order_id}} on both sides
  # would match itself whether or not anything was substituted.
  Scenario: Keys named after a value from an earlier step
    When "app-client" sends "POST" to "/echo" with json:
      """
      {"id": "{{uuid}}"}
      """
    And "app-client" response json "body.id" saved as "{{order_id}}"
    And "app-client" sends "POST" to "/cache?key=lock:{{order_id}}" with json:
      """
      {"value": "held by {{order_id}}"}
      """
    Then "app-cache" key "lock:{{order_id}}" exists
    And "app-cache" key "lock:{{order_id}}" has value "held by {{order_id}}"
    And "app-cache" key "lock:{{order_id}}" contains "{{order_id}}"
    And "app-cache" key "lock:{{order_id}}" has TTL greater than "3500" seconds
    When "app-cache" key "session:{{order_id}}" is "user-{{order_id}}"
    And "app-client" sends "GET" to "/cache?key=session:{{order_id}}"
    Then "app-client" response status is "200"
    And "app-client" response json "value" is "user-{{order_id}}"
    When "app-cache" key "lock:{{order_id}}" is deleted
    And "app-client" sends "GET" to "/cache?key=lock:{{order_id}}"
    Then "app-client" response status is "404"

  Scenario: Every step substitutes variables in its keys, values, fields and members
    When "app-client" sends "POST" to "/echo" with json:
      """
      {"n": "42"}
      """
    And "app-client" response json "body.n" saved as "{{n}}"
    And "app-cache" key "t:{{n}}" is "v-{{n}}" with TTL "1h"
    And "app-cache" key "doc:{{n}}" is:
      """
      {"id": "{{n}}"}
      """
    And "app-cache" hash "h:{{n}}" has fields:
      | field   | value   |
      | f-{{n}} | v-{{n}} |
    And "app-cache" list "l:{{n}}" has "v-{{n}}"
    And "app-cache" list "l:{{n}}" has values:
      | w-{{n}} |
    And "app-cache" set "s:{{n}}" has "m-{{n}}"
    And "app-cache" set "s:{{n}}" has members:
      | n-{{n}} |
    And "app-cache" key "c:{{n}}" is incremented by "3"
    And "app-cache" key "c:{{n}}" is incremented
    And "app-cache" key "c:{{n}}" is decremented
    Then "app-cache" key "t:42" has value "v-42"
    And "app-cache" key "doc:42" contains "42"
    And "app-cache" hash "h:42" field "f-42" is "v-42"
    And "app-cache" list "l:42" contains "w-42"
    And "app-cache" set "s:42" contains "n-42"
    And "app-cache" key "c:42" has value "3"
    And "app-cache" key "t:{{n}}" has TTL greater than "3500" seconds
    And "app-cache" hash "h:{{n}}" contains:
      | field   | value   |
      | f-{{n}} | v-{{n}} |
    And "app-cache" list "l:{{n}}" has "2" items
    And "app-cache" list "l:{{n}}" contains "v-{{n}}"
    And "app-cache" set "s:{{n}}" has "2" members
    And "app-cache" set "s:{{n}}" contains "m-{{n}}"
    When "app-cache" key "t:{{n}}" is deleted
    Then "app-cache" key "t:42" does not exist
    And "app-cache" key "t:{{n}}" does not exist
