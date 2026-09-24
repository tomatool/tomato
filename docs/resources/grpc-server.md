# gRPC Server

Steps for stubbing gRPC services the app depends on

!!! tip "Multi-line Content"
    Steps ending with `:` accept multi-line content using Gherkin's docstring syntax (`"""`). See examples below each section.


## Stub Setup

| Step | Description |
|------|-------------|
| `"{resource}" stub "payments.v1.Payments/Authorize" returns:` | Stubs a unary method to return a JSON-encoded response |
| `"{resource}" stub "payments.v1.Payments/Authorize" returns status "UNAVAILABLE"` | Stubs a method to fail with a gRPC status code |
| `"{resource}" stub "payments.v1.Payments/Authorize" returns status "FAILED_PRECONDITION" with message "card expired"` | Stubs a method to fail with a status code and message |


### Examples

**Stubs a unary method to return a JSON-encoded response:**
```gherkin
"{resource}" stub "payments.v1.Payments/Authorize" returns:
  """
  {"approved": true, "authorizationId": "auth-1"}
  """
```


## Request Verification

| Step | Description |
|------|-------------|
| `"{resource}" received "payments.v1.Payments/Authorize"` | Asserts the method was called at least once |
| `"{resource}" received "payments.v1.Payments/Authorize" "2" times` | Asserts the method was called exactly N times |
| `"{resource}" did not receive "payments.v1.Payments/Refund"` | Asserts the method was never called |
| `"{resource}" received "payments.v1.Payments/Authorize" with json:` | Asserts the last call to the method contains the JSON fields |
| `"{resource}" received metadata "authorization" containing "Bearer"` | Asserts a call carried metadata (a header) containing the value |
| `"{resource}" received "3" requests` | Asserts the total number of calls received |


### Examples

**Asserts the last call to the method contains the JSON fields:**
```gherkin
"{resource}" received "payments.v1.Payments/Authorize" with json:
  """
  {"orderId": "order-1", "amount": "4200"}
  """
```


## Utilities

| Step | Description |
|------|-------------|
| `"{resource}" address is stored in "PAYMENTS_ADDR"` | Stores the server's host:port in a variable |


