# Writing scenarios

Get every step's wording from `tomato steps`. The shapes below are what makes a
scenario worth keeping.

## Layout

Feature files mirror the application's interfaces, the way its clients reach
them. The directories say which interface, the file which behavior of it.

```text
features/
├── http/api/v1/
│   ├── orders/
│   │   ├── place-order.feature
│   │   └── cancel-order.feature
│   └── health.feature
├── grpc/directory.v1.DirectoryService/
│   └── list-entities.feature
└── kafka/
    ├── customer.cmd.create-account.v1/
    │   └── creates-account.feature
    └── customer.fct.account-created.v1/
        └── published-from-outbox.feature
```

- The path follows how the interface is addressed: the URL path for HTTP, the package and service for gRPC, the topic or queue for messaging.
- A new API version gets its own tree, so v1 and v2 behavior can differ side by side.
- A scenario lives under the interface its When goes through. Its effects on other interfaces, like the message a POST publishes, are Thens, not a reason to move it.
- Behavior no client triggers, like a scheduled publisher, lives under the interface its effect shows on.

## Shape

- One feature file per behavior of an interface, named after the behavior: `place-order.feature`, not `api-tests.feature`.
- The Feature description states the rule in plain prose, and the fixtures the scenarios rely on.
- A scenario's name is the behavior, as a sentence someone outside the team would understand: "A non-owner sees the query redacted", not "GET change request 403".
- One focus per scenario: one action, all of its direct effects, then stop. Like single responsibility, the scenario has one reason to fail, and its name says what broke.
- Given sets up everything the action needs, even when it uses the application's API to do it. When is the one action, through the application's interface. Then checks every direct effect on every dependency the action touches: the response, the rows it wrote, the messages it published, the objects it stored, the calls it made to the services it depends on. A direct effect with no Then is a hole: the scenario passes while that effect is broken. When tomato has no step for one of them, mark it as a gap ([gaps.md](gaps.md)) instead of leaving it silent.
- Nothing follows the last Then. Effects further downstream are other scenarios: what a consumer does with the published message, a later update, a read of what was created. When the read is the behavior under test, create the record in Given and make the read the When.
- How the action behaves when a dependency answers differently is its own scenario: the payments mock returning 500, the row already existing, the token expired. Set the condition up in Given, keep the When the same action, assert this outcome's effects including the ones that must not happen, and name the scenario after the outcome. Mixed into the happy path, a failure hides which behavior broke.
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
- A negative on an asynchronous effect, like a topic staying empty, is weaker than a positive: it can pass because the message hasn't arrived yet. Check it after the synchronous effects, and say so in a comment.

## Data

- Docstrings for payloads, tables for rows.
- Save values a later step needs, like an id from a response, with a `saved as "{{name}}"` step. Most steps substitute `{{name}}`, not all do: when a scenario relies on it, check once that the value was substituted, and fix tomato if it wasn't.
- Only test credentials in feature files. Comment what a token or magic value stands for.

## Examples

The application under test here takes orders over HTTP, stores them in Postgres,
charges them through a payments service tomato mocks, and announces them on
Kafka. Placing an order touches all four, so a scenario about placing an order
has a Then for each.

Weak: the name says nothing, and it passes if the handler returns 201 without
storing, charging or announcing anything.

```gherkin
Scenario: Test orders endpoint
  When "api" sends "POST" to "/orders"
  Then "api" response status is "201"
```

Incomplete: each Then is right, but placing an order also charges and announces
it, and nothing checks either. It passes while the charge or the publish is
broken. Add the missing Thens, or mark them as gaps if tomato can't express
them; the name must not claim more than the Thens prove.

```gherkin
Scenario: A placed order is stored
  Given "api" json body is:
    """
    {"customer": "customer-7", "sku": "tomato-1", "quantity": 2}
    """
  When "api" sends "POST" to "/orders"
  Then "api" response status is "201"
  And "db" table "orders" contains:
    | customer   | sku      | quantity |
    | customer-7 | tomato-1 | 2        |
```

Strong: the name is the behavior, every dependency the action touches has a
Then that a broken implementation would fail, and it stops at the last direct
effect of the one action.

```gherkin
Scenario: A placed order is stored, charged and announced
  Given "payments" stub "POST" "/charges" returns "201"
  And "events" consumes from "orders.created.v1"
  And "api" json body is:
    """
    {"customer": "customer-7", "sku": "tomato-1", "quantity": 2}
    """
  When "api" sends "POST" to "/orders"
  Then "api" response status is "201"
  And "db" table "orders" contains:
    | customer   | sku      | quantity |
    | customer-7 | tomato-1 | 2        |
  And "payments" received "POST" "/charges"
  # Keyed by customer, so one customer's events stay in order.
  And "events" receives from "orders.created.v1" with key "customer-7" within "10s"
```

Dependency behavior: the same action under a different answer from a dependency
is its own scenario. The condition is in Given, the When is unchanged, and the
Thens are this outcome's effects, including the ones that must not happen. The
check on payments proves the decline is the reason for the rejection, not a
validation error that never reached payments.

```gherkin
Scenario: An order is rejected when the payment is declined
  Given "payments" stub "POST" "/charges" returns "402"
  And "events" consumes from "orders.created.v1"
  And "api" json body is:
    """
    {"customer": "customer-7", "sku": "tomato-1", "quantity": 2}
    """
  When "api" sends "POST" to "/orders"
  Then "api" response status is "402"
  And "payments" received "POST" "/charges"
  And "db" table "orders" is empty
  # Last, after the synchronous effects: a negative on a topic is weaker than a positive.
  And "events" topic "orders.created.v1" is empty
```

Too much: two focuses. When it fails, the name doesn't say which behavior broke,
and cancelling can't be tested unless placing passes first. Split it into
"A placed order is stored, charged and announced" and "Cancelling an order
announces it", with the order seeded in the second one's Given.

```gherkin
Scenario: Order lifecycle
  Given "events" consumes from "orders.created.v1"
  When "api" sends "POST" to "/orders"
  Then "api" response status is "201"
  And "api" response json "id" saved as "{{order_id}}"
  And "events" receives from "orders.created.v1" with key "customer-7" within "10s"
  When "api" sends "POST" to "/orders/{{order_id}}/cancel"
  Then "events" receives from "orders.cancelled.v1" with key "customer-7" within "10s"
```
