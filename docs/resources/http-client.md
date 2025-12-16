# HTTP Client

Steps for making HTTP requests and validating responses


## Request Setup

| Step | Description |
|------|-------------|
| `"api" header "Content-Type" is "application/json"` | Set a header |
| `"api" headers are:` | Set multiple headers from table |
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
| `"api" response body does not contain "error"` | Assert body doesn't contain substring |
| `"api" response body is empty` | Assert empty body |


## Response JSON

| Step | Description |
|------|-------------|
| `"api" response json "data.id" is "123"` | Assert JSON path value |
| `"api" response json "data.id" exists` | Assert JSON path exists |
| `"api" response json "data.deleted" does not exist` | Assert JSON path doesn't exist |
| `"api" response json matches:` | Assert exact JSON structure with matchers: @string, @number, @boolean, @array, @object, @any, @null, @notnull, @empty, @notempty, @regex:pattern, @contains:text, @startswith:text, @endswith:text, @gt:n, @gte:n, @lt:n, @lte:n, @len:n |
| `"api" response json contains:` | Assert JSON contains specified fields (ignores extra fields). Supports same matchers as 'matches' |


## Response Timing

| Step | Description |
|------|-------------|
| `"api" response time is less than "500ms"` | Assert response time |

