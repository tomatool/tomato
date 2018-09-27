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
    Then message from "tomato-queue" target "customers:created" count should be 2
    Then message from "tomato-queue" target "customers:deleted" count should be 0
    Then message from "tomato-queue" target "customers:created" should look like
        """
            {
                "country":"us",
                "name":"cembri"
            }
        """
    Then message from "tomato-queue" target "customers:created" should look like
        """
            {
                "country":"id",
                "name":"cebre",
                "timestamp":"*"
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
    Then message from "tomato-nsq" target "customer_created" count should be 2
    Then message from "tomato-nsq" target "customer_deleted" count should be 0
    Then message from "tomato-nsq" target "customer_created" should look like
        """
            {
                "country":"us",
                "name":"cembri"
            }
        """
    Then message from "tomato-nsq" target "customer_created" should look like
        """
            {
                "country":"id",
                "name":"cebre",
                "timestamp":"*"
            }
        """
