@http @sse
Feature: HTTP event streams
  The http resource reads Server-Sent Events streams. The test app's /events
  endpoint sends "tick" events with ids counting up from Last-Event-ID + 1, and
  echoes the X-Client header in each event's data.

  Scenario: Events arrive in order with the scenario's headers
    Given "app-client" header "X-Client" is "tomato"
    When "app-client" opens stream "/events?count=3"
    Then "app-client" receives event "tick" within "2s" with json containing:
      """
      {"seq": 1, "client": "tomato"}
      """
    And "app-client" receives events within "2s":
      | event | id | data        |
      | tick  | 2  | {"seq": 2}  |
      | tick  | 3  |             |
    And "app-client" stream event ids are consecutive from "1"
    And "app-client" does not receive an event within "300ms"

  Scenario: Query params set before opening go with the stream request
    Given "app-client" query param "count" is "1"
    When "app-client" opens stream "/events"
    Then "app-client" receives event "tick" within "2s"
    And "app-client" does not receive an event within "300ms"

  Scenario: An idle stream keeps sending heartbeat comments
    When "app-client" opens stream "/events?count=0"
    Then "app-client" receives a comment within "2s"
    And "app-client" receives a comment within "2s"
    And "app-client" does not receive an event within "300ms"

  Scenario: A client resumes from the last event it processed
    Given "app-client" opens stream "/events?count=2"
    And "app-client" receives event "tick" within "2s"
    And "app-client" receives event "tick" within "2s"
    And "app-client" last stream event id saved as "{{last_id}}"
    And "app-client" closes stream
    When "app-client" opens stream "/events?count=2" after event id "{{last_id}}"
    Then "app-client" receives events within "2s":
      | event | id |
      | tick  | 3  |
      | tick  | 4  |
    And "app-client" stream event ids are consecutive from "3"

  Scenario: The server ends the stream
    When "app-client" opens stream "/events?count=1&close=true"
    Then "app-client" receives event "tick" within "2s"
    And "app-client" stream is closed by the server within "2s"

  Scenario: Heartbeats sent as events are skipped
    Given "app-client" stream skips events "heartbeat"
    When "app-client" opens stream "/events?count=1&heartbeat=event"
    Then "app-client" receives event "tick" within "2s"
    And "app-client" does not receive an event within "500ms"
    And "app-client" stream event ids are consecutive from "1"

  Scenario: Request steps still work while a stream is open
    Given "app-client" opens stream "/events?count=1"
    When "app-client" sends "GET" to "/health"
    Then "app-client" response status is "200"
    And "app-client" receives event "tick" within "2s"

  Scenario: A client that stops reading is disconnected, and loses nothing it was sent
    Given "app-client" opens stream "/events?burst=true"
    And "app-client" receives event "tick" within "2s"
    When "app-client" stops reading the stream
    # Nothing to assert while paused: the server needs its 300ms write
    # deadline to pass with the connection's buffers full.
    And "shell" runs "sleep 1.5"
    And "app-client" resumes reading the stream
    Then "app-client" stream is disconnected by the server within "10s"
    And "app-client" stream event ids are consecutive from "1"
