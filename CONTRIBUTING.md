# Contributing to tomato

Thanks for helping. Bug reports, docs fixes, new steps and new resources are all
welcome. Before changing anything user-facing, read
[docs/stability.md](docs/stability.md): it says what counts as a breaking change and
how to deprecate things.

With Claude Code, the contributor skill in `.claude/skills/tomato-dev` loads
automatically in this repository. The user-facing skill in `skills/tomato` is
the contract to keep when you change steps, config or CLI flags.

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

## Adding a step to an existing resource

1. Add a `StepDef` to the resource's `Steps()` in `internal/resource/<name>/`:
   `Group`, `Pattern` (use `{resource}` for the resource name), `Description`,
   `Example` and `Handler`.
2. Cover it in `tests/features/<resource>.feature`. CI fails if any step of any
   resource type is not used by a scenario; check with
   `./bin/tomato coverage -c tests/tomato.yml --all-types`.
3. Regenerate the docs: `go build -o tomato . && ./tomato docs`. This rewrites
   `docs/resources/`; commit the result.

## Adding a new resource type

1. **Handler.** Create `internal/resource/<name>/<name>.go` with a constructor
   `New(name string, cfg config.Resource, cm *container.Manager) (*<Name>, error)`
   and implement the `resource.Handler` interface from `internal/resource`:
   `Name`, `Init`, `Ready`, `Reset`, `RegisterSteps` and `Cleanup`. `Reset` must
   leave the resource clean for the next scenario without destroying things the
   app needs (see the Postgres migration-table exclusions).
2. **Steps.** Implement `Steps() StepCategory` and call
   `RegisterStepsToGodog(ctx, r.name, r.Steps())` from `RegisterSteps`.
3. **Register it once** in `handlerFactories` in `internal/registry/registry.go`,
   and add it to `canonicalTypes` in the same file so it gets a docs page and
   shows up in `tomato steps`. That table drives `tomato run`, `tomato validate`
   and `tomato docs` alike; add the type to `ContainerBasedTypes()` too if it
   normally points at a container.
4. **Docs.** Add a page under `docs/configuration/` for its options and add both
   it and the generated `docs/resources/` page to the `nav` in `mkdocs.yml`. The
   step reference generates itself from the registry.
5. **Tests.** Unit tests in the resource's own package, plus a container in
   `tests/tomato.yml` and `tests/features/<name>.feature` exercising every step.
6. **Changelog.** Add a line under `## [Unreleased]` in `CHANGELOG.md`.

A new resource type is a good thing to discuss in an issue first (there's a
template for it), so the config and step wording can be agreed before you write
the code.

## Changing or removing steps and options

Steps, config fields, CLI flags and Action inputs are part of the stable surface.
Don't rename or remove them in place. Add the new one, set `Deprecated: "use … instead"`
on the old `StepDef` (it keeps working and warns once per run), document the
replacement, and note it under **Deprecated** in the changelog.

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

## Reporting bugs

Open an issue with the bug template: tomato version (`tomato version`), your
`tomato.yml` (without secrets), the feature file, and the output with `--verbose`.

## Maintainers

See [MAINTAINERS.md](MAINTAINERS.md) for who reviews and merges, and how to become a
maintainer.
