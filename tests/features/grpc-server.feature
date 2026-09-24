@grpc-server
Feature: gRPC Server Handler
  Stub a gRPC dependency and assert on the calls it receives.
  "payments" is the stub server; "payments-client" calls it the way an app would.

  Scenario: The stub server exposes the services from its proto files
    Then "payments-client" exposes service "payments.v1.Payments"
    And "payments-client" service "payments.v1.Payments" has method "Authorize"

  Scenario: Stub a unary method and assert on the request
    Given "payments" stub "payments.v1.Payments/Authorize" returns:
      """
      {"approved": true, "authorizationId": "auth-1", "authorizedAt": "2026-09-24T10:00:00Z"}
      """
    And "payments-client" metadata "authorization" is "Bearer test-token"
    When "payments-client" calls "payments.v1.Payments/Authorize" with:
      """
      {"orderId": "order-1", "amount": 4200, "currency": "EUR"}
      """
    Then "payments-client" call succeeds
    And "payments-client" response json "approved" is "true"
    And "payments-client" response json "authorizationId" is "auth-1"
    And "payments" received "payments.v1.Payments/Authorize"
    And "payments" received "payments.v1.Payments/Authorize" "1" times
    And "payments" received "payments.v1.Payments/Authorize" with json:
      """
      {"orderId": "order-1", "amount": "4200"}
      """
    And "payments" received metadata "authorization" containing "Bearer"
    And "payments" did not receive "payments.v1.Payments/Refund"
    And "payments" received "1" requests

  Scenario: Stub a failure status
    Given "payments" stub "payments.v1.Payments/Refund" returns status "FAILED_PRECONDITION" with message "already refunded"
    When "payments-client" calls "payments.v1.Payments/Refund" with:
      """
      {"authorizationId": "auth-1"}
      """
    Then "payments-client" call fails
    And "payments-client" response status is "FAILED_PRECONDITION"
    And "payments-client" response error contains "already refunded"

  Scenario: Stub a status without a message
    Given "payments" stub "payments.v1.Payments/Authorize" returns status "UNAVAILABLE"
    When "payments-client" calls "payments.v1.Payments/Authorize"
    Then "payments-client" response status is "UNAVAILABLE"

  Scenario: Unstubbed methods fail with UNIMPLEMENTED
    When "payments-client" calls "payments.v1.Payments/Refund"
    Then "payments-client" response status is "UNIMPLEMENTED"
    And "payments-client" response error contains "no stub"

  Scenario: Stubs and recorded calls reset between scenarios
    Then "payments" received "0" requests
    And "payments" did not receive "payments.v1.Payments/Authorize"

  Scenario: Store the server address
    Given "payments" address is stored in "PAYMENTS_ADDR"
    Given "payments" stub "payments.v1.Payments/Authorize" returns:
      """
      {"authorizationId": "{{PAYMENTS_ADDR}}"}
      """
    When "payments-client" calls "payments.v1.Payments/Authorize"
    Then "payments-client" response json "authorizationId" is "localhost:50061"
