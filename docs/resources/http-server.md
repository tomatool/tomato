# HTTP Server

Steps for stubbing HTTP services

!!! tip "Multi-line Content"
    Steps ending with `:` accept multi-line content using Gherkin's docstring syntax (`"""`). See examples below each section.


## Stub Setup

| Step | Description |
|------|-------------|
| `"{resource}" stub "GET" "/users" returns "200"` | Creates a stub that returns a status code. Between the path and "returns", `with header "k" "v"`, `with query "k" "v"` and `with body containing "x"` narrow which requests it answers; the most specific stub wins, then the most recent |
| `"{resource}" stub "GET" "/users" returns "200" with body:` | Creates a stub that returns a status code and body |
| `"{resource}" stub "POST" "/" with header "X-Amz-Target" "TrentService.Encrypt" returns "200" with json:` | Creates a stub that returns JSON (auto sets Content-Type) |
| `"{resource}" stub "GET" "/users" returns "200" with headers:` | Creates a stub that returns with custom headers |
| `"{resource}" stub "POST" "/token" returns in sequence:` | Creates a stub that answers each request with the next row, and repeats the last row once the rows are used up. Columns: status (required), json or body, and delay (e.g. 3s) to answer late, as a slow or timing-out dependency does |


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
"{resource}" stub "POST" "/" with header "X-Amz-Target" "TrentService.Encrypt" returns "200" with json:
  """
  {"KeyId": "test"}
  """
```

**Creates a stub that returns with custom headers:**
```gherkin
"{resource}" stub "GET" "/users" returns "200" with headers:
  | header       | value            |
  | X-Custom     | value            |
```

**Creates a stub that answers each request with the next row, and repeats the last row once the rows are used up. Columns: status (required), json or body, and delay (e.g. 3s) to answer late, as a slow or timing-out dependency does:**
```gherkin
"{resource}" stub "POST" "/token" returns in sequence:
  | status | json                         | delay |
  | 200    | {"pending": true}            | 5s    |
  | 200    | {"token": "t"}               |       |
```


## Availability

| Step | Description |
|------|-------------|
| `"{resource}" refuses connections` | Close the server's port, so the app's connections are refused as when the service is down. Requests already in flight finish |
| `"{resource}" accepts connections` | Open the port again on the same address, with the stubs kept. Every scenario starts accepting |



## Verification

| Step | Description |
|------|-------------|
| `"{resource}" received "GET" "/users"` | Asserts a request was received |
| `"{resource}" received "GET" "/users" "2" times` | Asserts a request was received N times |
| `"{resource}" received "POST" "/" with header "X-Amz-Target" "TrentService.Encrypt"` | Asserts a request was received with this exact header value, e.g. which operation of a one-path API was called |
| `"{resource}" did not receive "DELETE" "/users"` | Asserts a request was not received |
| `"{resource}" received request with header "Authorization" containing "Bearer"` | Asserts any request was received with header containing value |
| `"{resource}" received request with body containing "name"` | Asserts any request was received with body containing value |
| `"{resource}" received "5" requests` | Asserts total number of requests received |



## Received Request

| Step | Description |
|------|-------------|
| `"{resource}" last "POST" "/orders" request json "clientProcessId" saved as "{{process_id}}"` | Save a field of the body of the last matching request the stub received, e.g. an id the app generated and later refers to |
| `"{resource}" last "POST" "/orders" request header "Idempotency-Key" saved as "{{key}}"` | Save a header of the last matching request the stub received |
| `"{resource}" last "POST" "/orders" request json contains:` | Assert the body of the last matching request contains these JSON fields (extra fields ignored, same matchers as the http resource's "response json contains") |


### Examples

**Assert the body of the last matching request contains these JSON fields (extra fields ignored, same matchers as the http resource's "response json contains"):**
```gherkin
"{resource}" last "POST" "/orders" request json contains:
  """
  {"orderSize": "10"}
  """
```


## Server Info

| Step | Description |
|------|-------------|
| `"{resource}" url is stored in "SERVER_URL"` | Stores the server URL in a variable for use in other steps |


