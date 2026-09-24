Feature: Orders
  Every scenario starts from an empty orders table, so each one can use the
  same ids and still be independent of the others.

  Background:
    Given "kafka" consumes from "orders"

  Scenario: Creating an order stores it and emits OrderCreated
    When "api" sends "POST" to "/orders" with json:
      """
      {"id": "order-1", "amount": 4200, "currency": "EUR"}
      """
    Then "api" response status is "201"
    And "api" response json contains:
      """
      {"id": "order-1", "status": "CREATED"}
      """
    And "db" table "orders" contains:
      | id      | amount | currency | status  |
      | order-1 | 4200   | EUR      | CREATED |
    And "kafka" receives avro from "orders" within "15s":
      """
      {"id": "order-1", "amount": 4200, "currency": "EUR"}
      """

  Scenario: The same id works again because state is fresh
    When "api" sends "POST" to "/orders" with json:
      """
      {"id": "order-1", "amount": 99, "currency": "USD"}
      """
    Then "api" response status is "201"
    And "db" table "orders" has "1" rows

  Scenario: A duplicate id within one scenario is rejected
    Given "db" table "orders" has values:
      | id      | amount | currency | status  |
      | order-1 | 10     | EUR      | CREATED |
    When "api" sends "POST" to "/orders" with json:
      """
      {"id": "order-1", "amount": 10, "currency": "EUR"}
      """
    Then "api" response status is "409"

  Scenario: Currencies seeded by the migration survive every reset
    When "api" sends "POST" to "/orders" with json:
      """
      {"id": "order-2", "amount": 10, "currency": "GBP"}
      """
    Then "api" response status is "422"
    And "db" table "currencies" has "2" rows
