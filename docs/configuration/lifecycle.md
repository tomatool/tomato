# Lifecycle configuration

The `lifecycle` resource takes the application or one of its dependencies down
in the middle of a scenario, the way production fails: the app restarts under
connected clients, a database restarts, a broker hangs. The scenario then
drives the app and checks that it recovers.

```yaml
resources:
  lifecycle:
    type: lifecycle
```

It has no options. The steps name the containers from `containers:` in
`tomato.yml`.

```gherkin
Scenario: The app keeps its orders across a database failover
  Given "orders-client" sends "POST" to "/orders" with json:
    """
    {"id": "o-1"}
    """
  When "lifecycle" restarts container "postgres"
  And "orders-client" sends "GET" to "/orders/o-1"
  Then "orders-client" response status is "200"
```

## What each step does

| Step | Production failure | What tomato does |
|------|--------------------|------------------|
| `restarts the app` | a deploy or a crash | stops the app process and starts it again, waiting for its ready check |
| `restarts the app with env:` | a changed setting | the same, with these variables changed for the rest of the scenario |
| `stops the app` / `starts the app` | the app is down for a while | stops it, and later starts it and waits for its ready check |
| `stops container` / `starts container` | a dependency goes down | stops the container, keeping its data, and starts it again on the **same host port** |
| `restarts container` | a failover | stop, then start |
| `pauses container` / `unpauses container` | a hung dependency, a network partition | freezes its processes: connections stay open and nothing answers |

## Every scenario starts with everything up

Before each scenario, the lifecycle resource starts the containers the last
scenario stopped, unpauses the ones it paused, and restarts the app with its
configured environment if the scenario stopped it or changed its environment.
This runs before any other resource resets, so a postgres reset never meets a
stopped database.

## Same address after a restart

Docker gives a container a new host port every time it starts, so a database
that stops and starts would come back where the app no longer looks. With a
local Docker, tomato binds each dynamic container port to a free host port it
picks itself, and the container keeps that address for the whole run. A remote
Docker host (`DOCKER_HOST=tcp://...`) keeps Docker's own allocation.

## Limits

- Stopping and starting the app needs `app.command`: an app run as a container
  is removed when it stops and comes back on another port.
- Data a container keeps in memory only (Redis without persistence, say) is
  gone after a restart, as it would be in production.
