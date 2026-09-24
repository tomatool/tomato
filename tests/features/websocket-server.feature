@websocket-server
Feature: WebSocket Server Handler
  Stub a WebSocket dependency. "wsmock" is the stub server; "wsmock-client"
  connects to it the way an app would.

  Scenario: Greet clients on connect
    Given "wsmock" on connect sends:
      """
      {"type": "welcome"}
      """
    When "wsmock-client" connects
    Then "wsmock-client" receives json within "5s" matching:
      """
      {"type": "welcome"}
      """
    And "wsmock" has "1" connections

  Scenario: Reply to an exact message
    Given "wsmock" on message "ping" replies:
      """
      pong
      """
    And "wsmock-client" connects
    When "wsmock-client" sends "ping"
    Then "wsmock-client" receives within "5s":
      """
      pong
      """
    And "wsmock" received message "ping"
    And "wsmock" received "1" messages

  Scenario: Reply to messages matching a pattern
    Given "wsmock" on message matching "subscribe:.*" replies:
      """
      {"status": "subscribed"}
      """
    And "wsmock-client" connects
    When "wsmock-client" sends "subscribe:orders"
    And "wsmock-client" sends "subscribe:payments"
    Then "wsmock-client" receives "2" messages within "5s"
    And "wsmock-client" last message is json matching:
      """
      {"status": "subscribed"}
      """
    And "wsmock" received "2" messages

  Scenario: Messages without a rule get no reply
    Given "wsmock" on message "ping" replies:
      """
      pong
      """
    And "wsmock-client" connects
    When "wsmock-client" sends "hello?"
    Then "wsmock" received message "hello?"
    And "wsmock-client" does not receive within "300ms"

  Scenario: Broadcast to connected clients
    Given "wsmock-client" connects
    And "wsmock" has "1" connections
    When "wsmock" broadcasts:
      """
      {"event": "price", "value": 42}
      """
    Then "wsmock-client" receives json within "5s" matching:
      """
      {"event": "price", "value": 42}
      """
    When "wsmock" broadcasts "tick"
    Then "wsmock-client" receives within "5s":
      """
      tick
      """

  Scenario: Connections close between scenarios
    Then "wsmock" has "0" connections
    And "wsmock" received "0" messages

  Scenario: Disconnecting drops the connection
    Given "wsmock-client" connects
    And "wsmock" has "1" connections
    When "wsmock-client" disconnects
    Then "wsmock" has "0" connections
