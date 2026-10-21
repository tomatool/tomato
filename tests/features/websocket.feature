@websocket
Feature: WebSocket Handler
  Test all WebSocket handler steps

  Scenario: Connect and disconnect
    Given "app-ws-client" connects
    Then "app-ws-client" is connected
    When "app-ws-client" disconnects
    Then "app-ws-client" is disconnected

  Scenario: Send and receive text message
    Given "app-ws-client" connects
    When "app-ws-client" sends "Hello WebSocket!"
    Then "app-ws-client" receives within "5s":
      """
      Hello WebSocket!
      """
    And "app-ws-client" disconnects

  Scenario: Send and receive JSON message
    Given "app-ws-client" connects
    When "app-ws-client" sends json:
      """
      {"action": "ping"}
      """
    Then "app-ws-client" receives json within "5s" matching:
      """
      {"action": "pong"}
      """
    And "app-ws-client" disconnects

  Scenario: Echo action
    Given "app-ws-client" connects
    When "app-ws-client" sends json:
      """
      {"action": "echo", "payload": "test data"}
      """
    Then "app-ws-client" receives within "5s" containing "echo"
    And "app-ws-client" last message contains "test data"
    And "app-ws-client" disconnects

  Scenario: Send message with docstring
    Given "app-ws-client" connects
    When "app-ws-client" sends:
      """
      Plain text message
      """
    Then "app-ws-client" receives within "5s":
      """
      Plain text message
      """
    And "app-ws-client" disconnects

  Scenario: Check message count
    Given "app-ws-client" connects
    When "app-ws-client" sends "msg1"
    Then "app-ws-client" receives within "5s":
      """
      msg1
      """
    When "app-ws-client" sends "msg2"
    Then "app-ws-client" receives within "5s":
      """
      msg2
      """
    When "app-ws-client" sends "msg3"
    Then "app-ws-client" receives within "5s":
      """
      msg3
      """
    And "app-ws-client" received "3" messages
    And "app-ws-client" disconnects

  Scenario: No message expected
    Given "app-ws-client" connects
    Then "app-ws-client" does not receive within "1s"
    And "app-ws-client" disconnects

  Scenario: Last message assertions
    Given "app-ws-client" connects
    When "app-ws-client" sends "final message"
    Then "app-ws-client" receives within "5s":
      """
      final message
      """
    And "app-ws-client" last message is:
      """
      final message
      """
    And "app-ws-client" last message contains "final"
    And "app-ws-client" disconnects

  Scenario: Connect with handshake headers
    Given "app-ws-client" connects with headers:
      | header      | value    |
      | X-Client-Id | tomato-1 |
    Then "app-ws-client" receives json within "5s" matching:
      """
      {"action": "welcome", "client": "tomato-1"}
      """
    And "app-ws-client" disconnects

  Scenario: Receive a batch of messages
    Given "app-ws-client" connects
    When "app-ws-client" sends "one"
    And "app-ws-client" sends "two"
    And "app-ws-client" sends "three"
    Then "app-ws-client" receives "3" messages within "5s"
    And "app-ws-client" last message is:
      """
      three
      """
    And "app-ws-client" received "3" messages
    And "app-ws-client" does not receive within "300ms"
    And "app-ws-client" disconnects

  Scenario: Last message as JSON
    Given "app-ws-client" connects
    When "app-ws-client" sends json:
      """
      {"action": "echo", "payload": {"order": "order-1"}}
      """
    Then "app-ws-client" receives within "5s" containing "order-1"
    And "app-ws-client" last message is json matching:
      """
      {"action": "echo", "payload": {"order": "order-1"}}
      """
    And "app-ws-client" disconnects
