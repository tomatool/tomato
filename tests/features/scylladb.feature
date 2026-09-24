@scylladb
Feature: ScyllaDB Handler
  Test all ScyllaDB handler steps
  Note: tables in the keyspace are truncated before each scenario, except
  `countries`, which is excluded from reset

  Scenario: Insert and query data
    Given "scylla" table "users" has values:
      | id | name    | email            | active | tags        |
      | 1  | Charlie | charlie@test.com | true   | ["a", "b"]  |
      | 2  | Diana   | diana@test.com   | false  | []          |
    Then "scylla" table "users" has "2" rows
    And "scylla" table "users" contains:
      | id | name    | active |
      | 2  | Diana   | false  |
      | 1  | Charlie | true   |

  Scenario: Tables are reset between scenarios
    Then "scylla" table "users" is empty
    And "scylla" table "users" has "0" rows

  Scenario: Excluded tables keep their reference data
    Then "scylla" table "countries" has "2" rows
    And "scylla" query "SELECT name FROM countries WHERE code = 'DE'" returns:
      | name    |
      | Germany |

  Scenario: Execute raw CQL
    Given "scylla" executes:
      """
      INSERT INTO users (id, name, email) VALUES (3, 'Eve', 'eve@test.com');
      INSERT INTO users (id, name, email) VALUES (4, 'Frank; Jr', 'frank@test.com');
      """
    Then "scylla" table "users" has "2" rows
    And "scylla" query result of "SELECT name FROM users" contains:
      | name      |
      | Frank; Jr |
      | Eve       |

  Scenario: Update within a scenario
    Given "scylla" table "users" has values:
      | id | name  | email          |
      | 10 | Frank | frank@test.com |
    When "scylla" executes:
      """
      UPDATE users SET name = 'Franklin' WHERE id = 10
      """
    Then "scylla" query "SELECT id, name FROM users WHERE id = 10" returns:
      | id | name     |
      | 10 | Franklin |

  Scenario: Count with an alias
    Given "scylla" table "users" has values:
      | id | name  | email          |
      | 1  | Alice | alice@test.com |
      | 2  | Bob   | bob@test.com   |
    Then "scylla" query "SELECT count(*) AS cnt FROM users" returns:
      | cnt |
      | 2   |

  Scenario: Clear a table
    Given "scylla" table "users" has values:
      | id | name  | email          |
      | 1  | Alice | alice@test.com |
    When "scylla" clears table "users"
    Then "scylla" table "users" is empty
