Feature: test feature

  Scenario: Delete user feature
    Given set "db1" table "user" list of content
      | user_id |
      | 1       |
    Given "http" send request to "DELETE /api/v1/users/1" with body
    """
    """
    Then "http" response code should be 200
    Given "db1" table "user" should look like
      | user_id |
