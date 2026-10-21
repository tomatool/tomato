@network
Feature: Network faults between the app and its dependencies
  A network resource stands between the app and each dependency container. A
  scenario makes the link slow, hung, cut or down, and checks what the app's
  own clients do. Only the app goes through the proxies: tomato's resources
  still reach the containers directly. Every scenario starts healthy.

  Scenario Outline: A healthy network is invisible to <dependency>
    When "app-client" sends "GET" to "/deps/<dependency>"
    Then "app-client" response status is "200"

    Examples:
      | dependency |
      | postgres   |
      | redis      |
      | s3         |
      | kafka      |
      | rabbitmq   |

  Scenario Outline: Latency reaches the connections <dependency> already has open
    Given "app-client" sends "GET" to "/deps/<dependency>"
    And "app-client" response status is "200"
    When "<network>" adds latency "300ms"
    And "app-client" sends "GET" to "/deps/<dependency>"
    Then "app-client" response status is "200"
    And "app-client" response json contains:
      """
      {"ms": "@gte:300"}
      """

    Examples:
      | dependency | network          |
      | postgres   | postgres-network |
      | redis      | redis-network    |
      | s3         | minio-network    |
      | kafka      | kafka-network    |
      | rabbitmq   | rabbitmq-network |

  Scenario Outline: <dependency> stops responding, and the app gives up instead of hanging
    Given "app-client" sends "GET" to "/deps/<dependency>"
    And "app-client" response status is "200"
    When "<network>" stops responding
    And "app-client" sends "GET" to "/deps/<dependency>"
    Then "app-client" response status is "502"
    When "<network>" is healthy
    And "app-client" sends "GET" to "/deps/<dependency>?retry=10s"
    Then "app-client" response status is "200"

    Examples:
      | dependency | network          |
      | postgres   | postgres-network |
      | redis      | redis-network    |
      | s3         | minio-network    |
      | kafka      | kafka-network    |
      | rabbitmq   | rabbitmq-network |

  # How it rides it out is the client's: the probe retries, and go-redis
  # retries on its own, inside one attempt. Either way it waited the outage.
  Scenario Outline: The app rides out <dependency> being down for a second
    Given "app-client" sends "GET" to "/deps/<dependency>"
    And "app-client" response status is "200"
    When "<network>" refuses connections for "1s"
    And "app-client" sends "GET" to "/deps/<dependency>?retry=10s"
    Then "app-client" response status is "200"
    And "app-client" response json contains:
      """
      {"ms": "@gte:800"}
      """

    Examples:
      | dependency | network          |
      | postgres   | postgres-network |
      | redis      | redis-network    |
      | s3         | minio-network    |
      | kafka      | kafka-network    |
      | rabbitmq   | rabbitmq-network |

  Scenario: The app reconnects to RabbitMQ after its connection is reset
    Given "app-client" sends "GET" to "/deps/rabbitmq"
    And "app-client" response status is "200"
    When "rabbitmq-network" resets connections for "1s"
    And "app-client" sends "GET" to "/deps/rabbitmq?retry=10s"
    Then "app-client" response status is "200"
    And "rabbitmq-network" sees a new connection within "5s"

  Scenario: The next connections fail, and only those
    Given "minio-network" fails the next "2" connections
    When "app-client" sends "GET" to "/deps/s3?fresh=1"
    Then "app-client" response status is "502"
    When "app-client" sends "GET" to "/deps/s3?fresh=1"
    Then "app-client" response status is "502"
    When "app-client" sends "GET" to "/deps/s3?fresh=1"
    Then "app-client" response status is "200"
    And "minio-network" has seen "3" connections

  Scenario: tomato's own resources reach the containers directly
    Given "postgres-network" refuses connections
    And "kafka-network" refuses connections
    And "app-events" consumes from "test-events"
    When "app-client" sends "GET" to "/deps/postgres"
    Then "app-client" response status is "502"
    And "app-db" table "users" has "0" rows
    When "app-events" publishes to "test-events":
      """
      {"id": 1}
      """
    Then "app-events" receives from "test-events" within "10s"
