Feature: Redis Cache Operations
  As a developer
  I want to test Redis cache operations
  So that I can verify caching behavior in my application

  Scenario: Basic key-value operations
    Given I set "cache" key "user:1:name" with value "Alice"
    And I set "cache" key "user:1:email" with value "alice@test.com"
    Then "cache" key "user:1:name" should exist
    And "cache" key "user:1:name" should have value "Alice"
    And "cache" key "user:1:email" should have value "alice@test.com"
    And "cache" should have "2" keys

  Scenario: Key expiration with TTL
    Given I set "cache" key "session:abc123" with value "active" and TTL "10s"
    Then "cache" key "session:abc123" should exist
    And "cache" key "session:abc123" should have TTL greater than "5" seconds

  Scenario: JSON storage
    Given I set "cache" key "user:1:profile" with JSON:
      """
      {
        "name": "Alice",
        "email": "alice@test.com",
        "preferences": {
          "theme": "dark",
          "notifications": true
        }
      }
      """
    Then "cache" key "user:1:profile" should exist
    And "cache" key "user:1:profile" should contain "Alice"
    And "cache" key "user:1:profile" should contain "dark"

  Scenario: Hash operations
    Given I set "cache" hash "user:2" with fields:
      | field | value           |
      | name  | Bob             |
      | email | bob@test.com    |
      | role  | admin           |
    Then "cache" hash "user:2" field "name" should be "Bob"
    And "cache" hash "user:2" field "role" should be "admin"
    And "cache" hash "user:2" should contain:
      | field | value           |
      | name  | Bob             |
      | email | bob@test.com    |

  Scenario: List operations
    Given I push "task1" to "cache" list "queue:tasks"
    And I push "task2" to "cache" list "queue:tasks"
    And I push "task3" to "cache" list "queue:tasks"
    Then "cache" list "queue:tasks" should have "3" items
    And "cache" list "queue:tasks" should contain "task2"

  Scenario: Set operations
    Given I add "user:1" to "cache" set "online:users"
    And I add "user:2" to "cache" set "online:users"
    And I add "user:3" to "cache" set "online:users"
    Then "cache" set "online:users" should have "3" members
    And "cache" set "online:users" should contain "user:2"

  Scenario: Counter operations
    Given I set "cache" key "page:views" with value "100"
    When I increment "cache" key "page:views"
    Then "cache" key "page:views" should have value "101"
    When I increment "cache" key "page:views" by "10"
    Then "cache" key "page:views" should have value "111"
    When I decrement "cache" key "page:views"
    Then "cache" key "page:views" should have value "110"

  Scenario: Delete and verify cleanup
    Given I set "cache" key "temp:data" with value "temporary"
    Then "cache" key "temp:data" should exist
    When I delete "cache" key "temp:data"
    Then "cache" key "temp:data" should not exist

  Scenario: Reset verification
    # This scenario runs after reset - cache should be empty
    Then "cache" should be empty
