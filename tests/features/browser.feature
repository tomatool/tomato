@browser
Feature: Browser resource
  Two people use the test app's contract editor (tests/app/ui.html) in their
  own browser contexts on one Chromium: one continuous document, a master
  clause applied to selected countries, and the database checked for the
  effect. Steps name what a person sees, never a selector.

  Scenario: The editor shows one document, not the clause structure underneath
    When "alice-browser" opens "/ui/master"
    Then "alice-browser" sees "7 Zinsen" within "5s"
    And "alice-browser" sees "7.2 Die Zinsen werden jährlich gezahlt" within "5s"
    And "alice-browser" does not see "clause_id"
    And "alice-browser" does not see "data-clause"

  Scenario: A master edit reaches only the selected countries
    Given "alice-browser" opens "/ui/master"
    And "alice-browser" sees "Apply" is disabled
    When "alice-browser" types " und verrechnet." at the end of "7.2"
    Then "alice-browser" sees "Apply" is enabled
    When "alice-browser" clicks "Apply"
    And "alice-browser" chooses "Selected countries"
    And "alice-browser" checks "Austria"
    And "alice-browser" checks "Netherlands"
    And "alice-browser" unchecks "Germany"
    And "alice-browser" selects "Next quarter" from "Effective from"
    And "alice-browser" clicks "Confirm"
    Then "alice-browser" sees "Change applied to 2 countries, effective next quarter" within "5s"
    And "alice-browser" sees "Apply" is disabled
    And "app-db" query "SELECT country, body, effective FROM country_clauses ORDER BY country" returns:
      | country | body                                              | effective    |
      | AT      | Die Zinsen werden jährlich gezahlt und verrechnet. | next-quarter |
      | NL      | Die Zinsen werden jährlich gezahlt und verrechnet. | next-quarter |

  Scenario: A country with its own version of the clause keeps it
    Given "app-db" table "country_clauses" has values:
      | country | clause_id | body                    | local_override |
      | AT      | 7.2       | Österreichische Fassung | true           |
    And "alice-browser" opens "/ui/master"
    When "alice-browser" types " und verrechnet." at the end of "7.2"
    And "alice-browser" clicks "Apply"
    And "alice-browser" chooses "Selected countries"
    And "alice-browser" checks "Austria"
    And "alice-browser" checks "Netherlands"
    And "alice-browser" unchecks "Germany"
    And "alice-browser" clicks "Confirm"
    Then "alice-browser" sees "Austria has its own version of 7.2, so it was not changed" within "5s"
    And "alice-browser" sees "Change applied to 1 country, effective immediately"
    And "app-db" query "SELECT country, body FROM country_clauses ORDER BY country" returns:
      | country | body                                              |
      | AT      | Österreichische Fassung                           |
      | NL      | Die Zinsen werden jährlich gezahlt und verrechnet. |

  Scenario: Two editors see each other's changes
    Given "alice-browser" opens "/ui/master"
    And "bob-browser" opens "/ui/master"
    And "bob-browser" sees "7.2 Die Zinsen werden jährlich gezahlt" within "5s"
    When "alice-browser" types " Neuer Satz." at the end of "7.2"
    Then "bob-browser" sees "gezahlt Neuer Satz." within "3s"

  Scenario: Undo takes back what was typed
    Given "alice-browser" opens "/ui/master"
    And "alice-browser" types " Neuer Satz." at the end of "7.2"
    And "alice-browser" sees "Neuer Satz."
    When "alice-browser" presses "ctrl+z"
    Then "alice-browser" does not see "Neuer Satz."

  Scenario: A field is filled and searched with the keyboard
    Given "alice-browser" opens "/ui/master"
    And "alice-browser" sees "7.2" within "5s"
    When "alice-browser" fills "Search" with "Zinsen"
    And "alice-browser" presses "Enter"
    Then "alice-browser" sees "2 matches for Zinsen"
    When "alice-browser" fills "Search" with ""
    And "alice-browser" presses "Enter"
    Then "alice-browser" sees "0 matches for"

  Scenario: Typing goes to the field that has focus
    Given "alice-browser" opens "/ui/master"
    When "alice-browser" clicks "Comment"
    And "alice-browser" types "Bitte prüfen"
    Then "alice-browser" sees "Bitte prüfen"

  Scenario: Escape closes the dialog
    Given "alice-browser" opens "/ui/master"
    And "alice-browser" types " x" at the end of "7.2"
    And "alice-browser" clicks "Apply"
    And "alice-browser" sees "Apply change to countries"
    When "alice-browser" presses "Escape"
    Then "alice-browser" does not see "Apply change to countries"

  Scenario: Waiting for a message to go away
    Given "alice-browser" opens "/ui/master"
    When "alice-browser" clicks "Publish"
    Then "alice-browser" sees "Publishing…"
    And "alice-browser" no longer sees "Publishing…" within "3s"
    And "alice-browser" sees "Published"

  Scenario: Following a link
    Given "app-db" table "country_clauses" has values:
      | country | clause_id | body                       |
      | NL      | 7.2       | Die Zinsen werden gezahlt. |
    And "alice-browser" opens "/ui/master"
    When "alice-browser" clicks "History"
    Then "alice-browser" sees "Change history" within "5s"
    And "alice-browser" url is "/ui/history"
    And "alice-browser" sees "NL 7.2: Die Zinsen werden gezahlt."

  Scenario: Each person has their own browser
    Given "alice-browser" opens "/ui/master"
    And "alice-browser" checks "Remember me"
    When "alice-browser" opens "/ui/master"
    Then "alice-browser" sees "Welcome back"
    When "bob-browser" opens "/ui/master"
    Then "bob-browser" sees "7 Zinsen" within "5s"
    And "bob-browser" does not see "Welcome back"

  Scenario: Each scenario starts with a fresh browser
    When "alice-browser" opens "/ui/master"
    Then "alice-browser" sees "7 Zinsen" within "5s"
    And "alice-browser" does not see "Welcome back"

  Scenario: A person's locale and screen size
    Given "alice-browser" opens "/ui/master"
    And "bob-browser" opens "/ui/master"
    Then "alice-browser" sees "Language: de-DE, number: 1.234,5, server heard: de-DE"
    And "alice-browser" sees "Viewport: 1440x900"
    And "bob-browser" sees "number: 1,234.5"
    And "bob-browser" sees "Viewport: 1280x720"

  Scenario: A screenshot of the page
    Given "shell" runs "rm -f .tomato/screenshots/master.png"
    And "alice-browser" opens "/ui/master"
    And "alice-browser" sees "7 Zinsen" within "5s"
    When "alice-browser" saves screenshot "master"
    Then "shell" file ".tomato/screenshots/master.png" exists

  Scenario: A failing step leaves a screenshot of the page
    Given "shell" runs "rm -rf .tomato/browser-fixture"
    When "shell" runs:
      """
      sh tests/testdata/cli/tomato.sh run -c tests/testdata/browser/tomato.yml
      """
    Then "shell" fails
    And "shell" stdout contains "1 scenarios (1 failed)"
    And "shell" stdout contains:
      """
      "visitor-browser" does not see "Order cancelled"; the page shows: "Order confirmed"
      """
    And "shell" stderr contains "screenshot of the page when the step failed"
    And "shell" file ".tomato/browser-fixture/failed-The-order-is-cancelled-visitor-browser.png" exists
