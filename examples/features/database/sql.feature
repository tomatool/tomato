Feature: database features example

  Scenario: Set and compare table using PostgreSQL database driver
    Given "tomato-psql" table "tblCustomers" should look like
        | customer_id | name    |
    Given set "tomato-psql" table "tblCustomers" list of content
        | name    | country |
        | cembri  | us      |
    Then "tomato-psql" table "tblCustomers" should look like
        | customer_id | country | name   |
        | 1           | us      | cembri |
    Given set "tomato-psql" table "tblCustomers" list of content
        | name    | country |
        | cembre  | id      |
    Then "tomato-psql" table "tblCustomers" should look like
        | customer_id | country | name   |
        | 1           | us      | cembri |
        | 2           | id      | cembre |

  Scenario: Test UUID value in postgres
    Given set "tomato-psql" table "tblCustomers" list of content
        | name                                 | country                             |
        | a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11 | id |
    Then "tomato-psql" table "tblCustomers" should look like
        | customer_id | country |
        | 1           | id      |
    Then "tomato-psql" table "tblCustomers" should look like
        | customer_id | country | name                                 |
        | 1           | id      | a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11 |

  Scenario: Table should always empty on each starting scenario
    Given "tomato-psql" table "tblCustomers" should look like
      | customer_id | name    |
    Then "tomato-mysql" table "customers" should look like
      | customer_id | name    |

  Scenario: Set and compare table using MySQL database driver, and test empty functionality
    Given "tomato-mysql" table "customers" should look like
      | customer_id | name    |
    Given set "tomato-mysql" table "customers" list of content
      | name    | country | active |
      | cembri  | us      | bit::1      |
    Then "tomato-mysql" table "customers" should look like
      | customer_id | country | name   | active | 
      | 1           | us      | cembri | 1      |
    Given set "tomato-mysql" table "customers" list of content
      | name    | country | active |
      | cembre  | id      | bit::0      |
    Then "tomato-mysql" table "customers" should look like
      | customer_id | country | name   | active |
      | 1           | us      | cembri | 1      |
      | 2           | id      | cembre | 0      |
