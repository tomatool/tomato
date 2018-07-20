Feature: test feature

  Scenario: Send test request
    Given "http" send request to "DELETE /api/v1/selection" with body
    """
    {
      "country":"DE",
      "subscriptionId":123,
      "interval":"2018-W01",
      "selection":"1-2-3",
      "customerId":321,
      "productSku":"uyeah"
    }
    """
    Then "http" response code should be 400
    Then "http" response body should be
    """
      {
        "test":"yeah"
      }
    """
