Feature: Payments
  A PaymentReceived event, published by tomato as Avro, is consumed by the
  app's @KafkaListener, which marks the order paid and emits OrderPaid.

  Background:
    Given "kafka" registers schema for subject "payments-value" from file "src/main/resources/avro/payment_received.avsc"
    And "kafka" consumes from "orders-paid"

  Scenario: A payment marks the order paid
    Given "db" table "orders" has values:
      | id      | amount | currency | status  |
      | order-7 | 4200   | EUR      | CREATED |
    When "kafka" publishes avro to "payments" with key "order-7":
      """
      {"paymentId": "pay-1", "orderId": "order-7", "amount": 4200}
      """
    Then "kafka" receives avro from "orders-paid" within "20s":
      """
      {"id": "order-7", "paymentId": "pay-1"}
      """
    And "db" table "orders" contains:
      | id      | status |
      | order-7 | PAID   |
    When "api" sends "GET" to "/orders/order-7"
    Then "api" response json contains:
      """
      {"status": "PAID"}
      """

  Scenario: A payment for an unknown order changes nothing
    When "kafka" publishes avro to "payments":
      """
      {"paymentId": "pay-2", "orderId": "missing", "amount": 1}
      """
    And "api" sends "GET" to "/orders/missing"
    Then "api" response status is "404"
    And "db" table "orders" is empty
