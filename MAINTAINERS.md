# Maintainers

| Name | GitHub | Role |
|------|--------|------|
| Ali Reza Yahya | [@alileza](https://github.com/alileza) | Lead maintainer |

## Current situation

tomato has one active maintainer. That is a risk for anyone depending on it:
reviews, releases and security fixes all wait on one person. Adding at least
one more maintainer is one of the criteria for declaring v2 stable (see
[docs/stability.md](docs/stability.md#dropping-the-at-your-own-risk-warning)).

## What maintainers do

- Review and merge pull requests, and triage issues (label, reproduce, close duplicates).
- Keep `main` releasable: CI green, changelog up to date.
- Cut releases and uphold the versioning and deprecation policy.
- Look after security reports and dependency updates.

## Becoming a maintainer

There is no formal process beyond this:

1. Contribute: several merged pull requests (code, tests or docs) and some issue
   triage or review on other people's pull requests.
2. Ask. Open an issue or reach out to an existing maintainer.
3. An existing maintainer agrees, adds you to this file and to
   `.github/CODEOWNERS`, and grants write access.

Teams that run tomato in their own CI are especially welcome: you know which
parts of the stable surface matter in practice.

## Stepping down

Maintainers can step down at any time by opening a pull request that moves them to
an "Emeritus" section of this file.
