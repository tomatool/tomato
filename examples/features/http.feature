Feature: http feature example

    Scenario: Set and compare http-wiremock responses
        Given set "tomato-wiremock" with path "/example" response code to 202 and response body
            """
          {
                "example":"word"
          }
            """
        Given set "tomato-wiremock" with path "/example?fail" response code to 500 and response body
            """
          {
                "example":"not word",
                "timestamp": "2018-01-05 00:39:33"
          }
            """
        Given "tomato-http-client" send request to "GET /example"
        Then "tomato-http-client" response code should be 202
        Then "tomato-http-client" response body should contain
            """
          {
                "example":"word"
          }
            """
        Given "tomato-http-client" send request to "GET /example?fail"
        Then "tomato-http-client" response code should be 500
        Then "tomato-http-client" response body should contain
            """
          {
                "example":"not word",
                "timestamp": "2018-01-05 00:39:33"
          }
            """