# HTTP Server Configuration

This guide covers how to configure the HTTP Server handler for mocking external services in integration tests.

## Overview

The HTTP Server handler creates a local mock server that can stub HTTP endpoints. This is useful for testing how your application interacts with external APIs without actually calling them.

## Use Cases

- Mock third-party APIs (payment gateways, notification services, etc.)
- Simulate various response scenarios (success, errors, timeouts)
- Verify your application sends correct requests
- Test retry logic and error handling

## Resource Configuration

The HTTP Server is a standalone handler that doesn't require a container:

```yaml
resources:
  mock:
    type: http-server
    options:
      port: 9999
```

### Resource Options

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `port` | int | `0` (random) | Port to listen on. Use `0` for system-assigned port, or specify a fixed port |

## Configuring Your App to Use Mock Servers

Since your application needs to call the mock server instead of the real external API, you must configure the mock server URLs via environment variables.

Use the `{{.resource_name.url}}` template syntax to reference mock server URLs:

```yaml
resources:
  payment-api:
    type: http-server
    options:
      port: 9001

  notification-api:
    type: http-server
    options:
      port: 9002

app:
  command: go run ./cmd/server
  env:
    # Use template syntax to reference mock server URLs
    PAYMENT_API_URL: "{{.payment-api.url}}"
    NOTIFICATION_API_URL: "{{.notification-api.url}}"
```

The templates resolve to:
- **Command mode**: `http://localhost:{port}` (e.g., `http://localhost:9001`)
- **Container mode**: `http://host.docker.internal:{port}` (accessible from Docker)

Your application code should read these URLs from environment variables:

```go
paymentAPIURL := os.Getenv("PAYMENT_API_URL")
// Use paymentAPIURL when making HTTP calls to payment service
```

!!! note "Fixed Ports Required"
    HTTP server resources must have a fixed `port` configured in options for the URL template to work. Without a port, the template cannot be resolved.

## Complete Example

Here's a complete `tomato.yml` with multiple mock servers:

```yaml
version: 2

settings:
  timeout: 5m
  fail_fast: false
  output: pretty
  reset:
    level: scenario

resources:
  # Mock external payment gateway
  payment-api:
    type: http-server
    options:
      port: 9001

  # Mock notification service
  notification-api:
    type: http-server
    options:
      port: 9002

  # HTTP client to test your app
  api:
    type: http-client
    base_url: "http://localhost:8080"

app:
  command: go run ./cmd/server
  port: 8080
  env:
    # Templates resolve to http://localhost:9001, http://localhost:9002
    PAYMENT_API_URL: "{{.payment-api.url}}"
    NOTIFICATION_API_URL: "{{.notification-api.url}}"
  ready:
    type: http
    path: /health
    timeout: 30s

features:
  paths:
    - ./features
```

## Writing HTTP Server Tests

### Basic Stubbing

```gherkin
Feature: Payment Processing

  Scenario: Process successful payment
    # Stub the payment gateway
    Given "payment-api" stub "POST" "/charge" returns "200" with json:
      """
      {"transaction_id": "txn_123", "status": "success"}
      """
    # Your app calls the payment gateway
    When "api" sends "POST" to "/process-payment" with json:
      """
      {"amount": 100, "currency": "USD"}
      """
    Then "api" response status is "200"
    And "api" response json "status" is "completed"
```

### Stubbing Different Responses

```gherkin
Scenario: Handle payment gateway error
  Given "payment-api" stub "POST" "/charge" returns "500" with json:
    """
    {"error": "Gateway unavailable"}
    """
  When "api" sends "POST" to "/process-payment" with json:
    """
    {"amount": 100}
    """
  Then "api" response status is "503"
```

### Custom Response Headers

```gherkin
Scenario: API returns rate limit headers
  Given "payment-api" stub "GET" "/balance" returns "200" with headers:
    | header           | value |
    | X-RateLimit-Remaining | 99    |
    | X-RateLimit-Reset     | 3600  |
  When "api" sends "GET" to "/check-balance"
  Then "api" response status is "200"
```

### Verifying Requests

```gherkin
Scenario: Verify authorization header is sent
  Given "payment-api" stub "POST" "/charge" returns "200"
  When "api" sends "POST" to "/process-payment" with json:
    """
    {"amount": 50}
    """
  Then "payment-api" received "POST" "/charge"
  And "payment-api" received request with header "Authorization" containing "Bearer"

Scenario: Verify request body
  Given "payment-api" stub "POST" "/charge" returns "200"
  When "api" sends "POST" to "/process-payment" with json:
    """
    {"amount": 75, "currency": "EUR"}
    """
  Then "payment-api" received request with body containing "EUR"
```

### Request Counting

```gherkin
Scenario: Verify retry behavior
  # Stub fails first two times, succeeds third
  Given "payment-api" stub "POST" "/charge" returns "503"
  When "api" sends "POST" to "/process-payment-with-retry"
  # App should have retried 3 times
  Then "payment-api" received "POST" "/charge" "3" times
```

### Negative Assertions

```gherkin
Scenario: Verify no unauthorized calls
  Given "payment-api" stub "GET" "/balance" returns "200"
  When "api" sends "GET" to "/public-info"
  Then "payment-api" did not receive "GET" "/balance"
```

See [HTTP Server Steps](../resources/http-server.md) for the complete list of available steps.

## Multiple Mock Servers

You can configure multiple mock servers for different external services:

```yaml
resources:
  payment-api:
    type: http-server
    options:
      port: 9001

  notification-api:
    type: http-server
    options:
      port: 9002

  analytics-api:
    type: http-server
    options:
      port: 9003
```

## Reset Behavior

Between each scenario:
- All stubs are cleared
- All recorded calls are cleared

This ensures each scenario starts with a clean slate.

## Troubleshooting

### Port Already in Use

If you get "address already in use" error:

1. Use `port: 0` to let the system assign a free port
2. Or kill processes using the specified port: `lsof -ti:9999 | xargs kill -9`

### Requests Not Being Recorded

1. Verify the URL in your test matches the stub path exactly
2. Check that the HTTP method matches (GET vs POST)
3. Ensure your application is pointing to the mock server's port

### Stub Not Matching

Stubs match in order they were defined. If a request doesn't match any stub, the server returns 404 with a message indicating no stub was found.
