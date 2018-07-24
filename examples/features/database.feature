Feature: database features example

  Scenario: Set and compare table
    Given set "tomato-psql" table "customers" list of content
        | name    | country |
        | cembri  | us      |
    Then "tomato-psql" table "customers" should look like
        | customer_id | name    |
        | 1           | cembri  |
