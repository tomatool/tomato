# gRPC server configuration

The `grpc-server` resource stands in for a gRPC service your application
calls, the way [`http-server`](http-server.md) stands in for an HTTP one. A
scenario stubs the methods it needs, the app calls them as it would the real
service, and the scenario asserts on what the app sent.

## The schema

A stub has no server to ask for its schema over reflection, the way the
[`grpc`](grpc.md) client does, so it is given one: a **FileDescriptorSet**,
the compiled form of your `.proto` files. Either tool writes one:

```bash
buf build -o proto/users.binpb                       # imports included
protoc --include_imports --descriptor_set_out=proto/users.binpb users.proto
```

Commit it next to the suite, or build it in CI before `tomato run`. With
protoc, `--include_imports` matters: a set missing an import fails with the
file it is missing. The well-known types (`google/protobuf/timestamp.proto`
and friends) are the exception; tomato has those built in.

Knowing the schema is what lets stubs and assertions be written as JSON, the
same protojson the `grpc` client uses, and what makes a mistake fail at the
step that makes it: a method that is not in the set fails with the ones that
are, and a stubbed response with a misspelt field fails with the message type
it did not fit.

## Resource configuration

```yaml
resources:
  users:
    type: grpc-server
    options:
      port: 9995
      descriptor_set: proto/users.binpb
```

| Option | Default | Description |
|--------|---------|-------------|
| `descriptor_set` | — | Required. Path to a FileDescriptorSet (`.binpb`, `.pb`), or a list of them. Relative to where tomato runs |
| `port` | `0` (a free port) | Port to listen on. Fix it so `app.env` can name the address |

The server speaks plaintext HTTP/2 (h2c), which is what a gRPC client dialing
with insecure credentials expects. It serves every service in the set, and
gRPC server reflection too, so `grpcurl -plaintext localhost:9995 list` and
tomato's own `grpc` resource work against it.

It starts before the app does, so an app that dials or checks its
dependencies while it starts finds it listening.

## Pointing your app at it

`app.env` names the server through its resource name:

```yaml
app:
  command: go run ./cmd/server
  env:
    USERS_ADDR: "{{.users.address}}"    # localhost:9995
    USERS_HOST: "{{.users.host}}"       # localhost
    USERS_PORT: "{{.users.port}}"       # 9995
```

A gRPC target is `host:port`, not a URL, so the address is the usual one to
use. With the app in a container, the host is `host.docker.internal`.
Templates need a fixed `port`; without one they are left as they are.

## Stubbing

```gherkin
Scenario: The page shows the user's name
  Given "users" stub "users.v1.UserService/GetUser" returns json:
    """
    {"id": "42", "name": "Alice", "role": "ROLE_ADMIN"}
    """
  When "api" sends "GET" to "/profile/42"
  Then "api" response json "name" is "Alice"
```

The docstring is the response message in protojson: field names in
lowerCamelCase (or as in the `.proto`), enums by name, `Timestamp` as an RFC
3339 string.

A failure is a status, with an optional message:

```gherkin
Given "users" stub "users.v1.UserService/GetUser" returns status "NOT_FOUND" with message "no user 42"
```

Status names follow the gRPC spec (`NOT_FOUND`, `PERMISSION_DENIED`,
`UNAVAILABLE`, ...); grpc-go's `NotFound` and the number work too.

### Which calls a stub answers

Between the method and `returns`, conditions narrow the calls a stub answers:

- `with metadata "authorization" "Bearer {{token}}"`: a metadata value, exactly.
- `with request json "id" "42"`: a field of the request message. The path is
  the one JSON assertions use (`user.address.city`, `items[0].sku`), and the
  value takes the same `@` matchers: `"@startswith:secret-"`, `"@gt:10"`.

```gherkin
Given "users" stub "users.v1.UserService/GetUser" returns json:
  """
  {"id": "1", "name": "Bob"}
  """
And "users" stub "users.v1.UserService/GetUser" with request json "id" "404" returns status "NOT_FOUND"
```

The most specific stub wins, then the most recent, so a scenario can override
what its Background stubbed. A call no stub answers gets `UNIMPLEMENTED`, with
a message naming the method and, when stubs for it exist, what they need.

### Over several calls, and slowly

