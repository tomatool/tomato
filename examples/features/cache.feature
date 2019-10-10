Feature: cache features example

  Scenario: Store key-value in cache and check if it exists and valid
    Given cache "tomato-redis" stores "example-key" with value "example-value"
    Then cache "tomato-redis" has key "example-key"
    Then cache "tomato-redis" hasn't key "wrong-example-key"
    Then cache "tomato-redis" stored key "example-key" should look like "example-value"
