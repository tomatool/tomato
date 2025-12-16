# WebSocket Client

Steps for connecting to WebSocket servers


## Connection

| Step | Description |
|------|-------------|
| `"ws" connects` | Connect to WebSocket endpoint |
| `"ws" connects with headers:` | Connect with custom headers |
| `"ws" disconnects` | Disconnect from WebSocket |
| `"ws" is connected` | Assert connected |
| `"ws" is disconnected` | Assert disconnected |


## Sending

| Step | Description |
|------|-------------|
| `"ws" sends:` | Send text message (docstring) |
| `"ws" sends "ping"` | Send short text message |
| `"ws" sends json:` | Send JSON message |


## Receiving

| Step | Description |
|------|-------------|
| `"ws" receives within "5s":` | Assert message received within timeout |
| `"ws" receives within "5s" containing "success"` | Assert message containing substring received |
| `"ws" receives json within "5s" matching:` | Assert JSON message matching structure |
| `"ws" receives "3" messages within "10s"` | Assert N messages received within timeout |
| `"ws" does not receive within "2s"` | Assert no message received |


## Assertions

| Step | Description |
|------|-------------|
| `"ws" last message is:` | Assert last message matches exactly |
| `"ws" last message contains "success"` | Assert last message contains substring |
| `"ws" last message is json matching:` | Assert last message is JSON matching structure |
| `"ws" received "5" messages` | Assert total message count |

