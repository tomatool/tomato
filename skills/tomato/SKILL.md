---
name: tomato
description: >
  For teams testing their own service with tomato: set up a suite, write and
  review scenarios, and debug runs. tomato tests an app in isolation through its
  interfaces, against dependencies it starts. Use when adding tomato to a
  repository, writing or reviewing feature files or tomato.yml, when a tomato run
  fails or flakes, or when tomato lacks a step or resource the suite needs. Not
  for changing tomato itself.
license: MIT
compatibility: Needs the tomato CLI (v2) and Docker where the agent runs.
---

# tomato

This skill is for the repository of a service under test. The team that owns the
service writes `tomato.yml` and feature files there, and this skill says how.
Changing tomato itself, such as adding a step or a resource, is a different job
with different rules: the tomato repository has its own skill for that
(`.claude/skills/tomato-dev`) and
[CONTRIBUTING.md](https://github.com/tomatool/tomato/blob/main/CONTRIBUTING.md).

tomato builds an isolated environment for an app: its dependencies run as
containers tomato starts, and the services it calls are mocks tomato serves. It
resets them before every scenario and checks behavior through the app's own
interfaces.

## Principles

Hard rules. A suite that breaks one is wrong even when it is green.

| Rule | Means | So |
|---|---|---|
| Isolated | The suite runs only against what tomato starts: containers, mocks, tomato's STS. | Nothing shared or external: no staging databases, real cloud accounts, third-party APIs or other teams' running services. A host, URL or credential that points outside the run is a bug. |
| Config only | A suite is `tomato.yml`, feature files and a CI workflow. | No harness scripts, wrapper commands or custom images. When tomato can't express something, follow the [gap protocol](references/gaps.md); never work around it in the suite. |
| Fail like production | A scenario must be able to fail the way production fails. | Reproduce the path that matters: auth, runtime flags, broker, schema. A shortcut that can't fail that way proves nothing. |
| Black box | Drive and assert only through what the application exposes or touches: its API, its database, its topics and queues, its buckets, the services it calls. | Never call its internals or assert on its logs. |
| One focus | A scenario is one action and all of its direct effects. Like single responsibility, it has one reason to fail. | Assert every effect on every dependency the action touches, then stop. An endpoint that stores a row, publishes a message and writes an object gets a Then for each; checking one of them passes while the others are broken. What consumes the message, a later change to the record, or how the action behaves when a dependency answers differently are other scenarios. |

"Fail like production" means the same software and wiring, never a real
environment: a real Postgres in a container, not the staging one. tomato's own
defaults follow from these rules: one config is the source of truth, every
scenario starts from reset state, and the application's language doesn't matter.

## Tool constraints

- `tomato steps [--type <resource>] [--filter <word>] [--json]` is the only source of step wording. Never write a step from memory or from prose docs; older examples use wording that no longer exists.
- `tomato validate` before every run, `tomato run [--scenario <regex>] [--tags <expr>]` to run, `tomato coverage` for which steps the features use.
- `--keep-alive` and `--no-reset` are for inspecting a run locally. Never commit them to CI.
- Every run leaves `.tomato/runs/<timestamp>_<id>/` with `tomato.log`, `app.log` and `container-<name>.log`.

## Process

| Job | Read |
|---|---|
| Add tomato to a repository without a suite | [references/onboarding.md](references/onboarding.md) |
| Write or change scenarios | [references/scenarios.md](references/scenarios.md) |
| Review a suite | [references/review.md](references/review.md) |
| A run fails or flakes | [references/debugging.md](references/debugging.md) |
| tomato lacks a step, resource or option the suite needs | [references/gaps.md](references/gaps.md) |

Language-specific extensions, such as a JVM layer, build on this skill and only
add how that runtime is built, started and configured.

## Rules

- Lay features out by the interface they exercise, like `features/http/api/v1/orders/place-order.feature` ([layout](references/scenarios.md#layout)).
- Every fixture and assertion must be able to fail. A scenario that still passes with the behavior removed is wrong.
- Assert the reason for the pass, not a coincidence: add the assertion that proves the intended path was taken.
- Assert every direct effect of the action: the response, the rows, the messages, the objects, the calls to the services it depends on. Then stop: nothing follows the last Then.
- A dependency's answer that changes the outcome, such as the payments mock returning 500, is its own scenario with its own name. Never mix it into the happy path.
- Wait for asynchronous effects with a `within` step. A fixed wait needs a comment saying why no `within` step fits.
- Scenarios own their state: seed what they need in the scenario or its Background, never rely on data a migration inserts or on an earlier scenario.
- Exclude a table from reset only when truncating it breaks the application, such as a lock table, or it holds reference data a migration seeds. Say which in a comment.
- Comment every non-obvious line of `tomato.yml` with why it is there.
- Run changed features at least twice before calling them green. Timing bugs pass once.

## When tomato can't do it

A missing step, resource or option is a gap in tomato, not a reason to bend the
suite. Confirm it is a gap with `tomato steps` and the configuration reference,
write what tomato can express and mark the rest with a comment, file the gap
upstream, and use the fix from its commit until it is released. The exact steps
are in [references/gaps.md](references/gaps.md).

## Output

- Onboarding: `tomato.yml`, the first feature files, the CI workflow, and the `tomato run` summary.
- Scenarios: the changed feature files and the `tomato run` summary.
- Review: findings ordered by severity, each with `file:line`, the rule it breaks and the fix:

  ```text
  high  features/orders.feature:9  checks the 201 but not the stored row, so it
        passes when nothing is saved (can't fail). Add: "db" table "orders" contains:
  ```
- Debugging: the failing step, the evidence from `.tomato/runs`, the cause and the fix.
- Gaps: what is missing, the issue filed upstream, and where the suite marks it.
