@postgres
Feature: PostgreSQL reset by schema
  "ledger" resets every table in the billing schema and in any plugin_* schema,
  found when it resets, plus audit.events. billing.fx_rates is excluded and
  billing.schema_migrations is a migration table, so both keep their rows.
  The scenarios run in order: each checks what the reset before it did to the
  rows the one before wrote.

  Scenario: Rows are written to every schema the resource resets
    Given "ledger" executes:
      """
      INSERT INTO billing.invoices (amount) VALUES (100), (200);
      INSERT INTO audit.events (kind) VALUES ('invoice.created');
      CREATE SCHEMA IF NOT EXISTS plugin_42;
      CREATE TABLE IF NOT EXISTS plugin_42.settings (key TEXT PRIMARY KEY, value TEXT NOT NULL);
      INSERT INTO plugin_42.settings VALUES ('theme', 'dark');
      """
    Then "ledger" table "billing.invoices" has "2" rows
    And "ledger" table "audit.events" has "1" rows
    And "ledger" table "plugin_42.settings" has "1" rows

  Scenario: The next scenario starts with those tables empty
    Then "ledger" table "billing.invoices" is empty
    And "ledger" table "audit.events" is empty
    And "ledger" table "plugin_42.settings" is empty

  Scenario: Excluded and migration tables keep their rows
    Then "ledger" table "billing.fx_rates" has "1" rows
    And "ledger" table "billing.schema_migrations" has "1" rows

  Scenario: Identities restart in every reset schema
    Given "ledger" executes:
      """
      INSERT INTO billing.invoices (amount) VALUES (300)
      """
    Then "ledger" query "SELECT id, amount FROM billing.invoices" returns:
      | id | amount |
      | 1  | 300    |
