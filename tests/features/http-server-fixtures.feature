Feature: HTTP Server Fixtures

  Scenario: Auto-load fixtures from configuration
    # Fixtures are auto-loaded from tomato.yml config (fixturesPath + autoLoad: true)
    When "client" sends "GET" to "http://localhost:9997/api/user"
    Then "client" response status code should be "200"
    And "client" response body should contain "octocat"

  Scenario: Load fixtures manually and verify simple stub
    Given "mock" stub "GET" "/placeholder" returns "200"
    And "mock" loads fixtures from "tests/fixtures/github-api"
    When "client" sends "GET" to "http://localhost:9999/api/user"
    Then "client" response status code should be "200"
    And "client" response body should contain "octocat"
    And "mock" received "GET" "/api/user"

  Scenario: Load fixtures with body from file
    Given "mock" stub "GET" "/placeholder" returns "200"
    And "mock" loads fixtures from "tests/fixtures/github-api"
    When "client" sends "GET" to "http://localhost:9999/api/user/repos"
    Then "client" response status code should be "200"
    And "client" response body should contain "Hello-World"
    And "client" response body should contain "tomato"

  Scenario: Load fixtures with path pattern (regex)
    Given "mock" stub "GET" "/placeholder" returns "200"
    And "mock" loads fixtures from "tests/fixtures/github-api"
    When "client" sends "GET" to "http://localhost:9999/api/repos/octocat/Hello-World"
    Then "client" response status code should be "200"
    And "client" response body should contain "Hello-World"

  Scenario: Fixture with header condition - authenticated
    Given "mock" stub "GET" "/placeholder" returns "200"
    And "mock" loads fixtures from "tests/fixtures/github-api"
    When "client" sends "GET" to "http://localhost:9999/api/user/emails" with headers:
      | header        | value            |
      | Authorization | Bearer fake-token |
    Then "client" response status code should be "200"
    And "client" response body should contain "octocat@github.com"

  Scenario: Fixture with header condition - unauthenticated
    Given "mock" stub "GET" "/placeholder" returns "200"
    And "mock" loads fixtures from "tests/fixtures/github-api"
    When "client" sends "GET" to "http://localhost:9999/api/user/emails"
    Then "client" response status code should be "401"
    And "client" response body should contain "Requires authentication"

  Scenario: Fixture with query parameter condition
    Given "mock" stub "GET" "/placeholder" returns "200"
    And "mock" loads fixtures from "tests/fixtures/github-api"
    When "client" sends "GET" to "http://localhost:9999/api/search/repositories?page=1&per_page=10"
    Then "client" response status code should be "200"
    And "client" response body should contain "Spoon-Knife"

  Scenario: Fixture with body condition (JSONPath)
    Given "mock" stub "GET" "/placeholder" returns "200"
    And "mock" loads fixtures from "tests/fixtures/github-api"
    When "client" sends "POST" to "http://localhost:9999/api/repos/owner/repo/issues" with json:
      """
      {
        "title": "Found a bug",
        "body": "Something is broken",
        "labels": ["bug"]
      }
      """
    Then "client" response status code should be "201"
    And "client" response body should contain "Bug issue"

  Scenario: Dynamic stub overrides fixture stub
    Given "mock" stub "GET" "/placeholder" returns "200"
    And "mock" loads fixtures from "tests/fixtures/github-api"
    And "mock" stub "GET" "/api/user" returns "200" with json:
      """
      {"id": 999, "login": "dynamicuser", "name": "Dynamic Override"}
      """
    When "client" sends "GET" to "http://localhost:9999/api/user"
    Then "client" response status code should be "200"
    And "client" response body should contain "dynamicuser"
    And "client" response body should not contain "octocat"

  Scenario: Fixtures persist across scenarios (Reset behavior)
    Given "mock" stub "GET" "/placeholder" returns "200"
    And "mock" loads fixtures from "tests/fixtures/github-api"
    When "client" sends "GET" to "http://localhost:9999/api/user"
    Then "client" response status code should be "200"
    And "client" response body should contain "octocat"

  Scenario: Verify fixtures still loaded after reset
    When "client" sends "GET" to "http://localhost:9999/api/user/repos"
    Then "client" response status code should be "200"
    And "client" response body should contain "Hello-World"

  Scenario: Most specific fixture wins when multiple match
    Given "mock" stub "GET" "/placeholder" returns "200"
    And "mock" loads fixtures from "tests/fixtures/github-api"
    And "client" header "X-GitHub-Event" is "push"
    And "client" json body is:
      """
      {"ref": "refs/heads/main", "commits": []}
      """
    When "client" sends "POST" to "http://localhost:9999/api/webhook"
    Then "client" response status code should be "202"
    And "client" response body should contain "accepted"

  Scenario: No fixture match returns 404
    Given "mock" stub "GET" "/placeholder" returns "200"
    And "mock" loads fixtures from "tests/fixtures/github-api"
    When "client" sends "GET" to "http://localhost:9999/api/nonexistent"
    Then "client" response status code should be "404"
    And "client" response body should contain "No stub found"
