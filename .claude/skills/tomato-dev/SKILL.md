---
name: tomato-dev
description: >
  For changing tomato itself in this repository: adding or changing steps,
  resources, presets, config options, CLI flags, the GitHub Action or the docs,
  and handling gaps reported by suites that use tomato. Use when working on
  tomato's Go code or its integration suite under tests/. Not for writing a
  service's tomato suite; that is the tomato skill in skills/tomato.
license: MIT
compatibility: Needs Go (the version in go.mod), Docker and make in the tomato repository.
---

# Developing tomato

This skill is for contributors to tomato. Users of tomato, the teams testing
their own service, have a different skill in [`skills/tomato`](../../../skills/tomato/SKILL.md);
that skill is the contract this repository has to keep. Everything user-facing
is in [CONTRIBUTING.md](../../../CONTRIBUTING.md) and
[docs/stability.md](../../../docs/stability.md); this skill is the short version
with the rules that matter for an agent.

## Ground rules

- Steps, config fields, CLI flags and Action inputs are a stable surface. Never rename or remove one in place: add the new one, set `Deprecated: "use … instead"` on the old `StepDef`, document the replacement, and note it under **Deprecated** in the changelog.
- Suites are config only. Anything a user would have to script around is missing from tomato; the fix belongs here, as a step, resource option or preset, not in their repository.
- Step wording is the product. Every step has a `Description` and an `Example`, and `./tomato docs` regenerates `docs/resources/` from them; users and the user skill read wording from `tomato steps`, never from prose.
- When a change alters something the user skill states, such as a step's wording, a config key, the run directory layout or a CLI flag, update `skills/tomato` in the same PR.

## Workflow

```bash
make build              # ./bin/tomato
make test               # unit tests, go test -race ./...
make integration-test   # tomato testing itself against real containers
make coverage           # what CI runs: unit + integration coverage, report and gates
```

The integration suite is `tests/tomato.yml` plus `tests/features/*.feature`. It
starts Postgres, Redis, Kafka, RabbitMQ, MinIO and the test app in `tests/app` on
fixed ports, 8080 and 9090 among them; stop anything else using them first.

| Change | How |
|---|---|
| A step on an existing resource | `StepDef` in `internal/handler/<resource>.go`, a scenario in `tests/features/<resource>.feature`, then `go build -o tomato . && ./tomato docs`. [Details](../../../CONTRIBUTING.md#adding-a-step-to-an-existing-resource) |
| A new resource type | Handler, steps, registry, docs generator, config page, nav, tests, changelog. Agree the config and wording in an issue first. [Details](../../../CONTRIBUTING.md#adding-a-new-resource-type) |
| Changing or removing a step or option | Deprecate, never replace in place. [Details](../../../CONTRIBUTING.md#changing-or-removing-steps-and-options) |

## Definition of done

- Unit tests cover the change, and a scenario in `tests/features` uses every new step. CI fails when any step of any resource type is unused; check with `./bin/tomato coverage -c tests/tomato.yml --all-types`.
- `./tomato docs` regenerated the step reference, and `docs/` covers new options.
- `CHANGELOG.md` has an entry under `## [Unreleased]`.
- `make coverage` passes locally. When coverage rose, raise the floors in `.coverage-min` in the same PR.
- `skills/tomato` still describes what tomato does.

## Timing and infrastructure

When a change touches timing or the infrastructure the suite runs on, like a
broker, a database image or startup order, run the affected features several
times before opening the PR. One green run proves little about a race.

## Handling a gap report

A gap is a user's suite needing a step, resource or option tomato lacks. The
user skill tells them to file it rather than work around it, so treat the report
as the spec:

1. Reproduce it as a scenario in `tests/features` first: the production behavior the user has to reproduce, in the wording they expected.
2. For a new resource type, agree config and wording in the issue before writing code.
3. After the merge, tell the reporter the commit to use: `uses: tomatool/tomato@<sha>` in their workflow builds tomato from it until the release carries the change.

## Pull requests

- Commit messages follow Conventional Commits, as in the history: `feat(postgres): …`, `fix(handler): …`, `docs: …`.
- One feature or fix per PR, with tests and docs. Fill in the PR template.
- Before pushing to a PR's branch, check it is still open: `gh pr view <n> --json state`. A push after the merge never reaches main.
