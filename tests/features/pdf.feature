@pdf
Feature: PDF documents
  The application generates PDFs. The suite asserts what is in them, not just
  that they exist: pages, text and its order, metadata, the outline, and
  accessibility.

  Scenario: A contract served by the API has its pages, text, metadata and outline
    When "app-client" sends "GET" to "/documents/contract.pdf"
    Then "app-client" response status is "200"
    And "app-client" response header "Content-Type" is "application/pdf"
    When "document-pdf" reads "app-client" response body
    Then "document-pdf" has "5" pages
    And "document-pdf" text contains "9 Kryptowerte"
    And "document-pdf" text contains "Absichtlich frei gelassen"
    And "document-pdf" text contains "Version 07.07"
    And "document-pdf" text does not contain "{{"
    And "document-pdf" page "3" contains "Inhaltsverzeichnis"
    And "document-pdf" metadata "Title" is "Kundenvereinbarung"
    And "document-pdf" metadata "Producer" is "tomato fixtures"
    And "document-pdf" outline contains "7 Zinsen"
    And "document-pdf" outline contains "7.1 Sollzinsen"

  Scenario: A bilingual contract in the bucket is sequential, not columns
    Given "app-storage" object "reports/cas/CA_Version_No._07.07_AT-de.pdf" is file "tests/testdata/pdf/contract.pdf"
    When "document-pdf" reads "app-storage" object "reports/cas/CA_Version_No._07.07_AT-de.pdf"
    Then "document-pdf" text "Allgemeine Geschäftsbedingungen" appears before "General Terms and Conditions"
    And "document-pdf" text "Conditions générales" appears before "General Terms and Conditions"

  Scenario: A document on disk
    When "document-pdf" reads file "tests/testdata/pdf/contract.pdf"
    Then "document-pdf" has "5" pages
    And "document-pdf" language is "de-AT"

  Scenario: The accessible version is tagged and conforms to PDF/UA-1
    When "app-client" sends "GET" to "/documents/accessible.pdf"
    And "document-pdf" reads "app-client" response body
    Then "document-pdf" is tagged
    And "document-pdf" language is "de-AT"
    And "document-pdf" metadata "Title" is "Kundenvereinbarung"
    And "document-pdf" conforms to "PDF/UA-1"

  Scenario: Every scenario starts without a document
    When "document-pdf" reads file "tests/testdata/pdf/accessible.pdf"
    Then "document-pdf" has "1" pages
