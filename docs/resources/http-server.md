# HTTP Server

Steps for stubbing HTTP services

!!! tip "Multi-line Content"
    Steps ending with `:` accept multi-line content using Gherkin's docstring syntax (`"""`). See examples below each section.


## Stub Setup

| Step | Description |
|------|-------------|
| `"users-stub" stub "GET" "/users" returns "200"` | Creates a stub that returns a status code. Between the path and "returns", `with header "k" "v"`, `with query "k" "v"` and `with body containing "x"` narrow which requests it answers; the most specific stub wins, then the most recent |
| `"users-stub" stub "GET" "/users" returns "200" with body:` | Creates a stub that returns a status code and body |
| `"users-stub" stub "POST" "/" with header "X-Amz-Target" "TrentService.Encrypt" returns "200" with json:` | Creates a stub that returns JSON (auto sets Content-Type) |
| `"users-stub" stub "GET" "/users" returns "200" with headers:` | Creates a stub that returns with custom headers |
| `"users-stub" stub "POST" "/token" returns in sequence:` | Creates a stub that answers each request with the next row, and repeats the last row once the rows are used up. Columns: status (required), json or body, and delay (e.g. 3s) to answer late, as a slow or timing-out dependency does |


### Examples

**Creates a stub that returns a status code and body:**
```gherkin
"users-stub" stub "GET" "/users" returns "200" with body:
  """
  [{"id": 1}]
  """
```

**Creates a stub that returns JSON (auto sets Content-Type):**
```gherkin
"users-stub" stub "POST" "/" with header "X-Amz-Target" "TrentService.Encrypt" returns "200" with json:
  """
  {"KeyId": "test"}
  """
```

**Creates a stub that returns with custom headers:**
```gherkin
"users-stub" stub "GET" "/users" returns "200" with headers:
  | header       | value            |
  | X-Custom     | value            |
```

**Creates a stub that answers each request with the next row, and repeats the last row once the rows are used up. Columns: status (required), json or body, and delay (e.g. 3s) to answer late, as a slow or timing-out dependency does:**
```gherkin
"users-stub" stub "POST" "/token" returns in sequence:
  | status | json                         | delay |
  | 200    | {"pending": true}            | 5s    |
  | 200    | {"token": "t"}               |       |
```


## Availability

| Step | Description |
|------|-------------|
| `"users-stub" refuses connections` | Close the server's port, so the app's connections are refused as when the service is down. Requests already in flight finish |
| `"users-stub" accepts connections` | Open the port again on the same address, with the stubs kept. Every scenario starts accepting |



## Verification

| Step | Description |
|------|-------------|
| `"users-stub" received "GET" "/users"` | Asserts a request was received |
| `"users-stub" received "GET" "/users" "2" times` | Asserts a request was received N times |
| `"users-stub" received "POST" "/" with header "X-Amz-Target" "TrentService.Encrypt"` | Asserts a request was received with this exact header value, e.g. which operation of a one-path API was called |
| `"users-stub" did not receive "DELETE" "/users"` | Asserts a request was not received |
| `"users-stub" received request with header "Authorization" containing "Bearer"` | Asserts any request was received with header containing value |
| `"users-stub" received request with body containing "name"` | Asserts any request was received with body containing value |
| `"users-stub" received "5" requests` | Asserts total number of requests received |



## Received Request

| Step | Description |
|------|-------------|
| `"users-stub" last "POST" "/orders" request json "clientProcessId" saved as "{{process_id}}"` | Save a field of the body of the last matching request the stub received, e.g. an id the app generated and later refers to |
| `"users-stub" last "POST" "/orders" request header "Idempotency-Key" saved as "{{key}}"` | Save a header of the last matching request the stub received |
| `"users-stub" last "POST" "/orders" request json contains:` | Assert the body of the last matching request contains these JSON fields (extra fields ignored, same matchers as the http resource's "response json contains") |


### Examples

**Assert the body of the last matching request contains these JSON fields (extra fields ignored, same matchers as the http resource's "response json contains"):**
```gherkin
"users-stub" last "POST" "/orders" request json contains:
  """
  {"orderSize": "10"}
  """
```


## Server Info

| Step | Description |
|------|-------------|
| `"users-stub" url is stored in "SERVER_URL"` | Stores the server URL in a variable for use in other steps |


