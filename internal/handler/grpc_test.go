package handler

import (
	"context"
	"net"
	"strings"
	"sync"
	"testing"

	"github.com/cucumber/godog"
	messages "github.com/cucumber/messages/go/v21"
	"github.com/tomatool/tomato/internal/config"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/reflection"
	v1alpha "google.golang.org/grpc/reflection/grpc_reflection_v1alpha"
)

// The tests run against a real gRPC server serving grpc.health.v1.Health.
//
// The health service is used deliberately rather than a checked-in test
// proto: it ships inside grpc-go, so these tests need no .proto file, no
// protoc and no generated code, while still exercising the whole path —
// reflection, descriptor resolution, JSON to protobuf and back, enums,
// status codes, and a genuinely streaming method to reject.

type testServer struct {
	addr     string
	stop     func()
	mu       sync.Mutex
	lastMeta metadata.MD
}

// reflectionMode selects which version of the reflection API the test
// server offers, so both the modern path and the fallback are exercised
// against a real server rather than assumed.
type reflectionMode int

const (
	reflectNone        reflectionMode = iota
	reflectBoth                       // what grpc-go's reflection.Register does
	reflectV1AlphaOnly                // what an older server, e.g. grpc-java, offers
)

// startTestServer runs a health server on a random port.
func startTestServer(t *testing.T, mode reflectionMode) *testServer {
	t.Helper()

	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}

	ts := &testServer{addr: lis.Addr().String()}
	srv := grpc.NewServer(grpc.UnaryInterceptor(
		func(ctx context.Context, req any, _ *grpc.UnaryServerInfo, h grpc.UnaryHandler) (any, error) {
			md, _ := metadata.FromIncomingContext(ctx)
			ts.mu.Lock()
			ts.lastMeta = md
			ts.mu.Unlock()
			return h(ctx, req)
		}))

	hs := health.NewServer()
	hs.SetServingStatus("tomato", healthpb.HealthCheckResponse_SERVING)
	hs.SetServingStatus("degraded", healthpb.HealthCheckResponse_NOT_SERVING)
	healthpb.RegisterHealthServer(srv, hs)

	switch mode {
	case reflectBoth:
		reflection.Register(srv)
	case reflectV1AlphaOnly:
		v1alpha.RegisterServerReflectionServer(srv, reflection.NewServer(reflection.ServerOptions{Services: srv}))
	case reflectNone:
	}

	go func() { _ = srv.Serve(lis) }()
	ts.stop = srv.Stop
	t.Cleanup(srv.Stop)
	return ts
}

func (ts *testServer) metadata() metadata.MD {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	return ts.lastMeta
}

func newTestGRPC(t *testing.T, addr string) *GRPC {
	t.Helper()
	h, err := NewGRPC("grpc", config.Resource{Type: "grpc", Address: addr}, nil)
	if err != nil {
		t.Fatalf("NewGRPC: %v", err)
	}
	if err := h.Init(context.Background()); err != nil {
		t.Fatalf("Init: %v", err)
	}
	t.Cleanup(func() { _ = h.Cleanup(context.Background()) })
	return h
}

func doc(s string) *godog.DocString { return &godog.DocString{Content: s} }

// table builds a godog table from rows of cells.
func table(rows ...[]string) *godog.Table {
	t := &godog.Table{}
	for _, r := range rows {
		row := &messages.PickleTableRow{}
		for _, c := range r {
			row.Cells = append(row.Cells, &messages.PickleTableCell{Value: c})
		}
		t.Rows = append(t.Rows, row)
	}
	return t
}

func TestGRPC_ReadyRequiresReflection(t *testing.T) {
	ok := newTestGRPC(t, startTestServer(t, reflectBoth).addr)
	if err := ok.Ready(context.Background()); err != nil {
		t.Fatalf("Ready against a reflection-enabled server: %v", err)
	}

	// A server without reflection must fail at Ready, where the message can
	// say why, rather than on the user's first call step.
	missing := newTestGRPC(t, startTestServer(t, reflectNone).addr)
	err := missing.Ready(context.Background())
	if err == nil {
		t.Fatal("want an error when the server has no reflection service")
	}
	if !strings.Contains(err.Error(), "reflection") {
		t.Fatalf("the error should name reflection as the problem, got: %v", err)
	}
}

