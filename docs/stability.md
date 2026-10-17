# Stability and versioning

tomato v2 is the current major version; the `v1` branch no longer receives
updates. This page defines what "stable" means for v2, so that promise can be
kept, and so contributors know which changes need extra care.

## The stable surface

These are the parts of tomato that test suites depend on. Within v2 they change
only in backwards-compatible ways, following the [versioning](#versioning) and
[deprecation](#deprecation-policy) rules below.

| Surface | What is covered |
|---------|-----------------|
| `tomato.yml` schema | Every field documented in the [Configuration Reference](configuration/index.md) under `version: 2`, its meaning and its default |
| Step vocabulary | Every step listed by `tomato steps` (and on the [resource pages](resources/index.md)): its wording, its arguments and what it asserts |
| Resource types | The `type:` names accepted by `tomato validate`, including aliases (`postgresql`, `http-client`, `minio`, …) |
| CLI | Commands `init`, `run`, `validate`, `steps`, `coverage`, `version`, `update`, their flags, and exit codes (0 pass, non-zero fail) |
| Report formats | The console formats and the `junit` and `cucumber` file outputs described under [Reports](configuration/index.md#reports) |
| GitHub Action | Inputs of `tomatool/tomato@v2`: `version`, `config`, `features`, `tags`, `scenario`, `verbose`, `quiet`, `skip-validate`, `comment` |
| Template variables | `{{.container.host}}`, `{{.container.port.N}}` and `{{.resource.url}}` in `app.env`; `{{.resource.address}}`, `{{.resource.host}}` and `{{.resource.port}}` for a `grpc-server` |

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

A breaking change that still lands in a minor release is marked **Breaking** under
**Changed** or **Removed** in the changelog.

## Deprecation policy

When part of the stable surface has to go:

1. It is marked deprecated in a minor release, with its replacement named in the
   changelog and in the docs.
2. It keeps working for **at least one further minor release**. Deprecated steps log
   a warning the first time they run in a suite (`deprecated step: use … instead`),
   and the generated step docs label them **Deprecated**.
3. It is removed no earlier than the next minor release after that, or in the next
   major release.

Contributors: mark a step deprecated by setting `Deprecated: "use … instead"` on its
`StepDef`. The step keeps its handler; tomato wraps it to emit the warning.

## The "at your own risk" warning

Until October 2026 the README opened with a warning that v2 was under active
development and should be used at your own risk. It was removed on 13 October
2026: v2 is the version tomato ships and the `v1` branch is kept only for
reference, so the stable surface above is now held to the versioning and
deprecation rules on this page.

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
