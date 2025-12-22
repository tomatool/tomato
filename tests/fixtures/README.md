# HTTP Server Fixtures

This directory contains fixture-based API mocks for HTTP server testing.

## Overview

Fixtures provide a way to define reusable HTTP stub responses that can be automatically loaded into HTTP server resources. This is particularly useful when mocking external APIs (like GitHub, Slack, etc.) that have many endpoints.

## Benefits

- **Reusability**: Define common API mocks once, use across all test scenarios
- **Maintainability**: Centralized fixture management in version control
- **Flexibility**: Combine with dynamic stubs for scenario-specific overrides
- **Realistic Mocking**: Support complex conditions (headers, query params, request body)
- **Organization**: Keep test data separate from test logic

## Directory Structure

```
fixtures/
  {service-name}/           # e.g., github-api, slack-api
    stubs.yml              # Stub definitions
    responses/             # Optional response body files
      response1.json
      response2.json
```

## Configuration

### Auto-load from tomato.yml

```yaml
resources:
  github-mock:
    type: http-server
    options:
      port: 9999
      fixturesPath: tests/fixtures/github-api
      autoLoad: true  # optional, defaults to true if fixturesPath is set
```

### Manual load in scenarios

```gherkin
Given "mock" is a http server
And "mock" loads fixtures from "tests/fixtures/github-api"
```

## Stub Definition Format

### Basic Stub

```yaml
stubs:
  - id: get-user
    method: GET
    path: /api/user
    response:
      status: 200
      headers:
        Content-Type: application/json
      body: '{"id": 123, "login": "octocat"}'
```

### Stub with Body from File

```yaml
stubs:
  - id: list-repos
    method: GET
    path: /api/repos
    response:
      status: 200
      headers:
        Content-Type: application/json
      bodyFile: responses/repos.json
```

### Stub with Path Pattern (Regex)

```yaml
stubs:
  - id: get-repo
    method: GET
    pathPattern: ^/api/repos/[^/]+/[^/]+$
    response:
      status: 200
      bodyFile: responses/repo-detail.json
```

## Conditional Matching

### Header Conditions

```yaml
stubs:
  # Match when Authorization header contains "Bearer "
  - id: authenticated
    method: GET
    path: /api/protected
    conditions:
      headers:
        Authorization:
          contains: "Bearer "
    response:
      status: 200
      body: '{"authorized": true}'

  # Match exact header value
  - id: specific-auth
    method: GET
    path: /api/admin
    conditions:
      headers:
        Authorization:
          equals: "Bearer admin-token"
    response:
      status: 200
      body: '{"admin": true}'

  # Match header with regex
  - id: pattern-auth
    method: GET
    path: /api/users
    conditions:
      headers:
        User-Agent:
          matches: "^Mozilla/.*"
    response:
      status: 200
      body: '{"browser": "detected"}'
```

### Query Parameter Conditions

```yaml
stubs:
  - id: search-page-1
    method: GET
    path: /api/search
    conditions:
      query:
        page: "1"
        per_page: "10"
    response:
      status: 200
      bodyFile: responses/search-page-1.json

  - id: search-page-2
    method: GET
    path: /api/search
    conditions:
      query:
        page: "2"
        per_page: "10"
    response:
      status: 200
      bodyFile: responses/search-page-2.json
```

### Request Body Conditions

#### JSONPath Matching

```yaml
stubs:
  # Match specific field value
  - id: create-bug-issue
    method: POST
    path: /api/issues
    conditions:
      body:
        jsonPath: $.labels[0]
        equals: "bug"
    response:
      status: 201
      body: '{"id": 1, "labels": ["bug"]}'

  # Match field contains value
  - id: email-domain
    method: POST
    path: /api/users
    conditions:
      body:
        jsonPath: $.email
        contains: "@github.com"
    response:
      status: 201
      body: '{"provider": "github"}'

  # Match field with regex
  - id: webhook-push
    method: POST
    path: /api/webhook
    conditions:
      body:
        jsonPath: $.ref
        matches: "^refs/heads/.*"
    response:
      status: 202
      body: '{"status": "accepted"}'
```

#### Simple Body Matching

