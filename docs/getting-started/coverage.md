# Test coverage

The coverage worth tracking is **which lines of your service ran while the
scenarios ran**. tomato can't see that: it only talks to your service over the
network. Your service can. Every mainstream language can measure a running
process and write the result when it exits, and tomato starts your service
with its environment and stops it cleanly, so turning that on is all it takes.

| Language | Turn it on | Read it |
|---|---|---|
| Go | build with `go build -cover`, set `GOCOVERDIR` | `go tool covdata percent` |
| Rust | build with `RUSTFLAGS="-C instrument-coverage"`, set `LLVM_PROFILE_FILE` | `llvm-profdata`, `llvm-cov` |
| Java, Kotlin, Scala | the JaCoCo agent, through `JAVA_OPTS` or `JAVA_TOOL_OPTIONS` | `jacococli report` |
| Python | coverage.py, started by `COVERAGE_PROCESS_START` | `coverage combine`, `coverage report` |
| Node.js | set `NODE_V8_COVERAGE` | `c8 report` |

Set the variable on the `tomato run` step, or in your shell: tomato passes its
environment to the service, so `tomato.yml` stays the same with or without
coverage.

The one requirement: **your service exits cleanly on SIGTERM, within five
seconds.** That is how tomato stops it, and coverage is written on exit. A
service killed instead writes nothing.

[Code coverage for black-box tests](https://tomatool.github.io/blog/posts/code-coverage-for-black-box-tests/)
walks through each language, with the commands, a CI job, and what the number
does and doesn't tell you.

## `tomato coverage`: a lint for the suite

`tomato coverage` is something else: it reports which of tomato's **steps**
your feature files use. That says how the suite is worded, not what it tests,
so use it as a lint, not a metric:

```bash
tomato coverage --format json | jq '.unmatched_steps'
```

- **`unmatched_steps`** lists steps that match no step definition, usually a
  typo, before `tomato run` finds it.
- **A declared resource with no steps used** is a resource no scenario checks:
  a Kafka topic nobody asserts on, say.

Don't gate CI on its percentage. Using ten of the HTTP client's fifty-odd
steps can be a complete suite, and the percentage drops whenever tomato adds
steps.

| Flag | Description |
|------|-------------|
| `-c`, `--config` | Config file (default `tomato.yml`) |
| `--format` | `text` (default), `markdown` or `json` |
| `--all-types` | Include every resource type tomato supports, not only those in the config |
| `-o`, `--output` | Write the report to a file |
