# When tomato lacks something

A suite stays config only. When it needs a step, resource, option or preset that
tomato doesn't have, that is a gap in tomato, not a reason to bend the suite.
This protocol keeps the suite honest while the gap is open, and gets it closed.

## 1. Confirm it is a gap

- `tomato steps --filter <word>` and `tomato steps --type <resource>`: the step may exist under other words.
- The [configuration reference](https://tomatool.github.io/tomato/configuration/) for resource options and presets.
- Another resource may express it: a `shell` resource runs a CLI the application ships, an `http-server` stands in for any HTTP service the application calls, a `within` step waits for an asynchronous effect.

When none of these fits, it is a gap. Say in one sentence what is missing and
what it has to prove; that sentence becomes the comment and the issue.

## 2. Don't work around it

Never: a script or wrapper around tomato, a custom image, a fixed sleep for a
wait, an assertion on logs, a call into the application's internals, or a weaker
Then standing in for the missing one. Each breaks a hard rule, and each hides
the gap so it never gets fixed while the suite claims more than it proves.

## 3. Mark it in the suite

Write what tomato can express. For the part it can't:

- A missing assertion: leave the Then out and put a comment in its place naming the missing step and the issue, like `# gap: no step asserts the stored object yet, tomatool/tomato#123`. A marked hole is a known gap; an unmarked one is a bug a reviewer reports.
- A scenario that can't be driven at all, because its Given or When needs the missing piece: write it anyway with those steps as comments, tag it `@gap`, and exclude the tag in `features.tags` of `tomato.yml`, so `tomato run` and `tomato coverage` skip it. It records the intent and is ready to enable when the fix ships.

Always link the issue in the comment, so the mark can be removed when it closes.

## 4. File it upstream

Open an issue in [tomatool/tomato](https://github.com/tomatool/tomato/issues)
with the matching template. Include:

- the resource type, what the step or option should do, and the wording you expected;
- the production behavior it has to reproduce, and why a stand-in can't fail the same way;
- the `tomato.yml` and feature snippet, without secrets;
- `tomato version`.

To fix it yourself, follow
[CONTRIBUTING.md](https://github.com/tomatool/tomato/blob/main/CONTRIBUTING.md);
the tomato repository has its own skill for contributors in `.claude/skills/tomato-dev`.

## 5. Use the fix before the release

Once the change is on a commit, `uses: tomatool/tomato@<commit sha>` in the CI
workflow builds tomato from it. Locally, clone the tomato repository at that
commit and `make build`. Fill in the missing step, drop the `@gap` tag and the
comment, and switch back to `@v2` once a release carries the change.
