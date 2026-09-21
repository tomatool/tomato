# gRPC

Steps for calling unary gRPC methods and validating responses

!!! tip "Multi-line Content"
    Steps ending with `:` accept multi-line content using Gherkin's docstring syntax (`"""`). See examples below each section.


## Request Setup

| Step | Description |
|------|-------------|
| `"grpc" metadata "authorization" is "Bearer token"` | Set a request metadata entry |
| `"grpc" metadata are:` | Set multiple metadata entries from table |
| `"grpc" request is:` | Set the request message as JSON (docstring) |



## Request Execution

| Step | Description |
|------|-------------|
| `"grpc" calls "helloworld.Greeter/SayHello"` | Call a unary method using the previously set request |
| `"grpc" calls "helloworld.Greeter/SayHello" with:` | Call a unary method with a JSON request (docstring) |



## Response Status

| Step | Description |
|------|-------------|
| `"grpc" response status is "OK"` | Assert the gRPC status code by name (OK, NOT_FOUND, INVALID_ARGUMENT, ...) |
| `"grpc" call succeeds` | Assert the call returned OK |
| `"grpc" call fails` | Assert the call returned any non-OK status |
| `"grpc" response error contains "not found"` | Assert the status message contains a substring |



## Response Body

| Step | Description |
|------|-------------|
| `"grpc" response contains "SERVING"` | Assert the JSON-rendered response contains a substring |
| `"grpc" response does not contain "error"` | Assert the JSON-rendered response does not contain a substring |



## Response JSON

| Step | Description |
|------|-------------|
| `"grpc" response json "status" is "SERVING"` | Assert a JSON path value on the response message |
| `"grpc" response json "employees[0].email" exists` | Assert a JSON path exists on the response message |
| `"grpc" response json "nextPageToken" does not exist` | Assert a JSON path is absent from the response message |
| `"grpc" response json matches:` | Assert the exact response structure, with the same matchers the HTTP handler supports |
| `"grpc" response json contains:` | Assert the response contains the given fields, ignoring extras |



## Service Discovery

| Step | Description |
|------|-------------|
| `"grpc" exposes service "grpc.health.v1.Health"` | Assert the server advertises a service via reflection |
| `"grpc" service "grpc.health.v1.Health" has method "Check"` | Assert a service exposes a method |



## Response Timing

| Step | Description |
|------|-------------|
| `"grpc" response time is less than "500ms"` | Assert the call completed within a time budget |


