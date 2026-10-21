@websocket-server
Feature: WebSocket Server Handler
  Stub a WebSocket dependency. "feed-stub" is the stub server; "feed-client"
  connects to it the way an app would.

  Scenario: Greet clients on connect
    Given "feed-stub" on connect sends:
      """
      {"type": "welcome"}
      """
    When "feed-client" connects
    Then "feed-client" receives json within "5s" matching:
      """
      {"type": "welcome"}
      """
    And "feed-stub" has "1" connections

  Scenario: Reply to an exact message
    Given "feed-stub" on message "ping" replies:
      """
      pong
      """
    And "feed-client" connects
    When "feed-client" sends "ping"
    Then "feed-client" receives within "5s":
      """
      pong
      """
    And "feed-stub" received message "ping"
    And "feed-stub" received "1" messages

  Scenario: Reply to messages matching a pattern
    Given "feed-stub" on message matching "subscribe:.*" replies:
      """
      {"status": "subscribed"}
      """
    And "feed-client" connects
    When "feed-client" sends "subscribe:orders"
    And "feed-client" sends "subscribe:payments"
    Then "feed-client" receives "2" messages within "5s"
    And "feed-client" last message is json matching:
      """
      {"status": "subscribed"}
      """
    And "feed-stub" received "2" messages

  Scenario: Messages without a rule get no reply
    Given "feed-stub" on message "ping" replies:
      """
      pong
      """
    And "feed-client" connects
    When "feed-client" sends "hello?"
    Then "feed-stub" received message "hello?"
    And "feed-client" does not receive within "300ms"

  Scenario: Broadcast to connected clients
    Given "feed-client" connects
    And "feed-stub" has "1" connections
    When "feed-stub" broadcasts:
      """
      {"event": "price", "value": 42}
      """
    Then "feed-client" receives json within "5s" matching:
      """
      {"event": "price", "value": 42}
      """
    When "feed-stub" broadcasts "tick"
    Then "feed-client" receives within "5s":
      """
      tick
      """

  Scenario: Connections close between scenarios
    Then "feed-stub" has "0" connections
    And "feed-stub" received "0" messages

  Scenario: Disconnecting drops the connection
    Given "feed-client" connects
    And "feed-stub" has "1" connections
    When "feed-client" disconnects
    Then "feed-stub" has "0" connections
