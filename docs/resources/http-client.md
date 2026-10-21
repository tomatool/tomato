# HTTP Client

Steps for making HTTP requests, validating responses, reading event streams and sending concurrent requests

!!! tip "Multi-line Content"
    Steps ending with `:` accept multi-line content using Gherkin's docstring syntax (`"""`). See examples below each section.


## Request Setup

| Step | Description |
|------|-------------|
| `"app-client" header "Content-Type" is "application/json"` | Set a header (a "Host" header sets the request host) |
| `"app-client" headers are:` | Set multiple headers from table |
| `"app-client" cookie "session" is "{{session}}"` | Set a request cookie (kept for the rest of the scenario, like headers) |
| `"app-client" query param "page" is "1"` | Set a query parameter |
| `"app-client" body is:` | Set raw request body (docstring) |
| `"app-client" json body is:` | Set JSON body + Content-Type header |
| `"app-client" form body is:` | Set form-encoded body from table |



## Request Execution

| Step | Description |
|------|-------------|
| `"app-client" sends "GET" to "/users"` | Send HTTP request |
| `"app-client" sends "POST" to "/users" with body:` | Send with raw body |
| `"app-client" sends "POST" to "/users" with json:` | Send with JSON body |



## Response Status

| Step | Description |
|------|-------------|
| `"app-client" response status is "200"` | Assert exact status code |
| `"app-client" response status is success` | Assert status class (2xx, 3xx, 4xx, 5xx) |



## Response Headers

| Step | Description |
|------|-------------|
| `"app-client" response header "Content-Type" is "application/json"` | Assert exact header value |
| `"app-client" response header "Content-Type" contains "json"` | Assert header contains substring |
| `"app-client" response header "X-Request-Id" exists` | Assert header exists |



## Response Body

| Step | Description |
|------|-------------|
| `"app-client" response body is:` | Assert exact body match |
| `"app-client" response body contains "success"` | Assert body contains substring |
| `"app-client" response body contains:` | Assert body contains the docstring text (use for text with quotes) |
| `"app-client" response body does not contain "error"` | Assert body doesn't contain substring |
| `"app-client" response body does not contain:` | Assert body doesn't contain the docstring text |
| `"app-client" response body is empty` | Assert empty body |



## Response Cookies

| Step | Description |
|------|-------------|
| `"app-client" response cookie "session" exists` | Assert the response sets a cookie |
| `"app-client" response cookie "session" is ""` | Assert a cookie's value (an empty value means the cookie is cleared) |
| `"app-client" response cookie "session" saved as "{{session}}"` | Save a cookie's value to a variable |



## Response JSON

| Step | Description |
|------|-------------|
| `"app-client" response json "data.id" is "123"` | Assert JSON path value |
| `"app-client" response json "data.id" exists` | Assert JSON path exists |
| `"app-client" response json "data.deleted" does not exist` | Assert JSON path doesn't exist |
| `"app-client" response json matches:` | Assert exact JSON structure with matchers: @string, @number, @boolean, @array, @object, @any, @null, @notnull, @empty, @notempty, @regex:pattern, @contains:text, @startswith:text, @endswith:text, @gt:n, @gte:n, @lt:n, @lte:n, @len:n |
| `"app-client" response json contains:` | Assert JSON contains specified fields (ignores extra fields). Supports same matchers as 'matches' |
| `"app-client" response json "id" matches pattern "^[0-9a-f-]{36}$"` | Assert JSON path value matches regex pattern |
| `"app-client" response json "id" is uuid` | Assert JSON path value is a valid UUID |
| `"app-client" response json "email" is email` | Assert JSON path value is a valid email format |
| `"app-client" response json "created_at" is iso-timestamp` | Assert JSON path value is an ISO 8601 timestamp |



## Response Timing

| Step | Description |
|------|-------------|
| `"app-client" response time is less than "500ms"` | Assert response time |



## Variable Capture

| Step | Description |
|------|-------------|
| `"app-client" response json "id" saved as "{{user_id}}"` | Save JSON path value to variable for use in subsequent requests |
| `"app-client" response header "Location" saved as "{{location}}"` | Save response header value to variable |



## Event Streams

| Step | Description |
|------|-------------|
| `"app-client" opens stream "/events"` | Open a Server-Sent Events stream with the scenario's headers, cookies and query params, and fail unless it answers 200 with text/event-stream |
| `"app-client" opens stream "/events" after event id "{{last_id}}"` | Resume a stream: open it with a Last-Event-ID header, as a reconnecting client does |
| `"app-client" closes stream` | Close the stream from the client side |
| `"app-client" stream skips events "heartbeat"` | Leave events of these comma-separated types out of every receive and assertion for the rest of the scenario, e.g. heartbeats sent as events |
| `"app-client" receives event "order.created" within "5s"` | Assert the next event has this type, waiting for it up to the duration |
| `"app-client" receives event "order.created" within "5s" with json containing:` | Assert the next event has this type and its data contains the JSON fields given (extra fields ignored, same matchers as "response json contains") |
| `"app-client" receives events within "5s":` | Assert the next events, in order, from a table with an event column and optional id and data columns |
| `"app-client" does not receive an event within "2s"` | Assert no further event arrives for the duration (comments don't count) |
| `"app-client" receives a comment within "2s"` | Assert a comment line (": ..."), the usual keep-alive, arrives within the duration |
| `"app-client" stream event ids are consecutive from "1"` | Assert the ids of every event received on this stream so far are integers counting up by one from the number given, with no gap or repeat. Events without an id are left out |
| `"app-client" stops reading the stream` | Stop reading the stream without closing it, as a stalled client does: what the server sends piles up in the connection's buffers until the server blocks, and a server that protects itself disconnects the slow consumer |
| `"app-client" resumes reading the stream` | Read the stream again: what arrived while paused is delivered in order, then whatever the server did meanwhile (sent more, or closed the stream) |
| `"app-client" stream is closed by the server within "5s"` | Assert the server ends the stream within the duration |
| `"app-client" stream is disconnected by the server within "5s"` | Assert the server ends the stream within the duration, cleanly or by dropping the connection, as a server that disconnects a slow consumer does |
| `"app-client" last stream event id saved as "{{last_id}}"` | Save the id of the last event a receive step matched, the position a client would resume from |



## Concurrent Requests

| Step | Description |
|------|-------------|
| `"app-client" sends "2" concurrent "POST" to "/orders"` | Send the same request N times at the same moment, with the scenario's headers, cookies and query params |
| `"app-client" sends "2" concurrent "POST" to "/orders" with json:` | Send the same JSON request N times at the same moment |
| `"app-client" "2" of the concurrent responses have status "202"` | Assert how many of the concurrent responses have this status |
| `"app-client" "1" of the concurrent responses have json "duplicate" is "true"` | Assert how many of the concurrent responses have this value at a JSON path (a missing path counts as not having it) |
| `"app-client" "1" of the concurrent responses contain json:` | Assert how many of the concurrent responses contain these JSON fields (same matchers as "response json contains") |


### Examples

**Send the same JSON request N times at the same moment:**
```gherkin
"app-client" sends "2" concurrent "POST" to "/orders" with json:
  """
  {"clientOrderId": "c-60"}
  """
```

**Assert how many of the concurrent responses contain these JSON fields (same matchers as "response json contains"):**
```gherkin
"app-client" "1" of the concurrent responses contain json:
  """
  {"duplicate": true}
  """
```