`returns in sequence:` answers each call with the next row and repeats the
last one. A row is a message (`json`) or a failure (`status`, `message`), and
`delay` answers late:

```gherkin
Given "users" stub "users.v1.UserService/GetUser" returns in sequence:
  | status      | message    | json                          | delay |
  | UNAVAILABLE | warming up |                               |       |
  | OK          |            | {"id": "42", "name": "Slow"}  | 3s    |
  | OK          |            | {"id": "42", "name": "Alice"} |       |
```

A delay respects the caller's deadline: an app that gives the call one second
gets `DEADLINE_EXCEEDED` after one second, and the stub stops waiting.

### Server streaming

`streams:` answers a server-streaming method with one message per row, then
ends the stream. A last row with a `status` ends it with that status instead:

```gherkin
Given "users" stub "users.v1.UserService/WatchUsers" streams:
  | json                         | delay | status      | message          |
  | {"id": "1", "name": "Alice"} |       |             |                  |
  | {"id": "2", "name": "Bob"}   | 50ms  |             |                  |
  |                              |       | UNAVAILABLE | connection reset |
```

`returns json:` on a streaming method sends a stream of one message.

## Asserting on what the app sent

```gherkin
Then "users" received "users.v1.UserService/GetUser"
And "users" received "users.v1.UserService/GetUser" "2" times
And "users" did not receive "users.v1.UserService/DeleteUser"
And "users" received "3" calls
And "users" received "users.v1.UserService/GetUser" with metadata "authorization" "Bearer {{token}}"
And "users" last "users.v1.UserService/GetUser" request metadata "authorization" is "Bearer {{token}}"
And "users" last "users.v1.UserService/ListUsers" request json contains:
  """
  {"pageSize": 50}
  """
```

The request is rendered as JSON with its zero values, like the `grpc`
client's responses, so a field that is `0`, `""` or `false` is there to
assert on. A value the app generated can be kept for later steps:

```gherkin
And "users" last "users.v1.UserService/GetUser" request json "id" saved as "{{user_id}}"
And "users" last "users.v1.UserService/GetUser" request metadata "x-request-id" saved as "{{request_id}}"
```

Metadata keys are case-insensitive, as gRPC sends them in lower case.

## The service is down

```gherkin
Given "users" refuses connections
When "api" sends "GET" to "/profile/42"
Then "api" response status is "503"
When "users" accepts connections
```

`refuses connections` closes the port and the connections the app already
has open, so its calls fail with `UNAVAILABLE`; calls in flight finish first.
`accepts connections` opens it again on the same address, with the stubs kept.
A gRPC client backs off before reconnecting, so the app's first call after
that may still fail, depending on its channel settings.

## Reset behavior

Before every scenario the stubs and the recorded calls are cleared, and a
server left refusing connections accepts them again.

## A complete example

```yaml
version: 2

app:
  command: go run ./cmd/server
  port: 8080
  ready:
    type: http
    path: /health
  env:
    USERS_ADDR: "{{.users.address}}"

resources:
  api:
    type: http-client
    base_url: http://localhost:8080
  users:
    type: grpc-server
    options:
      port: 9995
      descriptor_set: proto/users.binpb
```

```gherkin
Feature: Profile page

  Scenario: The user's token reaches the user service
    Given "users" stub "users.v1.UserService/GetUser" with metadata "authorization" "Bearer alice" returns json:
      """
      {"id": "42", "name": "Alice"}
      """
    And "api" header "Authorization" is "Bearer alice"
    When "api" sends "GET" to "/profile/42"
    Then "api" response status is "200"
    And "users" last "users.v1.UserService/GetUser" request json contains:
      """
      {"id": "42"}
      """
```

See [gRPC Server Steps](../resources/grpc-server.md) for every step.

## Limitations

- **Unary and server-streaming methods only.** A client-streaming or
  bidirectional method cannot be stubbed yet: the step says so, and a call to
  one is recorded and answered `UNIMPLEMENTED`. Stubbing those needs steps for
  what arrives over time, which is a design of its own.
- **No response metadata or trailers** on stubbed answers yet.
- **Plaintext only**: no TLS.
- **No `.proto` files directly.** Build a descriptor set from them with buf or
  protoc.
