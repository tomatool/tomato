---
name: tomato
description: >
  Set up, write, review and debug tomato suites, which test an app through its
  own interfaces against real dependencies. Use when adding tomato to a repo,
  writing scenarios, reviewing a suite, or a run fails.
license: MIT
compatibility: Needs the tomato CLI (v2) and Docker where the agent runs.
---

# tomato

tomato runs an app against its real dependencies in containers, resets them before
every scenario, and checks behavior through the app's own interfaces.

## Principles

Hard rules. A suite that breaks one is wrong even when it is green.

| Rule | Means | So |
|---|---|---|
| Config only | A suite is `tomato.yml`, feature files and a CI workflow. | No harness scripts, wrapper commands or custom images. When tomato can't express something, add it to tomato first ([contributing](references/contributing.md)). |
| Fail like production | A scenario must be able to fail the way production fails. | Reproduce the path that matters: auth, runtime flags, broker, schema. A shortcut that can't fail that way proves nothing. |
| Black box | Drive and assert only through what the application exposes or touches: its API, its database, its topics and queues, the services it calls. | Never call its internals or assert on its logs. |

tomato's own defaults follow from them: one config is the source of truth, every
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
| tomato lacks what the suite needs | [references/contributing.md](references/contributing.md) |

Language-specific extensions, such as a JVM layer, build on this skill and only
add how that runtime is built, started and configured.

## Rules

- Every fixture and assertion must be able to fail. A scenario that still passes with the behavior removed is wrong.
- Assert the reason for the pass, not a coincidence: add the assertion that proves the intended path was taken.
- Wait for asynchronous effects with a `within` step. A fixed wait needs a comment saying why no `within` step fits.
- Scenarios own their state: seed what they need in the scenario or its Background, never rely on data a migration inserts or on an earlier scenario.
- Exclude a table from reset only when truncating it breaks the application, such as a lock table, or it holds reference data a migration seeds. Say which in a comment.
- Comment every non-obvious line of `tomato.yml` with why it is there.
- Run changed features at least twice before calling them green. Timing bugs pass once.

## Output

- Onboarding: `tomato.yml`, the first feature files, the CI workflow, and the `tomato run` summary.
- Scenarios: the changed feature files and the `tomato run` summary.
- Review: findings ordered by severity, each with `file:line`, the rule it breaks and the fix:

  ```text
  high  features/orders.feature:9  checks the 201 but not the stored row, so it
        passes when nothing is saved (can't fail). Add: "db" table "orders" contains:
  ```
- Debugging: the failing step, the evidence from `.tomato/runs`, the cause and the fix.
