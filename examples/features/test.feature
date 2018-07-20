Feature: test feature

  Scenario: Delete user feature
    Given set "service-a" response code to 202 and response body
      """
        {"status":"OK"}
      """
    Given "http" send request to "GET /" with body
      """
      """
    Then "http" response code should be 202
    Then "http" response body should be
      """
        {"status":"OK "}
      """
