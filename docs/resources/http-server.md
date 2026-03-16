# HTTP Server

Steps for stubbing HTTP services

!!! tip "Multi-line Content"
    Steps ending with `:` accept multi-line content using Gherkin's docstring syntax (`"""`). See examples below each section.


## Stub Setup

| Step | Description |
|------|-------------|
| `"{resource}" stub "GET" "/users" returns "200"` | Creates a stub that returns a status code |
| `"{resource}" stub "GET" "/users" returns "200" with body:` | Creates a stub that returns a status code and body |
| `"{resource}" stub "GET" "/users" returns "200" with json:` | Creates a stub that returns JSON (auto sets Content-Type) |
| `"{resource}" stub "GET" "/users" returns "200" with headers:` | Creates a stub that returns with custom headers |


### Examples

**Creates a stub that returns a status code and body:**
```gherkin
"{resource}" stub "GET" "/users" returns "200" with body:
  """
  [{"id": 1}]
  """
```

**Creates a stub that returns JSON (auto sets Content-Type):**
```gherkin
"{resource}" stub "GET" "/users" returns "200" with json:
  """
  [{"id": 1}]
  """
```

**Creates a stub that returns with custom headers:**
```gherkin
"{resource}" stub "GET" "/users" returns "200" with headers:
  | header       | value            |
  | X-Custom     | value            |
```


## Verification

| Step | Description |
|------|-------------|
| `"{resource}" received "GET" "/users"` | Asserts a request was received |
| `"{resource}" received "GET" "/users" "2" times` | Asserts a request was received N times |
| `"{resource}" did not receive "DELETE" "/users"` | Asserts a request was not received |
| `"{resource}" received request with header "Authorization" containing "Bearer"` | Asserts any request was received with header containing value |
| `"{resource}" received request with body containing "name"` | Asserts any request was received with body containing value |
| `"{resource}" received "5" requests` | Asserts total number of requests received |



## Server Info

| Step | Description |
|------|-------------|
| `"{resource}" url is stored in "SERVER_URL"` | Stores the server URL in a variable for use in other steps |



## Fixture Management

| Step | Description |
|------|-------------|
| `"{resource}" loads fixtures from "fixtures/github-api"` | Loads fixture stubs from the specified directory path |


