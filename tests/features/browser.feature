@browser
Feature: Browser resource
  Two people use the test app's contract editor (tests/app/ui.html) in their
  own browser contexts on one Chromium: one continuous document, a master
  clause applied to selected countries, and the database checked for the
  effect. Steps name what a person sees, never a selector.

  Scenario: The editor shows one document, not the clause structure underneath
    When "alice" opens "/ui/master"
    Then "alice" sees "7 Zinsen" within "5s"
    And "alice" sees "7.2 Die Zinsen werden jährlich gezahlt" within "5s"
    And "alice" does not see "clause_id"
    And "alice" does not see "data-clause"

  Scenario: A master edit reaches only the selected countries
    Given "alice" opens "/ui/master"
    And "alice" sees "Apply" is disabled
    When "alice" types " und verrechnet." at the end of "7.2"
    Then "alice" sees "Apply" is enabled
    When "alice" clicks "Apply"
    And "alice" chooses "Selected countries"
    And "alice" checks "Austria"
    And "alice" checks "Netherlands"
    And "alice" unchecks "Germany"
    And "alice" selects "Next quarter" from "Effective from"
    And "alice" clicks "Confirm"
    Then "alice" sees "Change applied to 2 countries, effective next quarter" within "5s"
    And "alice" sees "Apply" is disabled
    And "db" query "SELECT country, body, effective FROM country_clauses ORDER BY country" returns:
      | country | body                                              | effective    |
      | AT      | Die Zinsen werden jährlich gezahlt und verrechnet. | next-quarter |
      | NL      | Die Zinsen werden jährlich gezahlt und verrechnet. | next-quarter |

  Scenario: A country with its own version of the clause keeps it
    Given "db" table "country_clauses" has values:
      | country | clause_id | body                    | local_override |
      | AT      | 7.2       | Österreichische Fassung | true           |
    And "alice" opens "/ui/master"
    When "alice" types " und verrechnet." at the end of "7.2"
    And "alice" clicks "Apply"
    And "alice" chooses "Selected countries"
    And "alice" checks "Austria"
    And "alice" checks "Netherlands"
    And "alice" unchecks "Germany"
    And "alice" clicks "Confirm"
    Then "alice" sees "Austria has its own version of 7.2, so it was not changed" within "5s"
    And "alice" sees "Change applied to 1 country, effective immediately"
    And "db" query "SELECT country, body FROM country_clauses ORDER BY country" returns:
      | country | body                                              |
      | AT      | Österreichische Fassung                           |
      | NL      | Die Zinsen werden jährlich gezahlt und verrechnet. |

  Scenario: Two editors see each other's changes
    Given "alice" opens "/ui/master"
    And "bob" opens "/ui/master"
    And "bob" sees "7.2 Die Zinsen werden jährlich gezahlt" within "5s"
    When "alice" types " Neuer Satz." at the end of "7.2"
    Then "bob" sees "gezahlt Neuer Satz." within "3s"

  Scenario: Undo takes back what was typed
    Given "alice" opens "/ui/master"
    And "alice" types " Neuer Satz." at the end of "7.2"
    And "alice" sees "Neuer Satz."
    When "alice" presses "ctrl+z"
    Then "alice" does not see "Neuer Satz."

  Scenario: A field is filled and searched with the keyboard
    Given "alice" opens "/ui/master"
    And "alice" sees "7.2" within "5s"
    When "alice" fills "Search" with "Zinsen"
    And "alice" presses "Enter"
    Then "alice" sees "2 matches for Zinsen"
    When "alice" fills "Search" with ""
    And "alice" presses "Enter"
    Then "alice" sees "0 matches for"

  Scenario: Typing goes to the field that has focus
    Given "alice" opens "/ui/master"
    When "alice" clicks "Comment"
    And "alice" types "Bitte prüfen"
    Then "alice" sees "Bitte prüfen"

  Scenario: Escape closes the dialog
    Given "alice" opens "/ui/master"
    And "alice" types " x" at the end of "7.2"
    And "alice" clicks "Apply"
    And "alice" sees "Apply change to countries"
    When "alice" presses "Escape"
    Then "alice" does not see "Apply change to countries"

  Scenario: Waiting for a message to go away
    Given "alice" opens "/ui/master"
    When "alice" clicks "Publish"
    Then "alice" sees "Publishing…"
    And "alice" no longer sees "Publishing…" within "3s"
    And "alice" sees "Published"

  Scenario: Following a link
    Given "db" table "country_clauses" has values:
      | country | clause_id | body                       |
      | NL      | 7.2       | Die Zinsen werden gezahlt. |
    And "alice" opens "/ui/master"
    When "alice" clicks "History"
    Then "alice" sees "Change history" within "5s"
    And "alice" url is "/ui/history"
    And "alice" sees "NL 7.2: Die Zinsen werden gezahlt."

  Scenario: Each person has their own browser
    Given "alice" opens "/ui/master"
    And "alice" checks "Remember me"
    When "alice" opens "/ui/master"
    Then "alice" sees "Welcome back"
    When "bob" opens "/ui/master"
    Then "bob" sees "7 Zinsen" within "5s"
    And "bob" does not see "Welcome back"

  Scenario: Each scenario starts with a fresh browser
    When "alice" opens "/ui/master"
    Then "alice" sees "7 Zinsen" within "5s"
    And "alice" does not see "Welcome back"

  Scenario: A person's locale and screen size
    Given "alice" opens "/ui/master"
    And "bob" opens "/ui/master"
    Then "alice" sees "Language: de-DE, number: 1.234,5, server heard: de-DE"
    And "alice" sees "Viewport: 1440x900"
    And "bob" sees "number: 1,234.5"
    And "bob" sees "Viewport: 1280x720"

  Scenario: A screenshot of the page
    Given "shell" runs "rm -f .tomato/screenshots/master.png"
    And "alice" opens "/ui/master"
    And "alice" sees "7 Zinsen" within "5s"
    When "alice" saves screenshot "master"
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
      "page" does not see "Order cancelled"; the page shows: "Order confirmed"
      """
    And "shell" stderr contains "screenshot of the page when the step failed"
    And "shell" file ".tomato/browser-fixture/failed-The-order-is-cancelled-page.png" exists
