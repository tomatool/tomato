# gRPC Server Configuration

`grpc-server` is the gRPC counterpart of [`http-server`](http-server.md): a stub
server for the gRPC services your app depends on. The app calls it like the
real dependency; feature files stub the responses and assert on the calls.

```yaml
resources:
  payments:
    type: grpc-server
    options:
      port: 50061
      proto_files:
        - payments/v1/payments.proto
      import_paths:
        - proto

app:
  command: java -jar build/libs/orders.jar
  env:
    PAYMENTS_GRPC_TARGET: "{{.payments.url}}"   # -> localhost:50061
```

No `protoc` or generated code is needed: tomato compiles the `.proto` files
when it starts (well-known imports such as `google/protobuf/timestamp.proto`
are built in). A compiled descriptor set works too:

```yaml
    options:
      port: 50061
      protoset: build/descriptors.pb   # protoc --include_imports -o build/descriptors.pb ...
```

| Option | Description |
|--------|-------------|
| `port` | Port to listen on. Set it so `{{.name.url}}` can be resolved for the app (otherwise a free port is picked) |
| `proto_files` | `.proto` files defining the services to stub |
| `import_paths` | Directories `proto_files` and their imports are resolved against |
| `protoset` | A `FileDescriptorSet` instead of `proto_files` |

`{{.name.url}}` resolves to `localhost:<port>` for a local app, and to
`host.docker.internal:<port>` for a containerised one. It is a gRPC dial
target (host:port), not an `http://` URL.

## Writing tests

```gherkin
Scenario: An approved payment confirms the order
  Given "payments" stub "payments.v1.Payments/Authorize" returns:
    """
    {"approved": true, "authorizationId": "auth-1"}
    """
  When "api" sends "POST" to "/orders/order-1/pay"
  Then "api" response status is "200"
  And "payments" received "payments.v1.Payments/Authorize" with json:
    """
    {"orderId": "order-1", "amount": "4200"}
    """
  And "payments" received metadata "authorization" containing "Bearer"

Scenario: The payment provider is down
  Given "payments" stub "payments.v1.Payments/Authorize" returns status "UNAVAILABLE"
  When "api" sends "POST" to "/orders/order-1/pay"
  Then "api" response status is "503"
```

- Request and response bodies use the protobuf JSON mapping: lowerCamelCase
  field names, and 64-bit integers as strings (`"amount": "4200"`).
- A method without a stub fails with `UNIMPLEMENTED` and a message naming it,
  so a missing stub is obvious instead of hanging.
- Stubs and recorded calls are cleared before every scenario.
- Status names accept `NOT_FOUND`, `NotFound` or the number (`5`).
- Unary methods only; streaming methods fail with `UNIMPLEMENTED`.

The server also serves gRPC reflection for the loaded services, so `grpcurl`
and tomato's own [`grpc`](grpc.md) resource can call it.
