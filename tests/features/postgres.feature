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

  Scenario: Query returns exact result
    Given "db" table "users" has values:
      | id | name  | email           |
      | 1  | Alice | alice@test.com  |
      | 2  | Bob   | bob@test.com    |
    Then "db" query "SELECT name, email FROM users ORDER BY id" returns:
      | name  | email           |
      | Alice | alice@test.com  |
      | Bob   | bob@test.com    |

  Scenario: Query result contains expected rows
    Given "db" table "users" has values:
      | id | name    | email              |
      | 1  | Alice   | alice@test.com     |
      | 2  | Bob     | bob@test.com       |
      | 3  | Charlie | charlie@test.com   |
    Then "db" query result of "SELECT name, email FROM users ORDER BY id" contains:
      | name  | email          |
      | Alice | alice@test.com |
      | Bob   | bob@test.com   |

  Scenario: Query returns single scalar value
    Given "db" table "users" has values:
      | id | name  | email           |
      | 1  | Alice | alice@test.com  |
      | 2  | Bob   | bob@test.com    |
    Then "db" query "SELECT count(*) as cnt FROM users" returns:
      | cnt |
      | 2   |
