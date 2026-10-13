# Lifecycle

Steps for stopping and starting the app and its containers mid-scenario

!!! tip "Multi-line Content"
    Steps ending with `:` accept multi-line content using Gherkin's docstring syntax (`"""`). See examples below each section.


## App

| Step | Description |
|------|-------------|
| `"{resource}" restarts the app` | Stop the app and start it again, waiting until it is ready, as a deploy or a crash does |
| `"{resource}" restarts the app with env:` | Restart the app with these environment variables changed, for the rest of the scenario, e.g. a boundary a few seconds away |
| `"{resource}" stops the app` | Stop the app, as when it crashes |
| `"{resource}" starts the app` | Start the stopped app and wait until it is ready |


### Examples

**Restart the app with these environment variables changed, for the rest of the scenario, e.g. a boundary a few seconds away:**
```gherkin
"{resource}" restarts the app with env:
  | name             | value |
  | SESSION_BOUNDARY | 10s   |
```


## Containers

| Step | Description |
|------|-------------|
| `"{resource}" stops container "postgres"` | Stop a container, as when the service it runs goes down. It keeps its state and its host ports |
| `"{resource}" starts container "postgres"` | Start a stopped container on the same address and wait until it is ready |
| `"{resource}" restarts container "postgres"` | Stop a container and start it again, as a failover does |
| `"{resource}" pauses container "rabbitmq"` | Freeze a container: connections stay open and nothing answers, as with a hung service or a network partition |
| `"{resource}" unpauses container "rabbitmq"` | Resume a paused container |


