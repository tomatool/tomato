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
        Given set "tomato-wiremock" with method "POST" and path "/example-post" response code to 201 and response body
          """
          {
                "example":"word created"
          }
          """
        Given set "tomato-wiremock" with method "PUT" and path "/example-put" response code to 204
        Given "tomato-wiremock" with path "GET /example" request count should be 0
        Given "tomato-http-client" send request to "GET /example"
        Given "tomato-wiremock" with path "GET /example" request count should be 1
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
        Given "tomato-http-client" send request to "POST /example-post"
        Then "tomato-http-client" response code should be 201
        Then "tomato-http-client" response body should contain
          """
          {
                 "example":"word created"
          }
          """
        Given "tomato-http-client" send request to "PUT /example-put"
        Then "tomato-http-client" response code should be 204
