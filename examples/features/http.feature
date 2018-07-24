Feature: http feature example

  Scenario: Set and compare table
    Given set "tomato-http-server" response code to 202 and response body
        """
          {
                "status":"OK"
          }
        """
    Given set "tomato-http-server" with path "/status?fail" response code to 500 and response body
        """
          {
                "status":"NOT OK",
                "timestamp": "2018-01-05 00:39:33"
          }
        """
    Given "tomato-http-client" send request to "GET /status"
    Then "tomato-http-client" response code should be 202
    Then "tomato-http-client" response body should be
        """
            {
                "status":"OK"
            }
        """
    Given "tomato-http-client" send request to "GET /status?fail"
    Then "tomato-http-client" response code should be 500
    Then "tomato-http-client" response body should be
        """
            {
                "status":"NOT OK",
                "timestamp": "*"
            }
        """
