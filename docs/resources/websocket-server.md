# WebSocket Server

Steps for stubbing WebSocket services

!!! tip "Multi-line Content"
    Steps ending with `:` accept multi-line content using Gherkin's docstring syntax (`"""`). See examples below each section.


## Setup

| Step | Description |
|------|-------------|
| `"{resource}" on connect sends:` | Sends a message when a client connects |
| `"{resource}" on message "{message}" replies:` | Replies to an exact message |
| `"{resource}" on message matching "{pattern}" replies:` | Replies to messages matching a regex pattern |

### Examples

**Send welcome message on connect:**
```gherkin
Given "ws-mock" on connect sends:
  """
  {"type": "welcome", "version": "1.0"}
  """
```

**Reply to exact message:**
```gherkin
Given "ws-mock" on message "ping" replies:
  """
  pong
  """
```

**Reply to pattern:**
```gherkin
Given "ws-mock" on message matching ".*subscribe.*" replies:
  """
  {"status": "subscribed"}
  """
```


## Broadcast

| Step | Description |
|------|-------------|
| `"{resource}" broadcasts:` | Broadcasts a message to all connected clients |
| `"{resource}" broadcasts "{message}"` | Broadcasts a short message to all connected clients |

### Examples

**Broadcast message:**
```gherkin
When "ws-mock" broadcasts:
  """
  {"event": "update", "data": {"id": 1}}
  """
```

**Broadcast short message:**
```gherkin
When "ws-mock" broadcasts "ping"
```


## Assertions

| Step | Description |
|------|-------------|
| `"{resource}" has "{n}" connections` | Asserts the number of connected clients |
| `"{resource}" received message "{message}"` | Asserts a specific message was received |
| `"{resource}" received "{n}" messages` | Asserts the total number of messages received |