func TestGRPC_UnaryCallRoundTrip(t *testing.T) {
	h := newTestGRPC(t, startTestServer(t, reflectBoth).addr)

	if err := h.callWithBody("grpc.health.v1.Health/Check", doc(`{"service": "tomato"}`)); err != nil {
		t.Fatalf("call: %v", err)
	}
	if err := h.callShouldSucceed(); err != nil {
		t.Fatalf("callShouldSucceed: %v", err)
	}
	if err := h.responseStatusShouldBe("OK"); err != nil {
		t.Fatalf("responseStatusShouldBe: %v", err)
	}
	// Enums render as their name, which is what a feature file should assert.
	if err := h.responseJSONPathShouldBe("status", "SERVING"); err != nil {
		t.Fatalf("responseJSONPathShouldBe: %v", err)
	}
	if err := h.responseShouldContain("SERVING"); err != nil {
		t.Fatalf("responseShouldContain: %v", err)
	}
	if err := h.responseJSONShouldMatch(doc(`{"status": "SERVING"}`)); err != nil {
		t.Fatalf("responseJSONShouldMatch: %v", err)
	}
}

func TestGRPC_StatusIsRecordedNotRaised(t *testing.T) {
	h := newTestGRPC(t, startTestServer(t, reflectBoth).addr)

	// An unknown service is NOT_FOUND. Asserting on a rejection is a normal
	// thing to test, so the step must record the status rather than blow up.
	if err := h.callWithBody("grpc.health.v1.Health/Check", doc(`{"service": "nope"}`)); err != nil {
		t.Fatalf("a non-OK status must not fail the call step: %v", err)
	}
	if err := h.callShouldFail(); err != nil {
		t.Fatalf("callShouldFail: %v", err)
	}
	if err := h.responseStatusShouldBe("NotFound"); err != nil {
		t.Fatalf("status name matching should be case-insensitive: %v", err)
	}
	if err := h.callShouldSucceed(); err == nil {
		t.Fatal("callShouldSucceed must fail after a NOT_FOUND")
	}

	// And asserting on a body that does not exist should say why, naming the
	// status, rather than "no response".
	err := h.responseShouldContain("anything")
	if err == nil {
		t.Fatal("want an error when there is no response message")
	}
	if !strings.Contains(err.Error(), "NotFound") {
		t.Fatalf("the error should name the status that caused it, got: %v", err)
	}
}

func TestGRPC_EmitsDefaultValues(t *testing.T) {
	h := newTestGRPC(t, startTestServer(t, reflectBoth).addr)

	// The empty service name is registered as SERVING by health.NewServer,
	// so use one explicitly set to a non-zero status to prove the enum name
	// survives, then rely on EmitDefaultValues for the zero case below.
	if err := h.callWithBody("grpc.health.v1.Health/Check", doc(`{"service": "degraded"}`)); err != nil {
		t.Fatalf("call: %v", err)
	}
	if err := h.responseJSONPathShouldBe("status", "NOT_SERVING"); err != nil {
		t.Fatalf("responseJSONPathShouldBe: %v", err)
	}

	// A field left at its zero value must still appear, otherwise asserting
	// on it reads as "path does not exist" rather than "the value is zero".
	if err := h.callWithBody("grpc.health.v1.Health/Check", doc(`{"service": ""}`)); err != nil {
		t.Fatalf("call: %v", err)
	}
	if err := h.responseJSONPathShouldExist("status"); err != nil {
		t.Fatalf("a zero-valued field must still be present: %v", err)
	}
}

func TestGRPC_RejectsStreamingMethods(t *testing.T) {
	h := newTestGRPC(t, startTestServer(t, reflectBoth).addr)

	// Health/Watch is server-streaming. It must be refused with an
	// explanation, not attempted as a unary call.
	err := h.callWithBody("grpc.health.v1.Health/Watch", doc(`{"service": "tomato"}`))
	if err == nil {
		t.Fatal("want an error for a streaming method")
	}
	if !strings.Contains(err.Error(), "streaming") || !strings.Contains(err.Error(), "unary") {
		t.Fatalf("the error should explain the limitation, got: %v", err)
	}
}

