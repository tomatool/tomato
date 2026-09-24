---
layout: default
title: Stability
nav_order: 7
---

# Stability and Versioning

tomato v2 is still under active development: the README says "use v2 at your own
risk". This page defines what "stable" will mean, so that promise can be kept
once the [criteria](#dropping-the-at-your-own-risk-warning) below are met, and
so contributors know today which changes need extra care.

## The stable surface

These are the parts of tomato that test suites depend on. Once v2 is declared
stable, they only change in backwards-compatible ways within v2.

| Surface | What is covered |
|---------|-----------------|
| `tomato.yml` schema | Every field documented in the [Configuration Reference](configuration/index.md) under `version: 2`, its meaning and its default |
| Step vocabulary | Every step listed by `tomato steps` (and on the [resource pages](resources/index.md)): its wording, its arguments and what it asserts |
| Resource types | The `type:` names accepted by `tomato validate`, including aliases (`postgresql`, `http-client`, `minio`, …) |
| CLI | Commands `init`, `run`, `validate`, `steps`, `version`, `update`, their flags, and exit codes (0 pass, non-zero fail) |
| Report formats | The console formats and the `junit` and `cucumber` file outputs described under [Reports](configuration/index.md#reports) |
| GitHub Action | Inputs of `tomatool/tomato@v2`: `version`, `config`, `features`, `tags`, `scenario`, `no-reset`, `verbose`, `quiet`, `skip-validate`, `comment` |
| Template variables | `{{.container.host}}`, `{{.container.port.N}}` and `{{.resource.url}}` in `app.env` |

## Not covered

These can change in any release:

- **Go packages.** Everything under `internal/` and the `command` package. tomato
  is a binary, not a library; don't import it.
- **The `tomato` event format** (`--format tomato`, `TOMATO_EVENT:` lines). It exists for
  the GitHub Action and the web UI. It becomes stable only when it is documented
  as a format for users.
- **The web UI** (`tomato ui`) and hidden commands such as `tomato docs`.
- **Console output text**: wording, colours and layout of `pretty` output and error
  messages. Parse the `junit` or `cucumber` outputs instead.
- **Default container images** in `tomato init` templates.

## Versioning

tomato follows [Semantic Versioning](https://semver.org/) for the stable surface:

- **Patch** (`2.1.x`): bug fixes only. A fix may make a step stricter when it was
  wrongly passing; that is called out in the [changelog](https://github.com/tomatool/tomato/blob/main/CHANGELOG.md).
- **Minor** (`2.x.0`): new resources, steps, options and flags. Existing suites keep
  working. Deprecations are announced in minor releases.
- **Major** (`3.0.0`): removals and incompatible changes. A new major also gets a new
  `version:` number in `tomato.yml`, so an old binary never misreads a new config:
  tomato rejects any `version` it doesn't support, with a link to this page.

Until the warning is dropped, minor releases may still contain breaking changes.
Each one is listed under **Changed** or **Removed** in the changelog.

## Deprecation policy

When part of the stable surface has to go:

1. It is marked deprecated in a minor release, with its replacement named in the
   changelog and in the docs.
2. It keeps working for **at least one further minor release**. Deprecated steps log
   a warning the first time they run in a suite (`deprecated step: use … instead`),
   and the generated step docs label them **Deprecated**.
3. It is removed no earlier than the next minor release after that, or in the next
   major release once v2 is stable.

Contributors: mark a step deprecated by setting `Deprecated: "use … instead"` on its
`StepDef`. The step keeps its handler; tomato wraps it to emit the warning.

## Dropping the "at your own risk" warning

The README warning goes away when all of these hold:

- [ ] This policy is published (this page) and has been followed for one minor release.
- [ ] Every item in the stable surface is documented and covered by the integration suite in `tests/features`.
- [ ] CI runs unit tests as well as integration tests, and both pass on `main`.
- [ ] Releases carry changelog entries sorted into Added / Changed / Deprecated / Removed / Fixed.
- [ ] At least two maintainers can review and merge (see [MAINTAINERS.md](https://github.com/tomatool/tomato/blob/main/MAINTAINERS.md)).

Progress on these is tracked in the changelog and the README.

## Migrating from v1

tomato v1 (the [`v1` branch](https://github.com/tomatool/tomato/tree/v1)) no longer
receives updates. v2 is a rewrite, not an upgrade in place:

| v1 | v2 |
|----|----|
| No `version` field | `version: 2` at the top of `tomato.yml` |
| `resources:` is a list of `{name, type, options}` | `resources:` is a map keyed by name: `api: {type: http, …}` |
| Types such as `httpclient`, `http/client`, `queue` + `driver: rabbitmq`, `mysql`, `nsq`, `wiremock` | Types such as `http`, `http-server`, `rabbitmq`, `postgres`; MySQL, NSQ and Wiremock have no v2 equivalent yet (see `tomato validate` for the list) |
| Connection strings (`datasource`) supplied by you, services started by you (e.g. docker-compose) | `containers:` started by tomato; resources point at them with `container:` |
| v1 step wording | v2 step wording: run `tomato steps` |

tomato recognises a v1 file (no `version` and a list under `resources`) and stops
with a pointer to this section instead of a YAML decoding error.
