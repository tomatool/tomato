Feature: database features example

  Scenario: Set and compare table
    Given set "my-awesome-postgres-db" table "customers" list of content
        | name    | country |
        | cembri  | us      |
    Then "my-awesome-postgres-db" table "customers" should look like
        | customer_id | name    |
        | 1           | cembri  |