func TestGRPC_UnknownMethodListsWhatExists(t *testing.T) {
	h := newTestGRPC(t, startTestServer(t, reflectBoth).addr)

	err := h.call("grpc.health.v1.Health/Nope")
	if err == nil {
		t.Fatal("want an error for an unknown method")
	}
	// The whole point of having reflection is that we can say what IS there.
	if !strings.Contains(err.Error(), "Check") {
		t.Fatalf("the error should list the methods that exist, got: %v", err)
	}
}

func TestGRPC_InvalidRequestJSONNamesTheMessage(t *testing.T) {
	h := newTestGRPC(t, startTestServer(t, reflectBoth).addr)

	err := h.callWithBody("grpc.health.v1.Health/Check", doc(`{"nonexistent": true}`))
	if err == nil {
		t.Fatal("want an error for a field the message does not have")
	}
	if !strings.Contains(err.Error(), "HealthCheckRequest") {
		t.Fatalf("the error should name the message type, got: %v", err)
	}
}

func TestGRPC_MetadataIsSent(t *testing.T) {
	ts := startTestServer(t, reflectBoth)
	h := newTestGRPC(t, ts.addr)

	if err := h.setMetadata("x-tomato", "hello"); err != nil {
		t.Fatalf("setMetadata: %v", err)
	}
	if err := h.callWithBody("grpc.health.v1.Health/Check", doc(`{"service": "tomato"}`)); err != nil {
		t.Fatalf("call: %v", err)
	}

	got := ts.metadata().Get("x-tomato")
	if len(got) != 1 || got[0] != "hello" {
		t.Fatalf("metadata not sent, server saw: %v", ts.metadata())
	}
}

func TestGRPC_MetadataTable(t *testing.T) {
	ts := startTestServer(t, reflectBoth)
	h := newTestGRPC(t, ts.addr)

	// A header row is the usual way people lay these out, and must not be
	// sent as a metadata entry.
	if err := h.setMetadataTable(table(
		[]string{"key", "value"},
		[]string{"x-one", "1"},
		[]string{"x-two", "2"},
	)); err != nil {
		t.Fatalf("setMetadataTable: %v", err)
	}
	if err := h.callWithBody("grpc.health.v1.Health/Check", doc(`{"service": "tomato"}`)); err != nil {
		t.Fatalf("call: %v", err)
	}

	md := ts.metadata()
	if v := md.Get("x-one"); len(v) != 1 || v[0] != "1" {
		t.Fatalf("x-one not sent: %v", md)
	}
	if v := md.Get("x-two"); len(v) != 1 || v[0] != "2" {
		t.Fatalf("x-two not sent: %v", md)
	}
	if v := md.Get("key"); len(v) != 0 {
		t.Fatalf("the header row leaked into metadata: %v", v)
	}
}

func TestGRPC_ResetClearsState(t *testing.T) {
	h := newTestGRPC(t, startTestServer(t, reflectBoth).addr)

	if err := h.setMetadata("x-tomato", "hello"); err != nil {
		t.Fatalf("setMetadata: %v", err)
	}
	if err := h.callWithBody("grpc.health.v1.Health/Check", doc(`{"service": "tomato"}`)); err != nil {
		t.Fatalf("call: %v", err)
	}
	if err := h.Reset(context.Background()); err != nil {
		t.Fatalf("Reset: %v", err)
	}

	if len(h.requestMetadata) != 0 || h.lastBody != nil || h.lastStatus != nil {
		t.Fatalf("Reset left state behind: %+v", h)
	}
	// Assertions after a reset must report "nothing called yet", not carry
	// the previous scenario's response forward.
	if err := h.callShouldSucceed(); err == nil {
		t.Fatal("want an error asserting on a response after Reset")
	}
}

