Feature: WebSocket Communication
  As a developer
  I want to test WebSocket connections
  So that I can verify real-time communication works correctly

  Scenario: Connect and send message
    Given I connect to websocket "ws"
    Then websocket "ws" should be connected
    When I send message to websocket "ws":
      """
      {"type": "ping"}
      """
    Then I should receive message from websocket "ws" within "5s":
      """
      {"type": "pong"}
      """

  Scenario: Connect with custom headers
    Given I connect to websocket "ws" with headers:
      | header        | value           |
      | Authorization | Bearer token123 |
    Then websocket "ws" should be connected

  Scenario: Send and receive JSON
    Given I connect to websocket "ws"
    When I send JSON to websocket "ws":
      """
      {
        "type": "subscribe",
        "channel": "notifications"
      }
      """
    Then I should receive message from websocket "ws" within "5s" containing "subscribed"

  Scenario: Receive multiple messages
    Given I connect to websocket "ws"
    When I send JSON to websocket "ws":
      """
      {"type": "subscribe", "channel": "updates"}
      """
    Then I should receive "3" messages from websocket "ws" within "10s"
    And websocket "ws" should have received "3" messages

  Scenario: JSON message matching
    Given I connect to websocket "ws"
    When I send text "hello" to websocket "ws"
    Then I should receive JSON from websocket "ws" within "5s" matching:
      """
      {
        "type": "echo",
        "message": "hello"
      }
      """

  Scenario: Last message assertions
    Given I connect to websocket "ws"
    When I send message to websocket "ws":
      """
      {"action": "get_status"}
      """
    Then I should receive message from websocket "ws" within "5s" containing "status"
    And the last message from websocket "ws" should contain "status"

  Scenario: No unexpected messages
    Given I connect to websocket "ws"
    # After authentication, we should not receive unsolicited messages
    Then I should not receive message from websocket "ws" within "2s"

  Scenario: Disconnect and reconnect
    Given I connect to websocket "ws"
    Then websocket "ws" should be connected
    When I disconnect from websocket "ws"
    Then websocket "ws" should be disconnected
    When I connect to websocket "ws"
    Then websocket "ws" should be connected

  Scenario: Chat room simulation
    Given I connect to websocket "ws"
    When I send JSON to websocket "ws":
      """
      {"type": "join", "room": "general"}
      """
    Then I should receive message from websocket "ws" within "5s" containing "joined"
    When I send JSON to websocket "ws":
      """
      {"type": "message", "room": "general", "text": "Hello everyone!"}
      """
    Then I should receive message from websocket "ws" within "5s" containing "Hello everyone!"
