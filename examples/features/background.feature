Feature: test that background actually doing things, and not get reset by resource reset function

    Background:
        Given set "tomato-wiremock" with path "/status" response code to 202 and response body
            """
          {
                "status":"OK"
          }
            """

    Scenario: First test
        Given "tomato-http-client" send request to "GET /status"
        Then "tomato-http-client" response code should be 202

    Scenario: Second test
        Given "tomato-http-client" send request to "GET /status"
        Then "tomato-http-client" response code should be 202

    Scenario: Third test
        Given "tomato-http-client" send request to "GET /status"
        Then "tomato-http-client" response code should be 202

    Scenario: Fourth test
        Given "tomato-http-client" send request to "GET /status"
        Then "tomato-http-client" response code should be 202

    Scenario: Fifth test
        Given "tomato-http-client" send request to "GET /status"
        Then "tomato-http-client" response code should be 202
