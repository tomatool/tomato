# HTTP Client

Steps for making HTTP requests, validating responses, reading event streams and sending concurrent requests

!!! tip "Multi-line Content"
    Steps ending with `:` accept multi-line content using Gherkin's docstring syntax (`"""`). See examples below each section.


## Request Setup

| Step | Description |
|------|-------------|
| `"api" header "Content-Type" is "application/json"` | Set a header (a "Host" header sets the request host) |
| `"api" headers are:` | Set multiple headers from table |
| `"api" cookie "session" is "{{session}}"` | Set a request cookie (kept for the rest of the scenario, like headers) |
| `"api" query param "page" is "1"` | Set a query parameter |
| `"api" body is:` | Set raw request body (docstring) |
| `"api" json body is:` | Set JSON body + Content-Type header |
| `"api" form body is:` | Set form-encoded body from table |



## Request Execution

| Step | Description |
|------|-------------|
| `"api" sends "GET" to "/users"` | Send HTTP request |
| `"api" sends "POST" to "/users" with body:` | Send with raw body |
| `"api" sends "POST" to "/users" with json:` | Send with JSON body |



## Response Status

| Step | Description |
|------|-------------|
| `"api" response status is "200"` | Assert exact status code |
| `"api" response status is success` | Assert status class (2xx, 3xx, 4xx, 5xx) |



## Response Headers

| Step | Description |
|------|-------------|
| `"api" response header "Content-Type" is "application/json"` | Assert exact header value |
| `"api" response header "Content-Type" contains "json"` | Assert header contains substring |
| `"api" response header "X-Request-Id" exists` | Assert header exists |



## Response Body

| Step | Description |
|------|-------------|
| `"api" response body is:` | Assert exact body match |
| `"api" response body contains "success"` | Assert body contains substring |
| `"api" response body contains:` | Assert body contains the docstring text (use for text with quotes) |
| `"api" response body does not contain "error"` | Assert body doesn't contain substring |
| `"api" response body does not contain:` | Assert body doesn't contain the docstring text |
| `"api" response body is empty` | Assert empty body |



## Response Cookies

| Step | Description |
|------|-------------|
| `"api" response cookie "session" exists` | Assert the response sets a cookie |
| `"api" response cookie "session" is ""` | Assert a cookie's value (an empty value means the cookie is cleared) |
| `"api" response cookie "session" saved as "{{session}}"` | Save a cookie's value to a variable |



## Response JSON

| Step | Description |
|------|-------------|
| `"api" response json "data.id" is "123"` | Assert JSON path value |
| `"api" response json "data.id" exists` | Assert JSON path exists |
| `"api" response json "data.deleted" does not exist` | Assert JSON path doesn't exist |
| `"api" response json matches:` | Assert exact JSON structure with matchers: @string, @number, @boolean, @array, @object, @any, @null, @notnull, @empty, @notempty, @regex:pattern, @contains:text, @startswith:text, @endswith:text, @gt:n, @gte:n, @lt:n, @lte:n, @len:n |
| `"api" response json contains:` | Assert JSON contains specified fields (ignores extra fields). Supports same matchers as 'matches' |
| `"api" response json "id" matches pattern "^[0-9a-f-]{36}$"` | Assert JSON path value matches regex pattern |
| `"api" response json "id" is uuid` | Assert JSON path value is a valid UUID |
| `"api" response json "email" is email` | Assert JSON path value is a valid email format |
| `"api" response json "created_at" is iso-timestamp` | Assert JSON path value is an ISO 8601 timestamp |



## Response Timing

| Step | Description |
|------|-------------|
| `"api" response time is less than "500ms"` | Assert response time |



## Variable Capture

| Step | Description |
|------|-------------|
| `"api" response json "id" saved as "{{user_id}}"` | Save JSON path value to variable for use in subsequent requests |
| `"api" response header "Location" saved as "{{location}}"` | Save response header value to variable |



## Event Streams

| Step | Description |
|------|-------------|
| `"api" opens stream "/events"` | Open a Server-Sent Events stream with the scenario's headers, cookies and query params, and fail unless it answers 200 with text/event-stream |
| `"api" opens stream "/events" after event id "{{last_id}}"` | Resume a stream: open it with a Last-Event-ID header, as a reconnecting client does |
| `"api" closes stream` | Close the stream from the client side |
| `"api" stream skips events "heartbeat"` | Leave events of these comma-separated types out of every receive and assertion for the rest of the scenario, e.g. heartbeats sent as events |
| `"api" receives event "order.created" within "5s"` | Assert the next event has this type, waiting for it up to the duration |
| `"api" receives event "order.created" within "5s" with json containing:` | Assert the next event has this type and its data contains the JSON fields given (extra fields ignored, same matchers as "response json contains") |
| `"api" receives events within "5s":` | Assert the next events, in order, from a table with an event column and optional id and data columns |
| `"api" does not receive an event within "2s"` | Assert no further event arrives for the duration (comments don't count) |
| `"api" receives a comment within "2s"` | Assert a comment line (": ..."), the usual keep-alive, arrives within the duration |
| `"api" stream event ids are consecutive from "1"` | Assert the ids of every event received on this stream so far are integers counting up by one from the number given, with no gap or repeat. Events without an id are left out |
| `"api" stops reading the stream` | Stop reading the stream without closing it, as a stalled client does: what the server sends piles up in the connection's buffers until the server blocks, and a server that protects itself disconnects the slow consumer |
| `"api" resumes reading the stream` | Read the stream again: what arrived while paused is delivered in order, then whatever the server did meanwhile (sent more, or closed the stream) |
| `"api" stream is closed by the server within "5s"` | Assert the server ends the stream within the duration |
| `"api" stream is disconnected by the server within "5s"` | Assert the server ends the stream within the duration, cleanly or by dropping the connection, as a server that disconnects a slow consumer does |
| `"api" last stream event id saved as "{{last_id}}"` | Save the id of the last event a receive step matched, the position a client would resume from |



## Concurrent Requests

| Step | Description |
|------|-------------|
| `"api" sends "2" concurrent "POST" to "/orders"` | Send the same request N times at the same moment, with the scenario's headers, cookies and query params |
| `"api" sends "2" concurrent "POST" to "/orders" with json:` | Send the same JSON request N times at the same moment |
| `"api" "2" of the concurrent responses have status "202"` | Assert how many of the concurrent responses have this status |
| `"api" "1" of the concurrent responses have json "duplicate" is "true"` | Assert how many of the concurrent responses have this value at a JSON path (a missing path counts as not having it) |
| `"api" "1" of the concurrent responses contain json:` | Assert how many of the concurrent responses contain these JSON fields (same matchers as "response json contains") |


### Examples

**Send the same JSON request N times at the same moment:**
```gherkin
"api" sends "2" concurrent "POST" to "/orders" with json:
  """
  {"clientOrderId": "c-60"}
  """
```

**Assert how many of the concurrent responses contain these JSON fields (same matchers as "response json contains"):**
```gherkin
"api" "1" of the concurrent responses contain json:
  """
  {"duplicate": true}
  """
```