func TestGRPC_ServiceDiscovery(t *testing.T) {
	h := newTestGRPC(t, startTestServer(t, reflectBoth).addr)

	if err := h.shouldExposeService("grpc.health.v1.Health"); err != nil {
		t.Fatalf("shouldExposeService: %v", err)
	}
	if err := h.serviceShouldHaveMethod("grpc.health.v1.Health", "Check"); err != nil {
		t.Fatalf("serviceShouldHaveMethod: %v", err)
	}

	err := h.shouldExposeService("nope.NotAService")
	if err == nil {
		t.Fatal("want an error for a service that is not exposed")
	}
	if !strings.Contains(err.Error(), "grpc.health.v1.Health") {
		t.Fatalf("the error should list what IS exposed, got: %v", err)
	}
}

func TestGRPC_ResolveTargetRequiresAddressOrContainer(t *testing.T) {
	h, err := NewGRPC("grpc", config.Resource{Type: "grpc"}, nil)
	if err != nil {
		t.Fatalf("NewGRPC: %v", err)
	}
	if err := h.Init(context.Background()); err == nil {
		t.Fatal("want an error when neither address nor container is set")
	}
}

func TestSplitMethod(t *testing.T) {
	cases := []struct {
		in              string
		service, method string
		wantErr         bool
	}{
		{in: "pkg.Service/Method", service: "pkg.Service", method: "Method"},
		{in: "/pkg.Service/Method", service: "pkg.Service", method: "Method"},
		{in: "  pkg.Service/Method  ", service: "pkg.Service", method: "Method"},
		{in: "grpc.health.v1.Health/Check", service: "grpc.health.v1.Health", method: "Check"},
		{in: "no-slash", wantErr: true},
		{in: "trailing/", wantErr: true},
		{in: "/leading-only", wantErr: true},
		{in: "", wantErr: true},
	}
	for _, tc := range cases {
		svc, m, err := splitMethod(tc.in)
		if tc.wantErr {
			if err == nil {
				t.Errorf("splitMethod(%q): want an error", tc.in)
			}
			continue
		}
		if err != nil {
			t.Errorf("splitMethod(%q): %v", tc.in, err)
			continue
		}
		if svc != tc.service || m != tc.method {
			t.Errorf("splitMethod(%q) = %q, %q; want %q, %q", tc.in, svc, m, tc.service, tc.method)
		}
	}
}

func TestGRPC_StepsAreWellFormed(t *testing.T) {
	h, err := NewGRPC("grpc", DummyConfig(), nil)
	if err != nil {
		t.Fatalf("NewGRPC: %v", err)
	}
	cat := h.Steps()
	if cat.Name == "" || len(cat.Steps) == 0 {
		t.Fatal("the handler must publish a named, non-empty step category")
	}
	seen := map[string]bool{}
	for _, s := range cat.Steps {
		if s.Pattern == "" || s.Description == "" || s.Handler == nil {
			t.Errorf("incomplete step definition: %+v", s)
		}
		if !strings.Contains(s.Pattern, "{resource}") {
			t.Errorf("pattern must be resource-scoped: %q", s.Pattern)
		}
		if seen[s.Pattern] {
			t.Errorf("duplicate step pattern: %q", s.Pattern)
		}
		seen[s.Pattern] = true
	}
}

// TestGRPC_FallsBackToV1Alpha covers servers that predate the stable
// reflection API — grpc-java's ProtoReflectionService among them. Without
// the fallback, this handler would simply not work against them.
func TestGRPC_FallsBackToV1Alpha(t *testing.T) {
	h := newTestGRPC(t, startTestServer(t, reflectV1AlphaOnly).addr)

	if err := h.Ready(context.Background()); err != nil {
		t.Fatalf("Ready against a v1alpha-only server: %v", err)
	}
	if err := h.callWithBody("grpc.health.v1.Health/Check", doc(`{"service": "tomato"}`)); err != nil {
		t.Fatalf("call: %v", err)
	}
	if err := h.responseJSONPathShouldBe("status", "SERVING"); err != nil {
		t.Fatalf("responseJSONPathShouldBe: %v", err)
	}
	if err := h.shouldExposeService("grpc.health.v1.Health"); err != nil {
		t.Fatalf("shouldExposeService: %v", err)
	}
}
