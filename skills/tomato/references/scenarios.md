# Writing scenarios

Get every step's wording from `tomato steps`. The shapes below are what makes a
scenario worth keeping.

## Shape

- One feature file per capability, named after it: `change-request-visibility.feature`, not `api-tests.feature`.
- The Feature description states the rule in plain prose, and the fixtures the scenarios rely on.
- A scenario's name is the behavior, as a sentence someone outside the team would understand: "A non-owner sees the query redacted", not "GET change request 403".
- Given sets state. When is one action through the application's interface. Then checks what shows up on its interfaces: the response, rows, messages, calls to the services it depends on.
- One behavior per scenario. A second When is fine when the behavior is a sequence, such as create then read.
- A Background holds setup every scenario in the file shares. A Scenario Outline runs one behavior over several inputs.
- Put a comment above any step whose purpose isn't obvious, saying what it proves.

## Make it able to fail

- Before writing the Thens, ask what a broken implementation would do, and assert the difference.
- Seed fixtures that tell the cases apart. To test "only owners can read", seed something owned by someone else; without it the scenario passes whatever the code does.
- Assert the reason for the pass. After "the owner can read it", also check the row points at the owned resource, so the pass isn't a fallthrough.
- When a bug or an incident reached production, write the scenario that reproduces it. It must fail on the broken version before it counts.

## Asynchronous effects

- Start listening before acting: consume from a topic or queue in a Given, before the When that makes the application publish.
- Wait with `within` steps (`tomato steps --filter within`): a message received within, a query that returns within, a consumer group consuming within. They return as soon as the condition holds, so a generous timeout costs nothing when the scenario passes.
- Timeouts are seconds, sized for a loaded CI runner, never milliseconds.

## Data

- Docstrings for payloads, tables for rows.
- Save values a later step needs, like an id from a response, with a `saved as "{{name}}"` step. Most steps substitute `{{name}}`, not all do: when a scenario relies on it, check once that the value was substituted, and fix tomato if it wasn't.
- Only test credentials in feature files. Comment what a token or magic value stands for.

## Example

Weak: the name says nothing, and it passes if the handler returns 201 without
storing or announcing anything.

```gherkin
Scenario: Test orders endpoint
  When "api" sends "POST" to "/orders"
  Then "api" response status is "201"
```

Strong: the name is the behavior, and each Then is something a broken
implementation would get wrong.

```gherkin
Scenario: A placed order is stored and announced
  Given "events" consumes from "orders.created.v1"
  And "api" json body is:
    """
    {"customer": "customer-7", "sku": "tomato-1", "quantity": 2}
    """
  When "api" sends "POST" to "/orders"
  Then "api" response status is "201"
  And "db" table "orders" contains:
    | customer   | sku      | quantity |
    | customer-7 | tomato-1 | 2        |
  # Keyed by customer, so one customer's events stay in order.
  And "events" receives from "orders.created.v1" with key "customer-7" within "10s"
```
