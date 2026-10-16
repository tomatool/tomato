Feature: A browser scenario that fails on purpose

  Scenario: The order is cancelled
    Given "page" opens "data:text/html,<h1>Order confirmed</h1>"
    Then "page" sees "Order cancelled"
