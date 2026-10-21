# Lifecycle

Steps for stopping and starting the app and its containers mid-scenario

!!! tip "Multi-line Content"
    Steps ending with `:` accept multi-line content using Gherkin's docstring syntax (`"""`). See examples below each section.


## App

| Step | Description |
|------|-------------|
| `"lifecycle" restarts the app` | Stop the app and start it again, waiting until it is ready, as a deploy or a crash does |
| `"lifecycle" restarts the app with env:` | Restart the app with these environment variables changed, for the rest of the scenario, e.g. a boundary a few seconds away |
| `"lifecycle" stops the app` | Stop the app, as when it crashes |
| `"lifecycle" starts the app` | Start the stopped app and wait until it is ready |


### Examples

**Restart the app with these environment variables changed, for the rest of the scenario, e.g. a boundary a few seconds away:**
```gherkin
"lifecycle" restarts the app with env:
  | name             | value |
  | SESSION_BOUNDARY | 10s   |
```


## Containers

| Step | Description |
|------|-------------|
| `"lifecycle" stops container "postgres"` | Stop a container, as when the service it runs goes down. It keeps its state and its host ports |
| `"lifecycle" starts container "postgres"` | Start a stopped container on the same address and wait until it is ready |
| `"lifecycle" restarts container "postgres"` | Stop a container and start it again, as a failover does |
| `"lifecycle" pauses container "rabbitmq"` | Freeze a container: connections stay open and nothing answers, as with a hung service or a network partition |
| `"lifecycle" unpauses container "rabbitmq"` | Resume a paused container |


