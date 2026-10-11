# Debugging a run

Read the evidence before changing anything. Every run writes
`.tomato/runs/<timestamp>_<id>/`:

| File | Holds |
|---|---|
| `tomato.log` | tomato's own output: startup, each step, the failure |
| `app.log` | the application's output, in command mode |
| `container-<name>.log` | each container's output, for as long as it ran |

## Signatures

| Symptom | Likely cause | Next step |
|---|---|---|
| Startup refuses because `app.port` is in use | Another process holds the port, often a local run | Stop it or move `app.port` |
| A container exits while tomato waits for it | Its configuration is wrong | Read `container-<name>.log` |
| `{{.name.port.N}}` stays literal in the application's config | The container or port in the template doesn't exist | Fix the name or port |
| A step matches no definition | The wording is wrong | `tomato steps --filter <word>` |
| A `within` step times out | The application never produced the effect | Read `app.log` for the error, and check the application points at the right endpoint |
| Passes alone, fails in the suite | State leaks between scenarios | Check reset exclusions and order dependence |
| Fails sometimes, mostly on CI | A race: something used before it is ready | Rerun the feature several times, then fix the wait in the suite or in tomato |
| The application authenticates as the wrong identity | Its credential chain fell back | Check the `aws` resource's role, and assert it was assumed |

## Tools

- `tomato run --scenario "<regex>"` runs one scenario.
- `tomato run --keep-alive` leaves the containers running after the run and prints how to reach them.
- Reset cannot be switched off. To look at the state a scenario left behind, run that one scenario with `tomato run --scenario '<regex>' --keep-alive`: nothing resets after it, and the containers stay up. Never commit `--keep-alive`.

When the failure turns out to be tomato's, like a race in a step or something it
can't express, follow the [gap protocol](gaps.md) rather than working around
it in the suite.
