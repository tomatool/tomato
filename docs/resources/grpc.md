# gRPC

Steps for calling unary gRPC methods and validating responses

!!! tip "Multi-line Content"
    Steps ending with `:` accept multi-line content using Gherkin's docstring syntax (`"""`). See examples below each section.


## Request Setup

| Step | Description |
|------|-------------|
| `"app-grpc-client" metadata "authorization" is "Bearer token"` | Set a request metadata entry |
| `"app-grpc-client" metadata are:` | Set multiple metadata entries from table |
| `"app-grpc-client" request is:` | Set the request message as JSON (docstring) |



## Request Execution

| Step | Description |
|------|-------------|
| `"app-grpc-client" calls "helloworld.Greeter/SayHello"` | Call a unary method using the previously set request |
| `"app-grpc-client" calls "helloworld.Greeter/SayHello" with:` | Call a unary method with a JSON request (docstring) |



## Response Status

| Step | Description |
|------|-------------|
| `"app-grpc-client" response status is "OK"` | Assert the gRPC status code by name (OK, NOT_FOUND, INVALID_ARGUMENT, ...; NotFound and the number work too) |
| `"app-grpc-client" call succeeds` | Assert the call returned OK |
| `"app-grpc-client" call fails` | Assert the call returned any non-OK status |
| `"app-grpc-client" response error contains "not found"` | Assert the status message contains a substring |



## Response Body

| Step | Description |
|------|-------------|
| `"app-grpc-client" response contains "SERVING"` | Assert the JSON-rendered response contains a substring |
| `"app-grpc-client" response does not contain "error"` | Assert the JSON-rendered response does not contain a substring |



## Response JSON

| Step | Description |
|------|-------------|
| `"app-grpc-client" response json "status" is "SERVING"` | Assert a JSON path value on the response message |
| `"app-grpc-client" response json "employees[0].email" exists` | Assert a JSON path exists on the response message |
| `"app-grpc-client" response json "nextPageToken" does not exist` | Assert a JSON path is absent from the response message |
| `"app-grpc-client" response json matches:` | Assert the exact response structure, with the same matchers the HTTP handler supports |
| `"app-grpc-client" response json contains:` | Assert the response contains the given fields, ignoring extras |



## Service Discovery

| Step | Description |
|------|-------------|
| `"app-grpc-client" exposes service "grpc.health.v1.Health"` | Assert the server advertises a service via reflection |
| `"app-grpc-client" service "grpc.health.v1.Health" has method "Check"` | Assert a service exposes a method |



## Response Timing

| Step | Description |
|------|-------------|
| `"app-grpc-client" response time is less than "500ms"` | Assert the call completed within a time budget |


