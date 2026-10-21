# WebSocket Client

Steps for connecting to WebSocket servers

!!! tip "Multi-line Content"
    Steps ending with `:` accept multi-line content using Gherkin's docstring syntax (`"""`). See examples below each section.


## Connection

| Step | Description |
|------|-------------|
| `"app-ws-client" connects` | Connect to WebSocket endpoint |
| `"app-ws-client" connects with headers:` | Connect with custom headers |
| `"app-ws-client" disconnects` | Disconnect from WebSocket |
| `"app-ws-client" is connected` | Assert connected |
| `"app-ws-client" is disconnected` | Assert disconnected |



## Sending

| Step | Description |
|------|-------------|
| `"app-ws-client" sends:` | Send text message (docstring) |
| `"app-ws-client" sends "ping"` | Send short text message |
| `"app-ws-client" sends json:` | Send JSON message |



## Receiving

| Step | Description |
|------|-------------|
| `"app-ws-client" receives within "5s":` | Assert message received within timeout |
| `"app-ws-client" receives within "5s" containing "success"` | Assert message containing substring received |
| `"app-ws-client" receives json within "5s" matching:` | Assert JSON message matching structure |
| `"app-ws-client" receives "3" messages within "10s"` | Assert N messages received within timeout |
| `"app-ws-client" does not receive within "2s"` | Assert no message received |



## Assertions

| Step | Description |
|------|-------------|
| `"app-ws-client" last message is:` | Assert last message matches exactly |
| `"app-ws-client" last message contains "success"` | Assert last message contains substring |
| `"app-ws-client" last message is json matching:` | Assert last message is JSON matching structure |
| `"app-ws-client" received "5" messages` | Assert total message count |


