@websocket
Feature: WebSocket Handler
  Test all WebSocket handler steps

  Scenario: Connect and disconnect
    Given "ws" connects
    Then "ws" is connected
    When "ws" disconnects
    Then "ws" is disconnected

  Scenario: Send and receive text message
    Given "ws" connects
    When "ws" sends "Hello WebSocket!"
    Then "ws" receives within "15s":
      """
      Hello WebSocket!
      """
    And "ws" disconnects

  Scenario: Send and receive JSON message
    Given "ws" connects
    When "ws" sends json:
      """
      {"action": "ping"}
      """
    Then "ws" receives json within "15s" matching:
      """
      {"action": "pong"}
      """
    And "ws" disconnects

  Scenario: Echo action
    Given "ws" connects
    When "ws" sends json:
      """
      {"action": "echo", "payload": "test data"}
      """
    Then "ws" receives within "15s" containing "echo"
    And "ws" last message contains "test data"
    And "ws" disconnects

  Scenario: Send message with docstring
    Given "ws" connects
    When "ws" sends:
      """
      Plain text message
      """
    Then "ws" receives within "15s":
      """
      Plain text message
      """
    And "ws" disconnects

  Scenario: Check message count
    Given "ws" connects
    When "ws" sends "msg1"
    Then "ws" receives within "15s":
      """
      msg1
      """
    When "ws" sends "msg2"
    Then "ws" receives within "15s":
      """
      msg2
      """
    When "ws" sends "msg3"
    Then "ws" receives within "15s":
      """
      msg3
      """
    And "ws" received "3" messages
    And "ws" disconnects

  Scenario: No message expected
    Given "ws" connects
    Then "ws" does not receive within "1s"
    And "ws" disconnects

  Scenario: Last message assertions
    Given "ws" connects
    When "ws" sends "final message"
    Then "ws" receives within "15s":
      """
      final message
      """
    And "ws" last message is:
      """
      final message
      """
    And "ws" last message contains "final"
    And "ws" disconnects
