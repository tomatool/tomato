@postgres
Feature: PostgreSQL Handler
  Test all PostgreSQL handler steps
  Note: Tables are automatically truncated before each scenario via reset

  Scenario: Insert and query data
    Given "app-db" table "users" has values:
      | id | name    | email              |
      | 1  | Charlie | charlie@test.com   |
      | 2  | Diana   | diana@test.com     |
    Then "app-db" table "users" has "2" rows
    And "app-db" table "users" contains:
      | id | name    |
      | 1  | Charlie |
      | 2  | Diana   |

  Scenario: Check empty table
    Then "app-db" table "users" is empty
    And "app-db" table "users" has "0" rows

  Scenario: Execute raw SQL
    Given "app-db" executes:
      """
      INSERT INTO users (name, email) VALUES ('Eve', 'eve@test.com')
      """
    Then "app-db" table "users" has "1" rows
    And "app-db" table "users" contains:
      | name |
      | Eve  |

  Scenario: Database state persists during scenario
    Given "app-db" table "users" has values:
      | id | name  | email           |
      | 10 | Frank | frank@test.com  |
    When "app-db" executes:
      """
      UPDATE users SET name = 'Franklin' WHERE id = 10
      """
    Then "app-db" table "users" contains:
      | id | name     |
      | 10 | Franklin |

  Scenario: Query returns exact result
    Given "app-db" table "users" has values:
      | id | name  | email           |
      | 1  | Alice | alice@test.com  |
      | 2  | Bob   | bob@test.com    |
    Then "app-db" query "SELECT name, email FROM users ORDER BY id" returns:
      | name  | email           |
      | Alice | alice@test.com  |
      | Bob   | bob@test.com    |

  Scenario: Query result contains expected rows
    Given "app-db" table "users" has values:
      | id | name    | email              |
      | 1  | Alice   | alice@test.com     |
      | 2  | Bob     | bob@test.com       |
      | 3  | Charlie | charlie@test.com   |
    Then "app-db" query result of "SELECT name, email FROM users ORDER BY id" contains:
      | name  | email          |
      | Alice | alice@test.com |
      | Bob   | bob@test.com   |

  Scenario: Query returns single scalar value
    Given "app-db" table "users" has values:
      | id | name  | email           |
      | 1  | Alice | alice@test.com  |
      | 2  | Bob   | bob@test.com    |
    Then "app-db" query "SELECT count(*) as cnt FROM users" returns:
      | cnt |
      | 2   |

  Scenario: Seed from a SQL file
    When "app-db" executes file "tests/testdata/sql/seed_users.sql"
    Then "app-db" table "users" has "2" rows
    And "app-db" table "users" contains:
      | id | name  |
      | 10 | Frank |
      | 11 | Grace |

  Scenario: Clear one table within a scenario
    Given "app-db" table "users" has values:
      | id | name | email          |
      | 1  | Ada  | ada@test.com   |
    When "app-db" clears table "users"
    Then "app-db" table "users" is empty

  Scenario: Clear several tables from a list
    Given "app-db" executes:
      """
      CREATE TABLE IF NOT EXISTS audit_log (id int PRIMARY KEY, entry text);
      INSERT INTO audit_log (id, entry) VALUES (1, 'created');
      """
    And "app-db" table "users" has values:
      | id | name | email        |
      | 2  | Bob  | bob@test.com |
    When "app-db" clears tables:
      | users     |
      | audit_log |
    Then "app-db" table "users" is empty
    And "app-db" table "audit_log" is empty

  Scenario: Wait for rows a query returns
    Given "app-db" executes:
      """
      INSERT INTO users (name, email) VALUES ('Async', 'async@test.com')
      """
    Then "app-db" query "SELECT name FROM users WHERE email = 'async@test.com'" returns within "5s":
      | name  |
      | Async |
