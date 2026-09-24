# Contributing to tomato

Thanks for helping. Bug reports, docs fixes, new steps and new resources are all
welcome. Before changing anything user-facing, read
[docs/stability.md](docs/stability.md): it says what counts as a breaking change and
how to deprecate things.

## Development setup

You need Go (the version in `go.mod`) and Docker, since tomato starts containers.

```bash
git clone https://github.com/tomatool/tomato.git
cd tomato
make build          # ./bin/tomato
make test           # unit tests (go test -race ./...)
make integration-test  # tomato testing itself against real containers
```

Other targets: `make lint`, `make fmt`, `make vet`, `make tidy`, `make coverage-all`.
Run `make help` for the full list.

The integration suite is `tests/tomato.yml` plus `tests/features/*.feature`. It
starts Postgres, Redis, Kafka, RabbitMQ, MinIO and a small test app from
`tests/app`, on fixed ports (8080 and 9090 among them). Stop anything else using
those ports first; tomato refuses to start the app if its port is taken.

## Adding a step to an existing resource

1. Add a `StepDef` to the resource's `Steps()` in `internal/handler/<resource>.go`:
   `Group`, `Pattern` (use `{resource}` for the resource name), `Description`,
   `Example` and `Handler`.
2. Cover it in `tests/features/<resource>.feature`.
3. Regenerate the docs: `go build -o tomato . && ./tomato docs`. This rewrites
   `docs/resources/`; commit the result.

## Adding a new resource type

1. **Handler.** Create `internal/handler/<name>.go` with a constructor
   `New<Name>(name string, cfg config.Resource, cm *container.Manager) (*<Name>, error)`
   and implement the `Handler` interface in `internal/handler/handler.go`:
   `Name`, `Init`, `Ready`, `Reset`, `RegisterSteps` and `Cleanup`. `Reset` must
   leave the resource clean for the next scenario without destroying things the
   app needs (see the Postgres migration-table exclusions).
2. **Steps.** Implement `Steps() StepCategory` and call
   `RegisterStepsToGodog(ctx, r.name, r.Steps())` from `RegisterSteps`.
3. **Register it once** in `handlerFactories` in `internal/handler/registry.go`. That
   table drives both `tomato run` and `tomato validate`; add the type to
   `ContainerBasedTypes()` too if it normally points at a container.
4. **Docs generator.** Add the handler to `collectStepCategories()` in
   `command/docs.go`, add a page under `docs/configuration/` for its options, and
   add both to the `nav` in `mkdocs.yml`.
5. **Tests.** Unit tests next to the handler, plus a container in `tests/tomato.yml`
   and `tests/features/<name>.feature` exercising every step.
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
in the PR template; it's short. CI builds tomato and runs the integration suite.
Please run `make test` locally as well.

## Reporting bugs

Open an issue with the bug template: tomato version (`tomato version`), your
`tomato.yml` (without secrets), the feature file, and the output with `--verbose`.

## Maintainers

See [MAINTAINERS.md](MAINTAINERS.md) for who reviews and merges, and how to become a
maintainer.