```yaml
stubs:
  # Match body contains string
  - id: body-contains
    method: POST
    path: /api/data
    conditions:
      body:
        bodyContains: "important"
    response:
      status: 200
      body: '{"processed": true}'

  # Match body with regex
  - id: body-pattern
    method: POST
    path: /api/data
    conditions:
      body:
        bodyMatches: ".*error.*"
    response:
      status: 400
      body: '{"error": "invalid"}'
```

### Multiple Conditions (AND Logic)

All conditions must match for the stub to be selected:

```yaml
stubs:
  - id: complex-match
    method: POST
    path: /api/webhook
    conditions:
      headers:
        X-GitHub-Event:
          equals: "push"
        Authorization:
          contains: "Bearer "
      body:
        jsonPath: $.ref
        equals: "refs/heads/main"
    response:
      status: 202
      body: '{"status": "accepted"}'
```

## Stub Matching Priority

When a request is received, stubs are matched in this order:

1. **Dynamic stubs** (added via Gherkin steps like `stub "GET" "/path"`)
   - Highest priority
   - Useful for scenario-specific overrides

2. **Fixture stubs with conditions** (most specific first)
   - Matched by number of conditions
   - More conditions = more specific = higher priority

3. **Fixture stubs without conditions**
   - General fallbacks

4. **404 Not Found**
   - No match

## Reset Behavior

- **Fixture stubs**: Preserved across scenarios (persistent)
- **Dynamic stubs**: Cleared on scenario reset
- **Recorded calls**: Cleared on scenario reset

This allows fixtures to provide a consistent baseline while enabling scenario-specific customization.

## Example: Mocking GitHub API

**Directory structure:**
```
fixtures/github-api/
  stubs.yml
  responses/
    user.json
    repos.json
    repo-detail.json
```

**stubs.yml:**
```yaml
stubs:
  - id: get-user
    method: GET
    path: /api/user
    response:
      status: 200
      headers:
        Content-Type: application/json
      bodyFile: responses/user.json

  - id: list-repos
    method: GET
    path: /api/user/repos
    response:
      status: 200
      headers:
        Content-Type: application/json
      bodyFile: responses/repos.json

  - id: get-repo
    method: GET
    pathPattern: ^/api/repos/[^/]+/[^/]+$
    response:
      status: 200
      bodyFile: responses/repo-detail.json

  - id: authenticated-emails
    method: GET
    path: /api/user/emails
    conditions:
      headers:
        Authorization:
          contains: "Bearer "
    response:
      status: 200
      body: '[{"email": "user@example.com", "verified": true}]'

  - id: unauthenticated
    method: GET
    path: /api/user/emails
    response:
      status: 401
      body: '{"message": "Requires authentication"}'
```

**Usage in tests:**
```gherkin
Scenario: Test GitHub integration
  Given "github-mock" is a http server
  And "github-mock" loads fixtures from "tests/fixtures/github-api"
  When "app" calls GitHub API
  Then "github-mock" received "GET" "/api/user"
  And response should contain user data

Scenario: Override fixture for specific test
  Given "github-mock" is a http server
  And "github-mock" loads fixtures from "tests/fixtures/github-api"
  # Override with scenario-specific data
  And "github-mock" stub "GET" "/api/user" returns "200" with json:
    """
    {"id": 999, "login": "testuser"}
    """
  When "app" calls GitHub API
  Then response should have test user data
```

## Tips

1. **Use bodyFile for large responses**: Keep YAML readable
2. **Most specific stubs first**: More conditions = higher priority
3. **Use path patterns for dynamic routes**: `/repos/{owner}/{repo}`
4. **Combine fixtures with dynamic stubs**: Base fixtures + scenario overrides
5. **Test your fixtures**: Verify they load correctly with unit tests
6. **Document your stubs**: Use descriptive IDs and comments

## Troubleshooting

### Fixtures not loading

Check:
- File path is correct (relative to project root)
- `stubs.yml` exists in the fixtures directory
- YAML syntax is valid
- Referenced `bodyFile` paths exist

### Wrong stub matching

Check:
- Stub priority (dynamic > conditional fixtures > simple fixtures)
- Condition syntax (JSONPath, regex patterns)
- Request actually matches conditions (headers, query params, body)

### Enable verbose logging

Run tests with `-v` flag to see detailed stub matching:
```bash
tomato run -v -c tests/tomato.yml
```
