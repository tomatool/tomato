---
layout: default
title: gRPC
nav_order: 7
---

# gRPC Configuration

The `grpc` resource calls unary gRPC methods and asserts on what comes back.

Methods are resolved through **server reflection**, so you name a method the
way you would say it out loud — `helloworld.Greeter/SayHello` — and no
`.proto` file or generated stub has to live next to your tests. The server
already knows its own schema; a second copy beside the suite is a copy that
goes stale.

## Requirements

**The server under test must register the reflection service.** This is the
one hard requirement, and it is checked at startup so a server without it
fails with an explanation rather than at your first call step.

In Go:

```go
import "google.golang.org/grpc/reflection"

srv := grpc.NewServer()
pb.RegisterYourServiceServer(srv, impl)
reflection.Register(srv)   // <- this
```

Both the stable `v1` reflection API and the older `v1alpha` are supported;
tomato prefers `v1` and falls back automatically, so servers that predate the
stable API — grpc-java's `ProtoReflectionService`, for instance — work
unchanged.

## Connecting to a fixed address

```yaml
resources:
  grpc:
    type: grpc
    address: "localhost:9090"
```

`address` is a dial target (`host:port`), not a URL — no scheme.

## Connecting to a managed container

```yaml
containers:
  myservice:
    image: myorg/myservice:latest
    ports:
      - "9090/tcp"
    wait_for:
      type: port
      target: "9090"

resources:
  grpc:
    type: grpc
    container: myservice
    options:
      port: "9090"      # container port to map; defaults to 9090
```

## Options

| Option | Default | Description |
|--------|---------|-------------|
| `address` | — | Dial target `host:port`. Either this or `container` is required. |
| `container` | — | Managed container to dial instead of a fixed address. |
| `options.port` | `9090` | Container port to map. Only used with `container`. |
| `options.timeout` | `30s` | Per-call deadline, also used for reflection lookups. |
| `options.tls` | `false` | Dial with TLS using the system roots. Plaintext otherwise. |

## Writing requests and reading responses

Request and response messages are JSON, converted with `protojson`:

```gherkin
When "grpc" calls "helloworld.Greeter/SayHello" with:
  """
  {"name": "tomato"}
  """
Then "grpc" call succeeds
And "grpc" response json "message" is "Hello tomato"
```

Because the response is rendered as JSON, **every JSON assertion tomato
already has works against a protobuf message** — paths, `matches`, `contains`
and the full matcher vocabulary (`@notempty`, `@regex:...`, `@gt:n`, …).

Two conversion details worth knowing:

- **Enums render as their names**, so assert `"SERVING"`, not `2`.
- **Zero values are emitted.** A field that is legitimately `0`, `""` or
  `false` still appears in the response, so asserting on it reads as "the
  value is zero" rather than "the path does not exist".

## Asserting on failures

A non-OK status is a result, not an error. The call step records it and lets
you assert on it, because "this request is rejected" is a normal thing to
test:

```gherkin
When "grpc" calls "helloworld.Greeter/SayHello" with:
  """
  {"name": ""}
  """
Then "grpc" call fails
And "grpc" response status is "INVALID_ARGUMENT"
And "grpc" response error contains "name is required"
```

Status names are matched case-insensitively, so `INVALID_ARGUMENT` and
`InvalidArgument` both work.

Only a problem with the call itself — an unknown method, or a request that is
not valid for the schema — fails the step. Those errors are specific: an
unknown method lists the methods that do exist, and an invalid request names
the message type it failed to parse into.

## Metadata

```gherkin
Given "grpc" metadata "authorization" is "Bearer {{token}}"
And "grpc" metadata are:
  | key          | value   |
  | x-request-id | abc-123 |
```

A `key`/`value` header row is skipped, so tables can be written either way.

## Limitations

**Unary calls only.** Streaming needs its own vocabulary for opening a
stream, sending over time and asserting on a sequence — that is a different
design rather than more steps on this one. A streaming method is refused with
a clear message rather than attempted.
