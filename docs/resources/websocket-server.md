# WebSocket Server

Steps for stubbing WebSocket services


## Setup

| Step | Description |
|------|-------------|
| `"{resource}" on connect sends:
  """
  {"type": "welcome"}
  """` | Sends a message when a client connects |
| `"{resource}" on message "ping" replies:
  """
  pong
  """` | Replies to an exact message |
| `"{resource}" on message matching ".*subscribe.*" replies:
  """
  {"status": "subscribed"}
  """` | Replies to messages matching a regex pattern |


## Broadcast

| Step | Description |
|------|-------------|
| `"{resource}" broadcasts:
  """
  {"event": "update"}
  """` | Broadcasts a message to all connected clients |
| `"{resource}" broadcasts "ping"` | Broadcasts a short message to all connected clients |


## Assertions

| Step | Description |
|------|-------------|
| `"{resource}" has "2" connections` | Asserts the number of connected clients |
| `"{resource}" received message "ping"` | Asserts a specific message was received |
| `"{resource}" received "3" messages` | Asserts the total number of messages received |

