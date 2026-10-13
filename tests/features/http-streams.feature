@http @sse
Feature: HTTP event streams
  The http resource reads Server-Sent Events streams. The test app's /events
  endpoint sends "tick" events with ids counting up from Last-Event-ID + 1, and
  echoes the X-Client header in each event's data.

  Scenario: Events arrive in order with the scenario's headers
    Given "api" header "X-Client" is "tomato"
    When "api" opens stream "/events?count=3"
    Then "api" receives event "tick" within "2s" with json containing:
      """
      {"seq": 1, "client": "tomato"}
      """
    And "api" receives events within "2s":
      | event | id | data        |
      | tick  | 2  | {"seq": 2}  |
      | tick  | 3  |             |
    And "api" stream event ids are consecutive from "1"
    And "api" does not receive an event within "300ms"

  Scenario: Query params set before opening go with the stream request
    Given "api" query param "count" is "1"
    When "api" opens stream "/events"
    Then "api" receives event "tick" within "2s"
    And "api" does not receive an event within "300ms"

  Scenario: An idle stream keeps sending heartbeat comments
    When "api" opens stream "/events?count=0"
    Then "api" receives a comment within "2s"
    And "api" receives a comment within "2s"
    And "api" does not receive an event within "300ms"

  Scenario: A client resumes from the last event it processed
    Given "api" opens stream "/events?count=2"
    And "api" receives event "tick" within "2s"
    And "api" receives event "tick" within "2s"
    And "api" last stream event id saved as "{{last_id}}"
    And "api" closes stream
    When "api" opens stream "/events?count=2" after event id "{{last_id}}"
    Then "api" receives events within "2s":
      | event | id |
      | tick  | 3  |
      | tick  | 4  |
    And "api" stream event ids are consecutive from "3"

  Scenario: The server ends the stream
    When "api" opens stream "/events?count=1&close=true"
    Then "api" receives event "tick" within "2s"
    And "api" stream is closed by the server within "2s"

  Scenario: Heartbeats sent as events are skipped
    Given "api" stream skips events "heartbeat"
    When "api" opens stream "/events?count=1&heartbeat=event"
    Then "api" receives event "tick" within "2s"
    And "api" does not receive an event within "500ms"
    And "api" stream event ids are consecutive from "1"

  Scenario: Request steps still work while a stream is open
    Given "api" opens stream "/events?count=1"
    When "api" sends "GET" to "/health"
    Then "api" response status is "200"
    And "api" receives event "tick" within "2s"

  Scenario: A client that stops reading is disconnected, and loses nothing it was sent
    Given "api" opens stream "/events?burst=true"
    And "api" receives event "tick" within "2s"
    When "api" stops reading the stream
    # Nothing to assert while paused: the server needs its 300ms write
    # deadline to pass with the connection's buffers full.
    And "shell" runs "sleep 1.5"
    And "api" resumes reading the stream
    Then "api" stream is disconnected by the server within "10s"
    And "api" stream event ids are consecutive from "1"
