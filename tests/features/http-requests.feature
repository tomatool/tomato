@http
Feature: HTTP requests and responses in depth
  Request bodies, headers, cookies, exact bodies, JSON matchers and variables

  Scenario: Set several headers from a table
    Given "api" headers are:
      | header      | value      |
      | X-Team      | tomato     |
      | X-Request-Id | req-123   |
    When "api" sends "GET" to "/echo"
    Then "api" response json "headers.X-Team[0]" is "tomato"
    And "api" response json "headers.X-Request-Id[0]" is "req-123"

  Scenario: Send a request cookie
    Given "api" cookie "session" is "abc-123"
    When "api" sends "GET" to "/echo"
    Then "api" response json "cookies.session" is "abc-123"

  Scenario: Send a raw body set beforehand
    Given "api" body is:
      """
      plain text payload
      """
    When "api" sends "POST" to "/echo"
    Then "api" response json "raw" is "plain text payload"
    And "api" response json "body" does not exist

  Scenario: Send a raw body inline
    When "api" sends "POST" to "/echo" with body:
      """
      {"inline": true}
      """
    Then "api" response body contains "inline"
    And "api" response json "body.inline" is "true"

  Scenario: Send a form-encoded body
    Given "api" form body is:
      | field    | value      |
      | username | ada        |
      | remember | yes        |
    When "api" sends "POST" to "/echo"
    Then "api" response json "form.username" is "ada"
    And "api" response json "form.remember" is "yes"
    And "api" response json "headers.Content-Type[0]" is "application/x-www-form-urlencoded"

  Scenario: Exact plain-text body and header value
    When "api" sends "GET" to "/text"
    Then "api" response status is "200"
    And "api" response header "X-Tomato" is "fresh"
    And "api" response body is:
      """
      Hello from the "tomato" test app
      line two
      """
    And "api" response body contains:
      """
      the "tomato" test
      """
    And "api" response body does not contain:
      """
      the "potato" test
      """

  Scenario: Empty response body
    When "api" sends "GET" to "/empty"
    Then "api" response status is "204"
    And "api" response body is empty

  Scenario: Response cookies and saved values
    When "api" sends "POST" to "/login"
    Then "api" response status is "201"
    And "api" response cookie "session" exists
    And "api" response cookie "session" is "sess-42"
    And "api" response cookie "session" saved as "{{session_cookie}}"
    And "api" response header "Location" saved as "{{session_url}}"
    Given "api" cookie "session" is "{{session_cookie}}"
    When "api" sends "GET" to "{{session_url}}"
    Then "api" response json "cookies.session" is "sess-42"
    And "api" response json "query.session[0]" is "sess-42"

  Scenario: A cleared cookie has an empty value
    When "api" sends "POST" to "/logout"
    Then "api" response cookie "session" is ""

  Scenario: Exact and partial JSON matching with matchers
    When "api" sends "GET" to "/profile"
    Then "api" response json matches:
      """
      {
        "id": "@regex:^[0-9a-f-]{36}$",
        "email": "@contains:@example.com",
        "name": "Ada",
        "created_at": "@string",
        "tags": "@len:2",
        "verified": "@boolean",
        "age": "@gte:18"
      }
      """
    And "api" response json contains:
      """
      {"name": "Ada", "verified": true}
      """
    And "api" response json "deleted_at" does not exist

  Scenario: Typed JSON field checks
    When "api" sends "GET" to "/profile"
    Then "api" response json "id" is uuid
    And "api" response json "email" is email
    And "api" response json "created_at" is iso-timestamp
    And "api" response json "name" matches pattern "^[A-Z][a-z]+$"

  Scenario: Chain requests with a saved JSON value
    When "api" sends "POST" to "/users" with json:
      """
      {"name": "Grace", "email": "grace@test.com"}
      """
    Then "api" response status is "201"
    And "api" response json "id" saved as "{{grace_id}}"
    When "api" sends "GET" to "/users/{{grace_id}}"
    Then "api" response status is "200"
    And "api" response json "name" is "Grace"
