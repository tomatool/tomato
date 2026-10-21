@s3
Feature: S3 Handler
  Test all S3 handler steps against MinIO

  Scenario: Write and read an object
    Given "app-storage" object "uploads/hello.txt" is "hello world"
    Then "app-storage" object "uploads/hello.txt" should exist
    And "app-storage" object "uploads/hello.txt" content should be "hello world"
    And "app-storage" object "uploads/hello.txt" content should contain "world"
    And "app-storage" object "uploads/hello.txt" size should be "11" bytes

  Scenario: Scenario reset clears the previous scenario's objects
    Then "app-storage" object "uploads/hello.txt" should not exist
    And "app-storage" bucket "uploads" should be empty

  Scenario: Write multiline content
    Given "app-storage" object "uploads/user.json" is:
      """
      {"id": 1, "name": "John", "active": true}
      """
    Then "app-storage" object "uploads/user.json" content should contain "John"

  Scenario: Match JSON content partially
    Given "app-storage" object "uploads/user.json" is:
      """
      {"id": 42, "name": "John", "email": "john@test.com"}
      """
    Then "app-storage" object "uploads/user.json" content should match:
      """
      {"id": "@number", "name": "John"}
      """

  Scenario: Buckets declared in config are created automatically
    Then "app-storage" bucket "uploads" should exist
    And "app-storage" bucket "reports" should exist

  Scenario: Create and delete a bucket
    Given "app-storage" bucket "archive" exists
    Then "app-storage" bucket "archive" should exist
    When "app-storage" bucket "archive" is deleted
    Then "app-storage" bucket "archive" should not exist

  Scenario: Nested keys and prefix counting
    Given "app-storage" objects:
      | path                      | content |
      | uploads/2026/01/a.txt     | jan     |
      | uploads/2026/02/b.txt     | feb     |
      | uploads/2025/12/c.txt     | dec     |
    Then "app-storage" bucket "uploads" should have "3" objects
    And "app-storage" bucket "uploads" should have "2" objects with prefix "2026/"
    And "app-storage" object "uploads/2026/01/a.txt" content should be "jan"

  Scenario: Delete a single object
    Given "app-storage" object "uploads/temp.txt" is "scratch"
    And "app-storage" bucket "uploads" should have "1" objects
    When "app-storage" object "uploads/temp.txt" is deleted
    Then "app-storage" object "uploads/temp.txt" should not exist
    And "app-storage" bucket "uploads" should be empty

  Scenario: Empty a bucket without deleting it
    Given "app-storage" objects:
      | path            | content |
      | uploads/a.txt   | one     |
      | uploads/b.txt   | two     |
    When "app-storage" bucket "uploads" is empty
    Then "app-storage" bucket "uploads" should exist
    And "app-storage" bucket "uploads" should be empty

  Scenario: Content type
    Given "app-storage" object "uploads/data.csv" is "id,name" with content type "text/csv"
    Then "app-storage" object "uploads/data.csv" content type should be "text/csv"

  Scenario: Upload a local file
    Given "app-storage" object "uploads/sample.csv" is file "tests/testdata/sample.csv"
    Then "app-storage" object "uploads/sample.csv" should exist
    And "app-storage" object "uploads/sample.csv" content should contain "John"

  Scenario: Capture object content into a variable
    Given "app-storage" object "uploads/id.txt" is "user-7"
    When "app-storage" object "uploads/id.txt" content is captured as "user_id"
    And "app-storage" object "uploads/copy.txt" is "{{user_id}}"
    Then "app-storage" object "uploads/copy.txt" content should be "user-7"

  Scenario: Dynamic variables in paths
    Given "app-storage" object "uploads/{{uuid}}.txt" is "generated"
    Then "app-storage" bucket "uploads" should have "1" objects

  Scenario: Wait for an object that appears asynchronously
    Given "app-storage" object "uploads/late.txt" is "arrived"
    Then "app-storage" object "uploads/late.txt" should exist within "5s"
    And "app-storage" bucket "uploads" should have "1" objects within "5s"

  Scenario: Writing to an undeclared bucket creates it
    Given "app-storage" object "adhoc/note.txt" is "auto-created"
    Then "app-storage" bucket "adhoc" should exist
    And "app-storage" object "adhoc/note.txt" content should be "auto-created"

  Scenario: Exact multi-line content from a docstring
    Given "app-storage" object "uploads/greeting.txt" is:
      """
      hello
      world
      """
    Then "app-storage" object "uploads/greeting.txt" content should be:
      """
      hello
      world
      """

  Scenario: Upload with user metadata and assert it
    Given "app-storage" object "uploads/report.csv" is "id,name" with metadata:
      | key   | value   |
      | owner | billing |
      | team  | finance |
    Then "app-storage" object "uploads/report.csv" metadata "owner" should be "billing"
    And "app-storage" object "uploads/report.csv" metadata "team" should be "finance"
