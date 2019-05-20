Feature: queue features example

    Scenario: Publish and consume message
        # This message should be ignored by the next step, because tomato haven't listen to this target yet.
        Given publish message to "tomato-queue" target "customers:created" with payload
            """
          {
              "country":"us",
              "name":"cembri"
          }
            """

        Then listen message from "tomato-queue" target "customers:created"
        Then listen message from "tomato-queue" target "customers:deleted"
        Then listen message from "tomato-queue" target "customers:updated"
        Then publish message to "tomato-queue" target "customers:created" with payload
            """
            {
                "country":"us",
                "name":"cembri"
            }
            """
        Then publish message to "tomato-queue" target "customers:created" with payload
            """
            {
                "country":"id",
                "name":"cebre",
                "timestamp":"2018-04-03 08:38:23"
            }
            """
        Then publish message to "tomato-queue" target "customers:updated" with payload from file "stub_1.json"
        Then message from "tomato-queue" target "customers:created" count should be 2
        Then message from "tomato-queue" target "customers:deleted" count should be 0
        Then message from "tomato-queue" target "customers:created" should contain
            """
            {
                "country":"us",
                "name":"cembri"
            }
            """
        Then message from "tomato-queue" target "customers:created" should contain
            """
            {
                "country":"id",
                "name":"cebre",
                "timestamp":"*"
            }
            """
        Then message from "tomato-queue" target "customers:updated" count should be 1
        Then message from "tomato-queue" target "customers:updated" should contain
            """
            {
                "country":"us",
                "name":"tom"
            }
            """

    Scenario: Publish and consume message using nsq
        # This message should be ignored by the next step, because tomato haven't listen to this target yet.
        Given publish message to "tomato-nsq" target "customer_created" with payload
            """
          {
              "country":"us",
              "name":"cembri"
          }
            """

        Then listen message from "tomato-nsq" target "customer_created"
        Then listen message from "tomato-nsq" target "customer_deleted"
        Then listen message from "tomato-nsq" target "customer_updated"
        Then publish message to "tomato-nsq" target "customer_created" with payload
            """
            {
                "country":"us",
                "name":"cembri"
            }
            """
        Then publish message to "tomato-nsq" target "customer_created" with payload
            """
            {
                "country":"id",
                "name":"cebre",
                "timestamp":"2018-04-03 08:38:23"
            }
            """
        Then publish message to "tomato-nsq" target "customer_updated" with payload from file "stub_1.json"
        Then message from "tomato-nsq" target "customer_created" count should be 2
        Then message from "tomato-nsq" target "customer_deleted" count should be 0
        Then message from "tomato-nsq" target "customer_created" should contain
            """
            {
                "country":"us",
                "name":"cembri"
            }
            """
        Then message from "tomato-nsq" target "customer_created" should contain
            """
            {
                "country":"id",
                "name":"cebre",
                "timestamp":"*"
            }
            """
        Then message from "tomato-nsq" target "customer_updated" count should be 1
        Then message from "tomato-nsq" target "customer_updated" should contain
            """
            {
                "country":"us",
                "name":"tom"
            }
            """
