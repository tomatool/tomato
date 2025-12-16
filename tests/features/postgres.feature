@postgres
Feature: PostgreSQL Handler
  Test all PostgreSQL handler steps
  Note: Tables are automatically truncated before each scenario via reset

  Scenario: Insert and query data
    Given "db" table "users" has values:
      | id | name    | email              |
      | 1  | Charlie | charlie@test.com   |
      | 2  | Diana   | diana@test.com     |
    Then "db" table "users" has "2" rows
    And "db" table "users" contains:
      | id | name    |
      | 1  | Charlie |
      | 2  | Diana   |

  Scenario: Check empty table
    Then "db" table "users" is empty
    And "db" table "users" has "0" rows

  Scenario: Execute raw SQL
    Given "db" executes:
      """
      INSERT INTO users (name, email) VALUES ('Eve', 'eve@test.com')
      """
    Then "db" table "users" has "1" rows
    And "db" table "users" contains:
      | name |
      | Eve  |

  Scenario: Database state persists during scenario
    Given "db" table "users" has values:
      | id | name  | email           |
      | 10 | Frank | frank@test.com  |
    When "db" executes:
      """
      UPDATE users SET name = 'Franklin' WHERE id = 10
      """
    Then "db" table "users" contains:
      | id | name     |
      | 10 | Franklin |
