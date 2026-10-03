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

Step wording always comes from `tomato steps`, so the agent needs the tomato CLI
and Docker where it runs.

Contributing to tomato itself is a different job with different rules. The
tomato repository carries a separate skill for that in `.claude/skills/tomato-dev`,
which Claude Code loads automatically when working in that repository. It is not
part of the user skill and is not installed with it.

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

## Extending it

A team can build a skill on top of this one for its own stack, covering how
services are built, started and configured, and keep the principles here.
