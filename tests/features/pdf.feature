@pdf
Feature: PDF documents
  The application generates PDFs. The suite asserts what is in them, not just
  that they exist: pages, text and its order, metadata, the outline, and
  accessibility.

  Scenario: A contract served by the API has its pages, text, metadata and outline
    When "api" sends "GET" to "/documents/contract.pdf"
    Then "api" response status is "200"
    And "api" response header "Content-Type" is "application/pdf"
    When "pdf" reads "api" response body
    Then "pdf" has "5" pages
    And "pdf" text contains "9 Kryptowerte"
    And "pdf" text contains "Absichtlich frei gelassen"
    And "pdf" text contains "Version 07.07"
    And "pdf" text does not contain "{{"
    And "pdf" page "3" contains "Inhaltsverzeichnis"
    And "pdf" metadata "Title" is "Kundenvereinbarung"
    And "pdf" metadata "Producer" is "tomato fixtures"
    And "pdf" outline contains "7 Zinsen"
    And "pdf" outline contains "7.1 Sollzinsen"

  Scenario: A bilingual contract in the bucket is sequential, not columns
    Given "files" object "reports/cas/CA_Version_No._07.07_AT-de.pdf" is file "tests/testdata/pdf/contract.pdf"
    When "pdf" reads "files" object "reports/cas/CA_Version_No._07.07_AT-de.pdf"
    Then "pdf" text "Allgemeine Geschäftsbedingungen" appears before "General Terms and Conditions"
    And "pdf" text "Conditions générales" appears before "General Terms and Conditions"

  Scenario: A document on disk
    When "pdf" reads file "tests/testdata/pdf/contract.pdf"
    Then "pdf" has "5" pages
    And "pdf" language is "de-AT"

  Scenario: The accessible version is tagged and conforms to PDF/UA-1
    When "api" sends "GET" to "/documents/accessible.pdf"
    And "pdf" reads "api" response body
    Then "pdf" is tagged
    And "pdf" language is "de-AT"
    And "pdf" metadata "Title" is "Kundenvereinbarung"
    And "pdf" conforms to "PDF/UA-1"

  Scenario: Every scenario starts without a document
    When "pdf" reads file "tests/testdata/pdf/accessible.pdf"
    Then "pdf" has "1" pages
