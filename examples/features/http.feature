Feature: http feature example

  Scenario: Set and compare table
    Given set "mockserver" response code to 202 and response body
        """
          {
                "status":"OK"
          }
        """
    Given set "mockserver" with path "/status?fail" response code to 500 and response body
        """
          {
                "status":"NOT OK"
          }
        """
    Given "httpcli" send request to "GET /status"
    Then "httpcli" response code should be 202
    Then "httpcli" response body should be
        """
            {
                "status":"OK"
            }
        """
    Given "httpcli" send request to "GET /status?fail"
    Then "httpcli" response code should be 500
    Then "httpcli" response body should be
        """
            {
                "status":"NOT OK"
            }
        """
