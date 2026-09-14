@s3
Feature: S3 Handler
  Test all S3 handler steps against MinIO

  Scenario: Write and read an object
    Given "files" object "uploads/hello.txt" is "hello world"
    Then "files" object "uploads/hello.txt" should exist
    And "files" object "uploads/hello.txt" content should be "hello world"
    And "files" object "uploads/hello.txt" content should contain "world"
    And "files" object "uploads/hello.txt" size should be "11" bytes

  Scenario: Scenario reset clears the previous scenario's objects
    Then "files" object "uploads/hello.txt" should not exist
    And "files" bucket "uploads" should be empty

  Scenario: Write multiline content
    Given "files" object "uploads/user.json" is:
      """
      {"id": 1, "name": "John", "active": true}
      """
    Then "files" object "uploads/user.json" content should contain "John"

  Scenario: Match JSON content partially
    Given "files" object "uploads/user.json" is:
      """
      {"id": 42, "name": "John", "email": "john@test.com"}
      """
    Then "files" object "uploads/user.json" content should match:
      """
      {"id": "@number", "name": "John"}
      """

  Scenario: Buckets declared in config are created automatically
    Then "files" bucket "uploads" should exist
    And "files" bucket "reports" should exist

  Scenario: Create and delete a bucket
    Given "files" bucket "archive" exists
    Then "files" bucket "archive" should exist
    When "files" bucket "archive" is deleted
    Then "files" bucket "archive" should not exist

  Scenario: Nested keys and prefix counting
    Given "files" objects:
      | path                      | content |
      | uploads/2026/01/a.txt     | jan     |
      | uploads/2026/02/b.txt     | feb     |
      | uploads/2025/12/c.txt     | dec     |
    Then "files" bucket "uploads" should have "3" objects
    And "files" bucket "uploads" should have "2" objects with prefix "2026/"
    And "files" object "uploads/2026/01/a.txt" content should be "jan"

  Scenario: Delete a single object
    Given "files" object "uploads/temp.txt" is "scratch"
    And "files" bucket "uploads" should have "1" objects
    When "files" object "uploads/temp.txt" is deleted
    Then "files" object "uploads/temp.txt" should not exist
    And "files" bucket "uploads" should be empty

  Scenario: Empty a bucket without deleting it
    Given "files" objects:
      | path            | content |
      | uploads/a.txt   | one     |
      | uploads/b.txt   | two     |
    When "files" bucket "uploads" is empty
    Then "files" bucket "uploads" should exist
    And "files" bucket "uploads" should be empty

  Scenario: Content type
    Given "files" object "uploads/data.csv" is "id,name" with content type "text/csv"
    Then "files" object "uploads/data.csv" content type should be "text/csv"

  Scenario: Upload a local file
    Given "files" object "uploads/sample.csv" is file "tests/testdata/sample.csv"
    Then "files" object "uploads/sample.csv" should exist
    And "files" object "uploads/sample.csv" content should contain "John"

  Scenario: Capture object content into a variable
    Given "files" object "uploads/id.txt" is "user-7"
    When "files" object "uploads/id.txt" content is captured as "user_id"
    And "files" object "uploads/copy.txt" is "{{user_id}}"
    Then "files" object "uploads/copy.txt" content should be "user-7"

  Scenario: Dynamic variables in paths
    Given "files" object "uploads/{{uuid}}.txt" is "generated"
    Then "files" bucket "uploads" should have "1" objects

  Scenario: Wait for an object that appears asynchronously
    Given "files" object "uploads/late.txt" is "arrived"
    Then "files" object "uploads/late.txt" should exist within "5s"
    And "files" bucket "uploads" should have "1" objects within "5s"

  Scenario: Writing to an undeclared bucket creates it
    Given "files" object "adhoc/note.txt" is "auto-created"
    Then "files" bucket "adhoc" should exist
    And "files" object "adhoc/note.txt" content should be "auto-created"
