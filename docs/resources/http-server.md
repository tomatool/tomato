# HTTP Server

Steps for stubbing HTTP services


## Stub Setup

| Step | Description |
|------|-------------|
| `"{resource}" stub "GET" "/users" returns "200"` | Creates a stub that returns a status code |
| `"{resource}" stub "GET" "/users" returns "200" with body:
  """
  [{"id": 1}]
  """` | Creates a stub that returns a status code and body |
| `"{resource}" stub "GET" "/users" returns "200" with json:
  """
  [{"id": 1}]
  """` | Creates a stub that returns JSON (auto sets Content-Type) |
| `"{resource}" stub "GET" "/users" returns "200" with headers:
  | header       | value            |
  | X-Custom     | value            |` | Creates a stub that returns with custom headers |


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

