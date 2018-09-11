Feature: database features example

  Scenario: Set and compare table using PostgreSQL database driver
    Then "tomato-psql" table "customers" should look like
        | customer_id | name    |
    Given set "tomato-psql" table "customers" list of content
        | name    | country |
        | cembri  | us      |
    Then "tomato-psql" table "customers" should look like
        | customer_id | name    |
        | 1           | cembri  |

  Scenario: Set and compare table using MySQL database driver, and test empty functionality
    Then "tomato-mysql" table "customers" should look like
        | customer_id | name    |
    Given set "tomato-mysql" table "customers" list of content
        | name    | country |
        | cembri  | us      |
    Then "tomato-mysql" table "customers" should look like
        | customer_id | name    |
        | 1           | cembri  |
    Then "tomato-mysql" table "customers" should look like
        | customer_id | name    |
    Given set "tomato-mysql" table "customers" list of content
        | name    | country |
        | cembri  | us      |
        | cembre  | id      |
        | cembra  | de      |
    Then "tomato-mysql" table "customers" should look like
        | customer_id | name    | country |
        | 1           | cembri  | us      |
        | 2           | cembre  | id      |
        | 3           | cembra  | de      |
    Then "tomato-mysql" table "customers" should look like
        | customer_id | name    |
