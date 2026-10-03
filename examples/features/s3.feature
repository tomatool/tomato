@s3
Feature: S3 Object Storage
  As a developer
  I want to test object storage operations
  So that I can verify file handling in my application

  Scenario: Basic object operations
    Given "files" object "uploads/greeting.txt" is "hello world"
    Then "files" object "uploads/greeting.txt" should exist
    And "files" object "uploads/greeting.txt" content should be "hello world"
    And "files" object "uploads/greeting.txt" size should be "11" bytes

  Scenario: Every scenario starts with empty storage
    Then "files" bucket "uploads" should be empty
    And "files" bucket "reports" should be empty

  Scenario: Storing JSON documents
    Given "files" object "uploads/user.json" is:
      """
      {
        "id": 1,
        "name": "Alice",
        "email": "alice@test.com"
      }
      """
    Then "files" object "uploads/user.json" content should match:
      """
      {"id": "@number", "name": "Alice"}
      """

  Scenario: Seeding several objects at once
    Given "files" objects:
      | path                     | content        |
      | reports/2026/q1.csv      | revenue,100    |
      | reports/2026/q2.csv      | revenue,150    |
      | reports/2025/q4.csv      | revenue,90     |
    Then "files" bucket "reports" should have "3" objects
    And "files" bucket "reports" should have "2" objects with prefix "2026/"
    And "files" object "reports/2026/q1.csv" content should contain "100"

  Scenario: Content types and metadata
    Given "files" object "uploads/data.csv" is "id,name" with content type "text/csv"
    Then "files" object "uploads/data.csv" content type should be "text/csv"

  Scenario: Deleting objects
    Given "files" object "uploads/temp.txt" is "scratch"
    When "files" object "uploads/temp.txt" is deleted
    Then "files" object "uploads/temp.txt" should not exist

  Scenario: Buckets are created on demand
    Given "files" object "archive/old.txt" is "archived"
    Then "files" bucket "archive" should exist

  Scenario: Waiting for an asynchronous upload
    Given "files" object "reports/nightly.csv" is "generated"
    Then "files" object "reports/nightly.csv" should exist within "10s"
    And "files" bucket "reports" should have "1" objects within "10s"

  Scenario: Reusing a value from one object in another
    Given "files" object "uploads/id.txt" is "user-42"
    When "files" object "uploads/id.txt" content is captured as "user_id"
    And "files" object "uploads/{{user_id}}.json" is "{{user_id}}"
    Then "files" object "uploads/user-42.json" content should be "user-42"
