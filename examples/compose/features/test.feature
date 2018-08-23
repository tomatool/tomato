Feature: check status endpoint

  Scenario: When everything is fine
    Given "app-client" send request to "GET /status"
    Then "app-client" response code should be 501
