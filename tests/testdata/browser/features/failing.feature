Feature: A browser scenario that fails on purpose

  Scenario: The order is cancelled
    Given "visitor-browser" opens "data:text/html,<h1>Order confirmed</h1>"
    Then "visitor-browser" sees "Order cancelled"
