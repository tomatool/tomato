Feature: queue features example

    Background:
        Then listen message from "tomato-queue" target "customers:created"

    Scenario: Publish and consume message
        Then message from "tomato-queue" target "customers:created" count should be 0
        Given publish message to "tomato-queue" target "customers:created" with payload
            """
          {
              "country":"us",
              "name":"cembri"
          }
            """
        Then message from "tomato-queue" target "customers:created" count should be 1
        Then message from "tomato-queue" target "customers:created" should contain
            """
            {
                "country":"us"
            }
            """
        Then message from "tomato-queue" target "customers:created" should contain
            """
            {
                "country":"us",
                "name": "*"
            }
            """
        Then message from "tomato-queue" target "customers:created" should contain
            """
            {
                "country":"us",
                "name": "cembri"
            }
            """
        Then message from "tomato-queue" target "customers:created" count should be 1

    Scenario: Publish and consume message
        Then message from "tomato-queue" target "customers:created" count should be 0
        Given publish message to "tomato-queue" target "customers:created" with payload
            """
          {
              "country":"us",
              "payload": {
                "age": 34,
                "arr": [1,5,7,9]
              }
          }
            """
        Then message from "tomato-queue" target "customers:created" count should be 1
        Then message from "tomato-queue" target "customers:created" should contain
            """
            {
                "country":"*",
                "payload": {
                  "age": 34,
                  "arr": "*"
                }
            }
            """
