# gRPC Server

Steps for stubbing the gRPC services the application calls

!!! tip "Multi-line Content"
    Steps ending with `:` accept multi-line content using Gherkin's docstring syntax (`"""`). See examples below each section.


## Stub Setup

| Step | Description |
|------|-------------|
| `"{resource}" stub "users.v1.UserService/GetUser" with request json "id" "42" returns json:` | Creates a stub that answers a method with this response message, written as JSON (protojson). Between the method and "returns", `with metadata "k" "v"` and `with request json "field" "value"` (a path into the request, with the same @ matchers as json assertions) narrow which calls it answers; the most specific stub wins, then the most recent. On a server-streaming method the message is a stream of one |
| `"{resource}" stub "users.v1.UserService/GetUser" returns status "NOT_FOUND" with message "no user 42"` | Creates a stub that fails a method with a gRPC status (NOT_FOUND, PERMISSION_DENIED, UNAVAILABLE, ...) and an optional message |
| `"{resource}" stub "users.v1.UserService/GetUser" returns in sequence:` | Creates a stub that answers each call with the next row, and repeats the last row once the rows are used up. Columns: json (the response message), status (default OK) and message for a failure, and delay (e.g. 3s) to answer late, as a slow dependency does; a client whose deadline passes first gets DEADLINE_EXCEEDED |
| `"{resource}" stub "users.v1.UserService/WatchUsers" streams:` | Creates a stub that answers a server-streaming method with one message per row, in order, then ends the stream. Columns: json, and delay before a message; a last row with a status (and message) instead of json ends the stream with that status |


### Examples

**Creates a stub that answers a method with this response message, written as JSON (protojson). Between the method and "returns", `with metadata "k" "v"` and `with request json "field" "value"` (a path into the request, with the same @ matchers as json assertions) narrow which calls it answers; the most specific stub wins, then the most recent. On a server-streaming method the message is a stream of one:**
```gherkin
"{resource}" stub "users.v1.UserService/GetUser" with request json "id" "42" returns json:
  """
  {"id": "42", "name": "Alice"}
  """
```

**Creates a stub that answers each call with the next row, and repeats the last row once the rows are used up. Columns: json (the response message), status (default OK) and message for a failure, and delay (e.g. 3s) to answer late, as a slow dependency does; a client whose deadline passes first gets DEADLINE_EXCEEDED:**
```gherkin
"{resource}" stub "users.v1.UserService/GetUser" returns in sequence:
  | status      | json                         | delay |
  | UNAVAILABLE |                              |       |
  | OK          | {"id": "42"}                 | 2s    |
```

**Creates a stub that answers a server-streaming method with one message per row, in order, then ends the stream. Columns: json, and delay before a message; a last row with a status (and message) instead of json ends the stream with that status:**
```gherkin
"{resource}" stub "users.v1.UserService/WatchUsers" streams:
  | json                 | delay |
  | {"id": "1"}          |       |
  | {"id": "2"}          | 100ms |
```


## Availability

| Step | Description |
|------|-------------|
| `"{resource}" refuses connections` | Close the server's port and its open connections, so the app's calls fail with UNAVAILABLE as when the service is down. Calls already in flight finish |
| `"{resource}" accepts connections` | Open the port again on the same address, with the stubs kept. Every scenario starts accepting |



## Verification

| Step | Description |
|------|-------------|
| `"{resource}" received "users.v1.UserService/GetUser"` | Asserts a method was called |
| `"{resource}" received "users.v1.UserService/GetUser" "2" times` | Asserts a method was called N times |
| `"{resource}" received "users.v1.UserService/GetUser" with metadata "authorization" "Bearer {{token}}"` | Asserts a method was called with this exact metadata value, e.g. the user's token the app forwards |
| `"{resource}" did not receive "users.v1.UserService/DeleteUser"` | Asserts a method was not called |
| `"{resource}" received "3" calls` | Asserts the total number of calls received, to any method |



## Received Request

| Step | Description |
|------|-------------|
| `"{resource}" last "orders.v1.OrderService/CreateOrder" request json "clientOrderId" saved as "{{order_id}}"` | Save a field of the last request message a method received, as JSON (protojson, so field names are lowerCamelCase), e.g. an id the app generated and later refers to |
| `"{resource}" last "users.v1.UserService/GetUser" request metadata "x-request-id" saved as "{{request_id}}"` | Save a metadata value of the last call a method received |
| `"{resource}" last "users.v1.UserService/GetUser" request metadata "authorization" is "Bearer {{token}}"` | Assert a metadata value of the last call a method received |
| `"{resource}" last "users.v1.UserService/ListUsers" request json contains:` | Assert the last request message a method received contains these JSON fields (extra fields ignored, same matchers as the http resource's "response json contains") |


### Examples

**Assert the last request message a method received contains these JSON fields (extra fields ignored, same matchers as the http resource's "response json contains"):**
```gherkin
"{resource}" last "users.v1.UserService/ListUsers" request json contains:
  """
  {"pageSize": 50}
  """
```


## Server Info

| Step | Description |
|------|-------------|
| `"{resource}" address is stored in "USERS_ADDR"` | Stores the server's address (localhost:port) in a variable for use in other steps |


