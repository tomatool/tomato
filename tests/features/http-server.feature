@http-server
Feature: HTTP Server Handler
  Test all HTTP server handler steps for stubbing external services

  # Stub Setup - Status only
  Scenario: Stub returns status code
    Given "upstream-stub" stub "GET" "/health" returns "200"
    When "app-client" sends "GET" to "http://localhost:9999/health"
    Then "app-client" response status is "200"

  # Stub Setup - With body
  Scenario: Stub returns body
    Given "upstream-stub" stub "GET" "/message" returns "200" with body:
      """
      Hello World
      """
    When "app-client" sends "GET" to "http://localhost:9999/message"
    Then "app-client" response status is "200"
    And "app-client" response body contains "Hello World"

  # Stub Setup - With JSON
  Scenario: Stub returns JSON
    Given "upstream-stub" stub "GET" "/users" returns "200" with json:
      """
      [{"id": 1, "name": "Alice"}, {"id": 2, "name": "Bob"}]
      """
    When "app-client" sends "GET" to "http://localhost:9999/users"
    Then "app-client" response status is "200"
    And "app-client" response header "Content-Type" contains "application/json"
    And "app-client" response json "[0].name" is "Alice"

  # Stub Setup - With headers
  Scenario: Stub returns with custom headers
    Given "upstream-stub" stub "GET" "/custom" returns "200" with headers:
      | header       | value           |
      | X-Custom     | custom-value    |
      | X-Request-Id | req-123         |
    When "app-client" sends "GET" to "http://localhost:9999/custom"
    Then "app-client" response status is "200"
    And "app-client" response header "X-Custom" contains "custom-value"
    And "app-client" response header "X-Request-Id" contains "req-123"

  # Stub Setup - Different methods
  Scenario: Stub POST endpoint
    Given "upstream-stub" stub "POST" "/items" returns "201" with json:
      """
      {"id": 100, "created": true}
      """
    Given "app-client" header "Content-Type" is "application/json"
    When "app-client" sends "POST" to "http://localhost:9999/items" with json:
      """
      {"name": "New Item"}
      """
    Then "app-client" response status is "201"
    And "app-client" response json "created" is "true"

  # Verification - Received request
  Scenario: Verify request was received
    Given "upstream-stub" stub "GET" "/check" returns "200"
    When "app-client" sends "GET" to "http://localhost:9999/check"
    Then "upstream-stub" received "GET" "/check"

  # Verification - Received N times
  Scenario: Verify request count
    Given "upstream-stub" stub "GET" "/counter" returns "200"
    When "app-client" sends "GET" to "http://localhost:9999/counter"
    And "app-client" sends "GET" to "http://localhost:9999/counter"
    And "app-client" sends "GET" to "http://localhost:9999/counter"
    Then "upstream-stub" received "GET" "/counter" "3" times

  # Verification - Did not receive
  Scenario: Verify request was not received
    Given "upstream-stub" stub "GET" "/exists" returns "200"
    When "app-client" sends "GET" to "http://localhost:9999/exists"
    Then "upstream-stub" did not receive "DELETE" "/exists"
    And "upstream-stub" did not receive "GET" "/other"

  # Verification - Header check
  Scenario: Verify request header
    Given "upstream-stub" stub "GET" "/auth" returns "200"
    Given "app-client" header "Authorization" is "Bearer token123"
    When "app-client" sends "GET" to "http://localhost:9999/auth"
    Then "upstream-stub" received request with header "Authorization" containing "Bearer"

  # Verification - Body check
  Scenario: Verify request body
    Given "upstream-stub" stub "POST" "/data" returns "200"
    Given "app-client" header "Content-Type" is "application/json"
    When "app-client" sends "POST" to "http://localhost:9999/data" with json:
      """
      {"action": "create", "value": 42}
      """
    Then "upstream-stub" received request with body containing "action"
    And "upstream-stub" received request with body containing "create"

  # Verification - Total requests
  Scenario: Verify total request count
    Given "upstream-stub" stub "GET" "/a" returns "200"
    And "upstream-stub" stub "GET" "/b" returns "200"
    And "upstream-stub" stub "POST" "/c" returns "201"
    When "app-client" sends "GET" to "http://localhost:9999/a"
    And "app-client" sends "GET" to "http://localhost:9999/b"
    Given "app-client" header "Content-Type" is "application/json"
    When "app-client" sends "POST" to "http://localhost:9999/c" with json:
      """
      {}
      """
    Then "upstream-stub" received "3" requests

  # No stub match returns 404
  Scenario: Unmatched request returns 404
    When "app-client" sends "GET" to "http://localhost:9999/not-stubbed"
    Then "app-client" response status is "404"

  # Multiple stub servers
  Scenario: Multiple mock servers work independently
    Given "upstream-stub" stub "GET" "/service-a" returns "200" with json:
      """
      {"service": "A"}
      """
    And "other-upstream-stub" stub "GET" "/service-b" returns "200" with json:
      """
      {"service": "B"}
      """
    When "app-client" sends "GET" to "http://localhost:9999/service-a"
    Then "app-client" response status is "200"
    And "app-client" response json "service" is "A"
    When "app-client" sends "GET" to "http://localhost:9998/service-b"
    Then "app-client" response status is "200"
    And "app-client" response json "service" is "B"
    # Verify isolation - each stub server only has its own stubs
    When "app-client" sends "GET" to "http://localhost:9999/service-b"
    Then "app-client" response status is "404"
    When "app-client" sends "GET" to "http://localhost:9998/service-a"
    Then "app-client" response status is "404"

  Scenario: Store the stub server URL in a variable
    Given "upstream-stub" url is stored in "MOCK_URL"
    And "upstream-stub" stub "GET" "/from-variable" returns "200" with body:
      """
      reached through the stored url
      """
    When "app-client" sends "GET" to "{{MOCK_URL}}/from-variable"
    Then "app-client" response status is "200"
    And "app-client" response body contains "stored url"
    And "upstream-stub" received "GET" "/from-variable"

  # Received Request - save what the app sent, assert its body
  Scenario: Save a field of a received request and assert its body
    Given "upstream-stub" stub "POST" "/orders" returns "201"
    And "upstream-stub" stub "POST" "/orders/ord-1/cancel" returns "200"
    And "app-client" header "Idempotency-Key" is "key-7"
    When "app-client" sends "POST" to "http://localhost:9999/orders" with json:
      """
      {"clientProcessId": "cp-42", "orderSize": "10", "leg": {"id": "ord-1"}}
      """
    Then "upstream-stub" last "POST" "/orders" request json contains:
      """
      {"orderSize": "10", "leg": {"id": "@notempty"}}
      """
    And "upstream-stub" last "POST" "/orders" request json "clientProcessId" saved as "{{process_id}}"
    And "upstream-stub" last "POST" "/orders" request json "leg.id" saved as "{{order_id}}"
    And "upstream-stub" last "POST" "/orders" request header "Idempotency-Key" saved as "{{key}}"
    When "app-client" sends "POST" to "http://localhost:9999/orders/{{order_id}}/cancel" with json:
      """
      {"clientProcessId": "{{process_id}}", "key": "{{key}}"}
      """
    Then "upstream-stub" last "POST" "/orders/{{order_id}}/cancel" request json contains:
      """
      {"clientProcessId": "cp-42", "key": "key-7"}
      """

  # Stub matching - one path, many operations
  Scenario: Stubs match on a header, a query parameter or the body
    Given "upstream-stub" stub "POST" "/" with header "X-Amz-Target" "Kms.Encrypt" returns "200" with json:
      """
      {"op": "encrypt"}
      """
    And "upstream-stub" stub "POST" "/" with header "X-Amz-Target" "Kms.Decrypt" returns "200" with body:
      """
      decrypted
      """
    And "upstream-stub" stub "GET" "/repos" with query "page" "2" returns "200" with headers:
      | header | value |
      | X-Page | 2     |
    And "upstream-stub" stub "POST" "/graphql" with body containing "resolveThread" returns "204"
    Given "app-client" header "X-Amz-Target" is "Kms.Decrypt"
    When "app-client" sends "POST" to "http://localhost:9999/"
    Then "app-client" response body contains "decrypted"
    And "upstream-stub" received "POST" "/" with header "X-Amz-Target" "Kms.Decrypt"
    When "app-client" sends "GET" to "http://localhost:9999/repos?page=2"
    Then "app-client" response header "X-Page" is "2"
    When "app-client" sends "POST" to "http://localhost:9999/graphql" with json:
      """
      {"query": "mutation resolveThread"}
      """
    Then "app-client" response status is "204"

  Scenario: The most specific stub wins, then the most recent
    Given "upstream-stub" stub "GET" "/greeting" returns "200" with body:
      """
      from the background
      """
    And "upstream-stub" stub "GET" "/greeting" returns "200" with body:
      """
      overridden
      """
    When "app-client" sends "GET" to "http://localhost:9999/greeting"
    Then "app-client" response body contains "overridden"

  # Stub sequence - retries, polls, a slow first answer
  Scenario: A stub answers in sequence and repeats its last answer
    Given "upstream-stub" stub "POST" "/token" returns in sequence:
      | status | json                    | delay |
      | 400    | {"error": "pending"}    | 100ms |
      | 200    | {"token": "t-1"}        |       |
    When "app-client" sends "POST" to "http://localhost:9999/token"
    Then "app-client" response status is "400"
    When "app-client" sends "POST" to "http://localhost:9999/token"
    Then "app-client" response json "token" is "t-1"
    When "app-client" sends "POST" to "http://localhost:9999/token"
    Then "app-client" response json "token" is "t-1"
    And "upstream-stub" received "POST" "/token" "3" times

  # Availability - the dependency is down
  Scenario: A stub server refuses connections and comes back
    Given "upstream-stub" stub "GET" "/health" returns "200"
    When "upstream-stub" refuses connections
    Then "shell" runs "curl -s -o /dev/null http://localhost:9999/health"
    And "shell" exit code is "7"
    When "upstream-stub" accepts connections
    And "app-client" sends "GET" to "http://localhost:9999/health"
    Then "app-client" response status is "200"
