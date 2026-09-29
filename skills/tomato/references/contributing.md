# When tomato lacks something

A suite stays config only. When it needs something tomato can't express, such as
a step, a resource option, or a preset, add it to tomato, release it, and use
it. Don't work around it with scripts in the service's repository.

## Changing tomato

Follow [CONTRIBUTING.md](https://github.com/tomatool/tomato/blob/main/CONTRIBUTING.md).
A change is ready when:

- Unit tests cover it, and a scenario in `tests/features` uses every new step. CI fails when a step is unused.
- `./tomato docs` regenerated the step reference, and `docs/` covers new options.
- `CHANGELOG.md` has an entry under `## [Unreleased]`.
- `make coverage` passes locally: the integration suite, step coverage and code coverage floors.

## Timing and infrastructure

When a change touches timing or the infrastructure the suite runs on, like a
broker, a database image or startup order, run the affected features several
times before opening the PR. One green CI run proves little about a race.

## Pull requests

- Before pushing to a PR's branch, check it is still open: `gh pr view <n> --json state`. A push after the merge never reaches main.
- Until a release has the change, a suite can run it with `uses: tomatool/tomato@<commit sha>`, which builds tomato from that commit. Switch back to `@v2` once it is released.
