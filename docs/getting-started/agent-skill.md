# Agent Skill

tomato ships an [agent skill](https://agentskills.io/) for coding agents such as
Claude Code, in [`skills/tomato`](https://github.com/tomatool/tomato/tree/main/skills/tomato).

## Who it is for

The skill is for the team that owns a service and tests it with tomato. It covers
the four jobs that team has, each in its own reference file the agent reads on
demand:

| Job | The agent gets |
|---|---|
| Onboarding a service | An inventory of the production path, a `tomato.yml`, the first features and the CI job |
| Writing scenarios | Layout by interface, one action per scenario with a Then for every effect, and examples of weak, incomplete, strong and over-long scenarios |
| Reviewing a suite | Findings by severity with `file:line`, the rule broken and the fix |
| Debugging a run | The evidence in `.tomato/runs`, a table of failure signatures, and the tools |

It is one skill rather than four because the jobs share the same hard rules and
run into each other: onboarding ends in writing, writing ends in reviewing, and
a failing run leads back to writing. The rules are tomato's own principles: a
suite runs isolated, it is only `tomato.yml`, feature files and a CI job, it
fails the way production fails, it tests the application as a black box, and
each scenario has one focus and asserts every effect of it.

The skill also has a protocol for when tomato lacks a step, resource or option:
confirm the gap, mark it in the suite instead of working around it, file it
upstream, and use the fix from its commit until a release carries it.

Contributing to tomato itself is a different job with different rules and has
its own skill; see [Contributing to tomato](#contributing-to-tomato).

## Install for Claude Code

For all your projects:

```bash
mkdir -p ~/.claude/skills
curl -fsSL https://github.com/tomatool/tomato/archive/refs/heads/main.tar.gz \
  | tar -xz -C ~/.claude/skills --strip-components=2 tomato-main/skills/tomato
```

For one repository, extract into its `.claude/skills` instead and commit it, so
everyone working on the repository gets it.

Other agents that read agentskills.io skills can use the same directory.

## Using it

The agent needs the tomato CLI and Docker on the machine it runs on, because the
skill makes it take step wording from `tomato steps` and prove its work with
`tomato validate` and `tomato run`. Start the agent in the repository of the
service under test.

The agent picks the skill up on its own when a task matches it. In Claude Code
you can also call it directly with `/tomato`. Either way, say the job in plain
words; the skill routes on it:

| You say | The agent does |
|---|---|
| "Add tomato to this repository" | Inventories every dependency and interface from the code and manifests, writes `tomato.yml`, the first features and the CI job, and runs the suite twice |
| "Write scenarios for placing an order" | Lays the feature out under the interface it exercises, one action per scenario with a Then for every effect, and runs the changed features |
| "Review our tomato suite", or a review of one feature file | Reports findings by severity, each with `file:line`, the rule it breaks and the fix |
| "This tomato run fails", "this scenario flakes on CI" | Reads `.tomato/runs/<run>/`, names the failing step, the evidence, the cause and the fix |
| "tomato has no step for this" | Follows the gap protocol: confirms the gap, marks it in the suite, files it upstream, pins the fix's commit |

What comes back is what the review reference checks, so a reviewer, human or
agent, can hold the result to the same list: no host or credential outside the
run, only config, every scenario one action with every effect asserted, nothing
that can pass while the behavior is broken.

Give the agent what the job needs: for onboarding, how production deploys and
configures the service; for a scenario, the behavior in one sentence and which
dependencies it touches; for a failing run, the run directory or the CI log.

## Contributing to tomato

The tomato repository carries a separate skill for people changing tomato
itself, in [`.claude/skills/tomato-dev`](https://github.com/tomatool/tomato/tree/main/.claude/skills/tomato-dev):
adding or changing steps, resources, presets, options and docs, and handling
gaps that users report. Claude Code loads it automatically when working in a
checkout of tomato, and `/tomato-dev` calls it directly; other agents can be
pointed at the file. It is not part of the user skill and is not installed with
it.

Its one rule that reaches outside this repository: the user skill is the
contract. A change to a step's wording, a config key or a CLI flag updates
`skills/tomato` in the same pull request.

## Extending it

A team can build a skill on top of this one for its own stack, covering how
services are built, started and configured, and keep the principles here.
