# HTTP Server

Steps for stubbing HTTP services

!!! tip "Multi-line Content"
    Steps ending with `:` accept multi-line content using Gherkin's docstring syntax (`"""`). See examples below each section.


## Stub Setup

| Step | Description |
|------|-------------|
| `"{resource}" stub "{method}" "{path}" returns "{status}"` | Creates a stub that returns a status code |
| `"{resource}" stub "{method}" "{path}" returns "{status}" with body:` | Creates a stub with a response body (see example below) |
| `"{resource}" stub "{method}" "{path}" returns "{status}" with json:` | Creates a stub with JSON response (auto sets Content-Type) |
| `"{resource}" stub "{method}" "{path}" returns "{status}" with headers:` | Creates a stub with custom response headers |

### Examples

**Stub with body:**
```gherkin
Given "mock" stub "GET" "/users" returns "200" with body:
  """
  [{"id": 1, "name": "Alice"}]
  """
```

**Stub with JSON (auto sets Content-Type: application/json):**
```gherkin
Given "mock" stub "POST" "/users" returns "201" with json:
  """
  {"id": 1, "created": true}
  """
```

**Stub with custom headers:**
```gherkin
Given "mock" stub "GET" "/users" returns "200" with headers:
  | header       | value            |
  | X-Custom     | custom-value     |
  | X-Request-Id | req-123          |
```


## Verification

| Step | Description |
|------|-------------|
| `"{resource}" received "{method}" "{path}"` | Asserts a request was received |
| `"{resource}" received "{method}" "{path}" "{n}" times` | Asserts a request was received N times |
| `"{resource}" did not receive "{method}" "{path}"` | Asserts a request was not received |
| `"{resource}" received request with header "{name}" containing "{value}"` | Asserts any request had header containing value |
| `"{resource}" received request with body containing "{value}"` | Asserts any request had body containing value |
| `"{resource}" received "{n}" requests` | Asserts total number of requests received |


## Server Info

| Step | Description |
|------|-------------|
| `"{resource}" url is stored in "{variable}"` | Stores the server URL in a variable for use in other steps |
