# Contributing to tomato

Thanks for helping. Bug reports, docs fixes, new steps and new resources are all
welcome. Before changing anything user-facing, read
[docs/stability.md](docs/stability.md): it says what counts as a breaking change and
how to deprecate things.

## Ground rules

- **A suite is configuration, not code.** Anything a user would have to script
  around is something tomato is missing. The fix belongs here — as a step, a
  resource option or a preset — not in their repository.
- **Step wording is the product.** Every step carries a `Description` and an
  `Example`, and `tomato docs` regenerates `docs/resources/` from them. Users
  read wording from `tomato steps`, never from prose.

## Where things are

[Architecture](docs/developer-guide/architecture.md) opens with a table of
every top-level directory: which part is the Go program, which parts ship
besides the binary, which files are generated rather than edited, and which
of them CI actually checks. Worth two minutes before your first change.

## Development setup

You need Go (the version in `go.mod`) and Docker, since tomato starts
containers. Changing the web UI additionally needs Node and pnpm; everything
else builds without them.

```bash
git clone https://github.com/tomatool/tomato.git
cd tomato
make build          # ./bin/tomato
make test           # unit tests (go test -race ./...)
make integration-test  # tomato testing itself against real containers
make coverage       # what CI runs: unit + integration coverage, report and gates
```

Other targets: `make lint`, `make fmt`, `make vet`, `make tidy`.
Run `make help` for the full list.

### When a change touches timing

If a change touches timing or the infrastructure the suite runs on — a broker,
a database image, startup order, a reset, anything concurrent — run the
affected features several times before opening the pull request. One green run
proves very little about a race, and tomato's whole claim is that a run means
something.

## Changing the web UI

`tomato ui` is a React app under `ui/`, bundled by Vite into
`command/ui_assets/dist`, which the binary embeds.

```bash
make ui        # build the bundle
make ui-dev    # Vite dev server, proxied at a `tomato ui` on :7788
make ui-check  # fail if the committed bundle is stale
```

`make build` rebuilds the bundle itself when anything under `ui/` is newer, and
warns and carries on with the committed bundle when pnpm is missing.

**Commit `command/ui_assets/dist` with any UI change.** Go cannot run pnpm, so
without the committed bundle `go build`, `go install` and `go test` would need
Node. CI runs `make ui-check` and fails when it is stale, the same arrangement
the kafka preset's jar uses. See [ui/README.md](ui/README.md).

The integration suite is `tests/tomato.yml` plus `tests/features/*.feature`. It
starts Postgres, Redis, Kafka, RabbitMQ, MinIO, ScyllaDB and a small test app
from `tests/app`, on fixed ports (8080 and 9090 among them). Stop anything else
using those ports first; tomato refuses to start the app if its port is taken.

tomato names everything it creates `tomato-<project>-<run>-<name>` and labels
it, so a run that died without cleaning up leaves findable debris:

```bash
docker rm -f $(docker ps -aq --filter label=tomato.managed=true)
```

## Adding a step or a resource

The walkthrough lives in the documentation, so it sits beside the architecture
it refers to: [Adding a Step or Resource](docs/developer-guide/adding-a-resource.md).
It covers both, plus deprecating a step or option rather than replacing it in
place, and what has to pass before you open the pull request.

## Commits and pull requests

Commit messages follow [Conventional Commits](https://www.conventionalcommits.org/),
as in the existing history:

```
feat(postgres): add query result assertion steps
fix(handler): register grpc in ValidResourceTypes
docs: document report formats
chore(deps): bump go.opentelemetry.io/otel/sdk
```

Keep pull requests focused: one feature or fix per PR, with tests and docs. Fill
in the PR template; it's short.

CI runs `make coverage` and posts the result as a comment on the pull request.
It fails when:

- a scenario in the integration suite fails,
- any step of any resource type is not used by at least one scenario, or
- Go statement coverage (unit + integration, merged) drops below the floors in
  `.coverage-min`: `resource` for the resource code in `internal/resource`,
  `total` for the whole module.

When a change raises coverage, raise the floors in `.coverage-min` in the same
PR, so coverage only goes up. Please run `make test` locally as well.

## Handling a gap report

A gap is a user's suite needing a step, resource or option tomato does not
have. Reporting it beats working around it — a suite is configuration, not
code — so treat the report as the spec:

1. Reproduce it as a scenario in `tests/features` first — the production
   behaviour they have to cover, in the wording they expected.
2. For a new resource type, agree the config and the wording in the issue
   before writing code.
3. After the merge, tell the reporter which commit to use:
   `uses: tomatool/tomato@<sha>` builds tomato from it until a release carries
   the change.

## Reporting bugs

Open an issue with the bug template: tomato version (`tomato version`), your
`tomato.yml` (without secrets), the feature file, and the output with `--verbose`.

## Maintainers

See [MAINTAINERS.md](MAINTAINERS.md) for who reviews and merges, and how to become a
maintainer.
