# Adding a step or a resource

This page is for people changing tomato itself. For writing a suite with
tomato, start at [Getting started](../getting-started/index.md).

Two rules shape everything below. Steps, config fields, CLI flags and Action
inputs are a **stable surface**: never rename or remove one in place. And
**step wording is the product** — users read it from `tomato steps`, and
`tomato docs` generates the reference from the same definitions, so the
wording you write is the documentation.

## Adding a step to a resource

1. Add a `StepDef` to the resource's `Steps()` in `internal/resource/<name>/`,
   with `Group`, `Pattern` (use `{resource}` where the resource name goes),
   `Description`, `Example` and `Handler`.
2. Use it in a scenario in `tests/features/<resource>.feature`. CI fails when
   any step of any resource type is unused — check locally with:

    ```bash
    ./bin/tomato coverage -c tests/tomato.yml --all-types
    ```

3. Regenerate the reference and commit the result:

    ```bash
    go build -o tomato . && ./tomato docs
    ```

The `Example` is not decoration. It appears in the generated page, and
`TestDocsExamplesUseRealSteps` checks every Gherkin snippet in `docs/` against
the live patterns, so an example that does not parse as a real step fails CI.

## Adding a resource type

1. **The handler.** Create `internal/resource/<name>/<name>.go` with

    ```go
    func New(name string, cfg config.Resource, cm *container.Manager) (*Name, error)
    ```

    implementing `resource.Handler`: `Name`, `Init`, `Ready`, `Reset`,
    `RegisterSteps`, `Cleanup`. The package imports `internal/resource` and
    **nothing from another resource** — `TestResourcesDoNotImportEachOther`
    enforces that. If two resources need the same helper, it belongs in
    `internal/resource` or `internal/jsonmatch`.

2. **Reset.** `Reset` runs before every scenario and has to leave the resource
   genuinely clean, including state that survives the obvious command —
   Postgres truncates with `RESTART IDENTITY` because plain `TRUNCATE` leaves
   sequences advanced, which made scenarios order-dependent. Exclude only what
   the application needs to exist, such as migration tables.

3. **Steps.** Implement `Steps() resource.StepCategory` and call
   `resource.RegisterStepsToGodog(ctx, r.name, r.Steps())` from
   `RegisterSteps`.

4. **Register it once**, in `internal/registry/registry.go`: add it to
   `handlerFactories` and to `canonicalTypes`. The first makes it buildable by
   `tomato run` and accepted by `tomato validate`; the second gives it a
   documentation page and a place in `tomato steps`. Two tests pin the lists
   against each other, so missing one fails CI. Add it to
   `ContainerBasedTypes()` if it normally points at a container.

5. **Documentation.** Add a page under `docs/configuration/` for its options,
   and add it and the generated `docs/resources/` page to the `nav` in
   `mkdocs.yml`. The step reference generates itself.

6. **Tests.** Unit tests in the resource's own package, a container in
   `tests/tomato.yml`, and `tests/features/<name>.feature` exercising every
   step.

7. **Changelog.** A line under `## [Unreleased]`.

Agree the config shape and the step wording in an issue before writing the
code. Both are a stable surface from the moment they ship.

## Changing or removing a step or an option

Add the replacement, then mark the old one:

```go
{
    Pattern:    `^"{resource}" old wording$`,
    Deprecated: "use the new wording instead",
    Handler:    r.handler,
}
```

A deprecated step keeps working and warns once per run, and the generated
reference labels it. Note the replacement under **Deprecated** in the
changelog. See [Stability](../stability.md) for how long it has to stay.

## Before opening the pull request

- `make coverage` passes. It is what CI runs: unit and integration coverage
  merged, every resource step used by a scenario, and the floors in
  `.coverage-min`. When coverage rises, raise the floors in the same PR.
- `./tomato docs` has been run and the result committed.
- If the change touches timing or the infrastructure the suite runs on — a
  broker, a database image, startup order, a reset, anything concurrent — run
  the affected features several times. One green run proves very little about
  a race.

## See also

- [Architecture](architecture.md) — what is in the repository, and which parts CI checks
- [Stability](../stability.md) — what counts as a breaking change
- [CONTRIBUTING.md](https://github.com/tomatool/tomato/blob/main/CONTRIBUTING.md) — setup, commits and review
