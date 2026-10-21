# WebSocket Server

Steps for stubbing WebSocket services

!!! tip "Multi-line Content"
    Steps ending with `:` accept multi-line content using Gherkin's docstring syntax (`"""`). See examples below each section.


## Setup

| Step | Description |
|------|-------------|
| `"prices-stub" on connect sends:` | Sends a message when a client connects |
| `"prices-stub" on message "ping" replies:` | Replies to an exact message |
| `"prices-stub" on message matching ".*subscribe.*" replies:` | Replies to messages matching a regex pattern |


### Examples

**Sends a message when a client connects:**
```gherkin
"prices-stub" on connect sends:
  """
  {"type": "welcome"}
  """
```

**Replies to an exact message:**
```gherkin
"prices-stub" on message "ping" replies:
  """
  pong
  """
```

**Replies to messages matching a regex pattern:**
```gherkin
"prices-stub" on message matching ".*subscribe.*" replies:
  """
  {"status": "subscribed"}
  """
```


## Broadcast

| Step | Description |
|------|-------------|
| `"prices-stub" broadcasts:` | Broadcasts a message to all connected clients |
| `"prices-stub" broadcasts "ping"` | Broadcasts a short message to all connected clients |


### Examples

**Broadcasts a message to all connected clients:**
```gherkin
"prices-stub" broadcasts:
  """
  {"event": "update"}
  """
```


## Assertions

| Step | Description |
|------|-------------|
| `"prices-stub" has "2" connections` | Asserts the number of connected clients |
| `"prices-stub" received message "ping"` | Asserts a specific message was received |
| `"prices-stub" received "3" messages` | Asserts the total number of messages received |


