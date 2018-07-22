Feature: queue features example

  Scenario: Publish and consume message
    Then listen message from "my-awesome-queue" target "customers:uyeah"
    Then publish message to "my-awesome-queue" target "customers:uyeah" with payload
        """
            {
                "test":"OK"
            }
        """
    Then publish message to "my-awesome-queue" target "customers:uyeah" with payload
        """
            {
                "test":"NOT OK"
            }
        """
    Then message from "my-awesome-queue" target "customers:uyeah" count should be 2
    Then message from "my-awesome-queue" target "customers:uyeah" should look like
        """
            {
                "test":"OK"
            }
        """
    Then message from "my-awesome-queue" target "customers:uyeah" should look like
        """
            {
                "test":"NOT OK"
            }
        """
