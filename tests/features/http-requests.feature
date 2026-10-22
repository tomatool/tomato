@http
Feature: HTTP requests and responses in depth
  Request bodies, headers, cookies, exact bodies, JSON matchers and variables

  Scenario: Set several headers from a table
    Given "app-client" headers are:
      | header      | value      |
      | X-Team      | tomato     |
      | X-Request-Id | req-123   |
    When "app-client" sends "GET" to "/echo"
    Then "app-client" response json "headers.X-Team[0]" is "tomato"
    And "app-client" response json "headers.X-Request-Id[0]" is "req-123"

  Scenario: Send a request cookie
    Given "app-client" cookie "session" is "abc-123"
    When "app-client" sends "GET" to "/echo"
    Then "app-client" response json "cookies.session" is "abc-123"

  Scenario: Send a raw body set beforehand
    Given "app-client" body is:
      """
      plain text payload
      """
    When "app-client" sends "POST" to "/echo"
    Then "app-client" response json "raw" is "plain text payload"
    And "app-client" response json "body" does not exist

  Scenario: Send a raw body inline
    When "app-client" sends "POST" to "/echo" with body:
      """
      {"inline": true}
      """
    Then "app-client" response body contains "inline"
    And "app-client" response json "body.inline" is "true"

  Scenario: Send a form-encoded body
    Given "app-client" form body is:
      | field    | value      |
      | username | ada        |
      | remember | yes        |
    When "app-client" sends "POST" to "/echo"
    Then "app-client" response json "form.username" is "ada"
    And "app-client" response json "form.remember" is "yes"
    And "app-client" response json "headers.Content-Type[0]" is "application/x-www-form-urlencoded"

  Scenario: A form-encoded body substitutes saved variables
    Given "app-client" sends "POST" to "/login"
    And "app-client" response header "Location" query param "session" saved as "{{session_id}}"
    And "app-client" form body is:
      | field   | value          |
      | session | {{session_id}} |
    When "app-client" sends "POST" to "/echo"
    Then "app-client" response json "form.session" is "sess-42"

  Scenario: Exact plain-text body and header value
    When "app-client" sends "GET" to "/text"
    Then "app-client" response status is "200"
    And "app-client" response header "X-Tomato" is "fresh"
    And "app-client" response body is:
      """
      Hello from the "tomato" test app
      line two
      """
    And "app-client" response body contains:
      """
      the "tomato" test
      """
    And "app-client" response body does not contain:
      """
      the "potato" test
      """

  Scenario: Empty response body
    When "app-client" sends "GET" to "/empty"
    Then "app-client" response status is "204"
    And "app-client" response body is empty

  Scenario: Response cookies and saved values
    When "app-client" sends "POST" to "/login"
    Then "app-client" response status is "201"
    And "app-client" response cookie "session" exists
    And "app-client" response cookie "session" is "sess-42"
    And "app-client" response cookie "session" saved as "{{session_cookie}}"
    And "app-client" response header "Location" saved as "{{session_url}}"
    Given "app-client" cookie "session" is "{{session_cookie}}"
    When "app-client" sends "GET" to "{{session_url}}"
    Then "app-client" response json "cookies.session" is "sess-42"
    And "app-client" response json "query.session[0]" is "sess-42"

  Scenario: A query param of the URL in a response header is saved
    When "app-client" sends "POST" to "/login"
    Then "app-client" response header "Location" query param "session" saved as "{{session_id}}"
    When "app-client" sends "GET" to "/echo?id={{session_id}}"
    Then "app-client" response json "query.id[0]" is "sess-42"

  Scenario: A cleared cookie has an empty value
    When "app-client" sends "POST" to "/logout"
    Then "app-client" response cookie "session" is ""

  Scenario: Exact and partial JSON matching with matchers
    When "app-client" sends "GET" to "/profile"
    Then "app-client" response json matches:
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
    And "app-client" response json contains:
      """
      {"name": "Ada", "verified": true}
      """
    And "app-client" response json "deleted_at" does not exist

  Scenario: Typed JSON field checks
    When "app-client" sends "GET" to "/profile"
    Then "app-client" response json "id" is uuid
    And "app-client" response json "email" is email
    And "app-client" response json "created_at" is iso-timestamp
    And "app-client" response json "name" matches pattern "^[A-Z][a-z]+$"

  Scenario: Chain requests with a saved JSON value
    When "app-client" sends "POST" to "/users" with json:
      """
      {"name": "Grace", "email": "grace@test.com"}
      """
    Then "app-client" response status is "201"
    And "app-client" response json "id" saved as "{{grace_id}}"
    When "app-client" sends "GET" to "/users/{{grace_id}}"
    Then "app-client" response status is "200"
    And "app-client" response json "name" is "Grace"

  Scenario: Assert a response against a saved value
    When "app-client" sends "POST" to "/users" with json:
      """
      {"name": "Ada", "email": "ada@test.com"}
      """
    Then "app-client" response status is "201"
    And "app-client" response json "id" saved as "{{ada_id}}"
    When "app-client" sends "GET" to "/users/{{ada_id}}"
    Then "app-client" response json "id" is "{{ada_id}}"
    And "app-client" response json contains:
      """
      {"id": {{ada_id}}, "name": "Ada"}
      """
    And "app-client" response body contains "{{ada_id}}"

  Scenario: Concurrent identical creates make one row
    When "app-client" sends "5" concurrent "POST" to "/users" with json:
      """
      {"name": "Race", "email": "race@test.com"}
      """
    Then "app-client" "1" of the concurrent responses have status "201"
    And "app-client" "4" of the concurrent responses have status "500"
    And "app-client" "1" of the concurrent responses have json "name" is "Race"
    And "app-client" "1" of the concurrent responses contain json:
      """
      {"email": "race@test.com", "id": "@number"}
      """
    And "app-db" table "users" has "1" rows

  Scenario: Concurrent requests without a body
    Given "app-client" header "X-Custom-Header" is "concurrent"
    When "app-client" sends "3" concurrent "GET" to "/echo"
    Then "app-client" "3" of the concurrent responses have status "200"
