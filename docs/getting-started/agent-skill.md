# Agent Skill

tomato ships an [agent skill](https://agentskills.io/) for coding agents such as
Claude Code, in [`skills/tomato`](https://github.com/tomatool/tomato/tree/main/skills/tomato).
With it, an agent can add tomato to a repository, write and review scenarios,
and debug failing runs the way this project intends: a suite is only
`tomato.yml`, feature files and a CI job, it fails the way production fails, and
it tests the application as a black box.

The skill takes step wording from `tomato steps`, so the agent needs the tomato
CLI and Docker where it runs.

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
