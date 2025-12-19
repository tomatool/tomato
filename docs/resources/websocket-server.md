# WebSocket Server

Steps for stubbing WebSocket services

!!! tip "Multi-line Content"
    Steps ending with `:` accept multi-line content using Gherkin's docstring syntax (`"""`). See examples below each section.


## Setup

| Step | Description |
|------|-------------|
| `"{resource}" on connect sends:` | Sends a message when a client connects |
| `"{resource}" on message "ping" replies:` | Replies to an exact message |
| `"{resource}" on message matching ".*subscribe.*" replies:` | Replies to messages matching a regex pattern |


### Examples

**Sends a message when a client connects:**
```gherkin
"{resource}" on connect sends:
  """
  {"type": "welcome"}
  """
```

**Replies to an exact message:**
```gherkin
"{resource}" on message "ping" replies:
  """
  pong
  """
```

**Replies to messages matching a regex pattern:**
```gherkin
"{resource}" on message matching ".*subscribe.*" replies:
  """
  {"status": "subscribed"}
  """
```


## Broadcast

| Step | Description |
|------|-------------|
| `"{resource}" broadcasts:` | Broadcasts a message to all connected clients |
| `"{resource}" broadcasts "ping"` | Broadcasts a short message to all connected clients |


### Examples

**Broadcasts a message to all connected clients:**
```gherkin
"{resource}" broadcasts:
  """
  {"event": "update"}
  """
```


## Assertions

| Step | Description |
|------|-------------|
| `"{resource}" has "2" connections` | Asserts the number of connected clients |
| `"{resource}" received message "ping"` | Asserts a specific message was received |
| `"{resource}" received "3" messages` | Asserts the total number of messages received |


