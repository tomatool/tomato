# Step Coverage

`tomato coverage` shows which steps of your resources your feature files
actually use. It reads `tomato.yml` and every feature file, expands Scenario
Outlines, skips scenarios that `features.tags` excludes, and matches each step
against the step definitions of the resources you declared.

```bash
tomato coverage
```

```
Step coverage: 41/52 (78.8%)

  http                20/36    55.6%
      missing: ^"<resource>" response header "([^"]*)" is "([^"]*)"$
      ...
  postgres            10/10   100.0%
  kafka               11/26    42.3%
```

| Flag | Description |
|------|-------------|
| `-c`, `--config` | Config file (default `tomato.yml`) |
| `--format` | `text` (default), `markdown` (a table for PR comments) or `json` |
| `--min` | Exit non-zero if any resource type is below this percentage |
| `--all-types` | Include every resource type tomato supports, not only those in the config |
| `-o`, `--output` | Write the report to a file |

Steps that match no resource are listed under `unmatched_steps` in the JSON
output, which catches typos in step text before `tomato run` does.

Use it in CI to keep a suite from quietly losing coverage:

```yaml
- run: tomato coverage --min 80 --format markdown -o coverage.md
```

Coverage here means "used by at least one scenario". It says nothing about
whether the scenarios pass; `tomato run` does that.
