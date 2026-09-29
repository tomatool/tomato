# Reviewing a suite

Read `tomato.yml`, the feature files and the CI job. Report each finding with
`file:line`, the rule it breaks and the fix, most severe first.

## Blocking: breaks a hard rule

| Finding | Rule |
|---|---|
| A host, URL, account or credential outside the run: a staging database, a real cloud account, a third-party API, another team's running service | Isolated |
| Harness scripts, wrapper commands, custom images or overlay config files the suite needs to run | Config only |
| A dependency replaced by something that can't fail the way production does: plaintext where production authenticates, a fake where the real server is cheap, runtime flags that differ from production's | Fail like production |
| A mock for a service that belongs to the application itself, or another service's tables living in the application's database | Fail like production |
| A step that reaches the application's internals: calling code, reading its logs, poking its in-memory state | Black box |
| A scenario that goes on after its focus: a second When after the Thens, or assertions on what happens downstream of the action's direct effects | One focus |

## High: can pass while the behavior is broken

- A Then that only restates what a Given seeded.
- No assertion that tells the intended path from a fallthrough.
- Fixtures that make the behavior vacuous: every row owned by everyone, every flag on.
- An incident or bug fixed without a scenario that fails on the broken version.
- Asynchronous effects checked immediately, or after a fixed wait with no comment saying why.

## Medium: flaky or order-dependent

- A scenario that relies on data a migration seeds, or on what an earlier scenario left.
- Tables excluded from reset with no comment saying why, or lock tables that are truncated under a running application.
- Timeouts in milliseconds, or sized for a laptop.
- `app.port` on the application's default port, so a local run can answer in its place.
- Timezone, locale or clock not pinned when outputs depend on them.
- Images on `latest`, or on versions production doesn't run.

## Low: hard to read or maintain

- Scenario names that describe steps rather than behavior.
- Several behaviors in one scenario.
- A feature file without a description of the rule it covers.
- Non-obvious config lines or fixture values without a comment saying why.

## CI

- The job must run on pull requests, build the artifact production runs, and have `pull-requests: write` for the results comment.
- `--no-reset` or `--keep-alive` never belong in CI.
