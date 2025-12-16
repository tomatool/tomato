Feature: User Management
  As a developer
  I want to test user database operations
  So that I can verify my application works correctly

  Scenario: Create and verify user
    Given I set "db" table "users" with values:
      | id | name  | email           |
      | 1  | Alice | alice@test.com  |
      | 2  | Bob   | bob@test.com    |
    Then "db" table "users" should have "2" rows
    And "db" table "users" should contain:
      | id | name  | email           |
      | 1  | Alice | alice@test.com  |
      | 2  | Bob   | bob@test.com    |

  Scenario: Empty table after reset
    # This scenario verifies that reset works correctly
    # The users table should be empty after reset
    Then "db" table "users" should be empty

  Scenario: Execute custom SQL
    Given I execute SQL on "db":
      """
      INSERT INTO users (name, email) VALUES ('Charlie', 'charlie@test.com');
      """
    Then "db" table "users" should have "1" rows
