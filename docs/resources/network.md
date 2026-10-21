# Network

Steps for making the network between the app and a dependency slow, unreliable or down. Every fault lasts until the scenario ends, or for the duration given

!!! tip "Multi-line Content"
    Steps ending with `:` accept multi-line content using Gherkin's docstring syntax (`"""`). See examples below each section.


## Faults

| Step | Description |
|------|-------------|
| `"postgres-network" adds latency "300ms" with jitter "100ms" for "10s"` | Delay every response, as a slow or distant dependency does; with jitter, by a random amount either side |
| `"postgres-network" limits bandwidth to "64KB/s"` | Throttle responses to a rate in KB/s or MB/s, as a congested link does |
| `"postgres-network" stops responding for "5s"` | Accept connections and requests but answer nothing, as a hung dependency or a network partition does. The app waits until its own timeout |
| `"postgres-network" resets connections for "3s"` | Reset every connection (TCP RST) as soon as it carries data, as a crash or a failover does |
| `"postgres-network" refuses connections for "5s"` | Close the open connections and refuse new ones, as when the dependency is down. The container keeps running and keeps its data |
| `"postgres-network" flaps every "2s" for "10s"` | Refuse connections and accept them again in turn, each for the period given, as a dependency restarting in a loop does |
| `"postgres-network" fails the next "2" connections` | Close the next new connections as soon as they open, then let them through, as a dependency that is coming back up does. Open connections are untouched |
| `"postgres-network" fails "30%" of new connections` | Close a share of new connections as soon as they open. Which ones is decided by the run's seed, so a failing run can be repeated |
| `"postgres-network" loses "30%" of the data` | Drop a share of the data the dependency sends, chunks of the stream rather than IP packets, so the app sees stalls and a garbled protocol |
| `"postgres-network" is healthy` | Turn every fault off |



## Connections

| Step | Description |
|------|-------------|
| `"postgres-network" has seen "2" connections` | Assert how many connections the app opened through the network since the scenario started, failed ones included: a retry or a reconnect is a new one |
| `"postgres-network" sees a new connection within "10s"` | Wait for the app to open a connection after the network last changed (a fault applied or healed), as an app that reconnects does |


