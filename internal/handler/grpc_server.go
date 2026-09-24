package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/bufbuild/protocompile"
	"github.com/cucumber/godog"
	"github.com/tomatool/tomato/internal/config"
	"github.com/tomatool/tomato/internal/container"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/reflection"
	reflectionpb "google.golang.org/grpc/reflection/grpc_reflection_v1"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/dynamicpb"
)

// GRPCServer is a stub gRPC server: the gRPC counterpart of http-server. The
// app under test calls it as if it were a real dependency; feature files stub
// responses per method and assert on what the app sent.
//
// It needs the services' descriptors, from .proto files (compiled at startup,
// no protoc or generated code needed) or a protoset. It also serves gRPC
// reflection for them, so grpcurl and tomato's own grpc resource can call it.
type GRPCServer struct {
	name      string
	config    config.Resource
	container *container.Manager

	files    *protoregistry.Files
	server   *grpc.Server
	listener net.Listener
	port     int

	mu    sync.Mutex
	stubs map[string]grpcStub // "/pkg.Service/Method" -> stub
	calls []grpcCall
}

type grpcStub struct {
	code     codes.Code
	message  string
	response []byte // JSON; empty means the default (zero) message
}

type grpcCall struct {
	method   string // "pkg.Service/Method"
	body     []byte // request as JSON
	metadata metadata.MD
}

func NewGRPCServer(name string, cfg config.Resource, cm *container.Manager) (*GRPCServer, error) {
	return &GRPCServer{name: name, config: cfg, container: cm, stubs: map[string]grpcStub{}}, nil
}

func (r *GRPCServer) Name() string { return r.name }

// startsBeforeClients marks handlers that other resources connect to, so the
// registry starts them before clients check readiness.
func (r *GRPCServer) startsBeforeClients() {}

func (r *GRPCServer) Init(ctx context.Context) error {
	files, err := r.loadDescriptors(ctx)
	if err != nil {
		return err
	}
	r.files = files

	port := 0
	if p, ok := r.config.Options["port"].(int); ok {
		port = p
	}
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return fmt.Errorf("grpc-server %q: listening on port %d: %w", r.name, port, err)
	}
	r.listener = listener
	r.port = listener.Addr().(*net.TCPAddr).Port

	r.server = grpc.NewServer(grpc.UnknownServiceHandler(r.handle))
	reflectionpb.RegisterServerReflectionServer(r.server, reflection.NewServerV1(reflection.ServerOptions{
		Services:           r,
		DescriptorResolver: files,
	}))
	go func() { _ = r.server.Serve(listener) }()
	return nil
}

// loadDescriptors reads service definitions from options.proto_files (with
// options.import_paths) or options.protoset.
func (r *GRPCServer) loadDescriptors(ctx context.Context) (*protoregistry.Files, error) {
	if path, ok := r.config.Options["protoset"].(string); ok && path != "" {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("grpc-server %q: reading protoset: %w", r.name, err)
		}
		var set descriptorpb.FileDescriptorSet
		if err := proto.Unmarshal(data, &set); err != nil {
			return nil, fmt.Errorf("grpc-server %q: parsing protoset: %w", r.name, err)
		}
		files, err := protodesc.NewFiles(&set)
		if err != nil {
			return nil, fmt.Errorf("grpc-server %q: protoset: %w", r.name, err)
		}
		return files, nil
	}

	protoFiles := stringList(r.config.Options["proto_files"])
	if len(protoFiles) == 0 {
		return nil, fmt.Errorf("grpc-server %q needs options.proto_files or options.protoset", r.name)
	}
	compiler := protocompile.Compiler{
		Resolver: protocompile.WithStandardImports(&protocompile.SourceResolver{
			ImportPaths: stringList(r.config.Options["import_paths"]),
		}),
	}
	compiled, err := compiler.Compile(ctx, protoFiles...)
	if err != nil {
		return nil, fmt.Errorf("grpc-server %q: compiling proto files: %w", r.name, err)
	}
	files := new(protoregistry.Files)
	for _, f := range compiled {
		if err := registerWithImports(files, f); err != nil {
			return nil, fmt.Errorf("grpc-server %q: %w", r.name, err)
		}
	}
	return files, nil
}

func registerWithImports(files *protoregistry.Files, fd protoreflect.FileDescriptor) error {
	if _, err := files.FindFileByPath(fd.Path()); err == nil {
		return nil
	}
	imports := fd.Imports()
	for i := 0; i < imports.Len(); i++ {
		if err := registerWithImports(files, imports.Get(i).FileDescriptor); err != nil {
			return err
		}
	}
	return files.RegisterFile(fd)
}

func stringList(v any) []string {
	items, _ := v.([]any)
	out := make([]string, 0, len(items))
	for _, item := range items {
		if s, ok := item.(string); ok && s != "" {
			out = append(out, s)
		}
	}
	return out
}

// GetServiceInfo lists the stubbable services, for gRPC reflection.
func (r *GRPCServer) GetServiceInfo() map[string]grpc.ServiceInfo {
	info := map[string]grpc.ServiceInfo{}
	r.files.RangeFiles(func(fd protoreflect.FileDescriptor) bool {
		services := fd.Services()
		for i := 0; i < services.Len(); i++ {
			sd := services.Get(i)
			var methods []grpc.MethodInfo
			for j := 0; j < sd.Methods().Len(); j++ {
				m := sd.Methods().Get(j)
				methods = append(methods, grpc.MethodInfo{
					Name:           string(m.Name()),
					IsClientStream: m.IsStreamingClient(),
					IsServerStream: m.IsStreamingServer(),
				})
			}
			info[string(sd.FullName())] = grpc.ServiceInfo{Methods: methods, Metadata: fd.Path()}
		}
		return true
	})
	return info
}

// method looks up "pkg.Service/Method" in the loaded descriptors.
func (r *GRPCServer) method(fullMethod string) (protoreflect.MethodDescriptor, error) {
	service, name, err := splitMethod(strings.TrimPrefix(fullMethod, "/"))
	if err != nil {
		return nil, err
	}
	desc, err := r.files.FindDescriptorByName(protoreflect.FullName(service))
	if err != nil {
		return nil, fmt.Errorf("service %q is not in the loaded proto files", service)
	}
	sd, ok := desc.(protoreflect.ServiceDescriptor)
	if !ok {
		return nil, fmt.Errorf("%q is not a service", service)
	}
	md := sd.Methods().ByName(protoreflect.Name(name))
	if md == nil {
		return nil, fmt.Errorf("service %q has no method %q", service, name)
	}
	return md, nil
}

// handle serves every call: decode the request, record it, reply with the stub.
func (r *GRPCServer) handle(_ any, stream grpc.ServerStream) error {
	fullMethod, ok := grpc.MethodFromServerStream(stream)
	if !ok {
		return status.Error(codes.Internal, "tomato grpc-server: no method in stream")
	}
	md, err := r.method(fullMethod)
	if err != nil {
		return status.Error(codes.Unimplemented, err.Error())
	}
	if md.IsStreamingClient() || md.IsStreamingServer() {
		return status.Errorf(codes.Unimplemented, "tomato grpc-server: %s is a streaming method; only unary methods can be stubbed", fullMethod)
	}

	req := dynamicpb.NewMessage(md.Input())
	if err := stream.RecvMsg(req); err != nil {
		return err
	}
	body, err := protojson.MarshalOptions{EmitDefaultValues: true}.Marshal(req)
	if err != nil {
		return status.Errorf(codes.Internal, "tomato grpc-server: rendering request: %v", err)
	}
	incoming, _ := metadata.FromIncomingContext(stream.Context())

	key := strings.TrimPrefix(fullMethod, "/")
	r.mu.Lock()
	r.calls = append(r.calls, grpcCall{method: key, body: body, metadata: incoming.Copy()})
	stub, stubbed := r.stubs[key]
	r.mu.Unlock()

	if !stubbed {
		return status.Errorf(codes.Unimplemented, "tomato grpc-server %q: no stub for %s", r.name, key)
	}
	if stub.code != codes.OK {
		return status.Error(stub.code, stub.message)
	}
	resp := dynamicpb.NewMessage(md.Output())
	if len(stub.response) > 0 {
		if err := protojson.Unmarshal(stub.response, resp); err != nil {
			return status.Errorf(codes.Internal, "tomato grpc-server: stubbed response is not a valid %s: %v", md.Output().FullName(), err)
		}
	}
	return stream.SendMsg(resp)
}

func (r *GRPCServer) Ready(ctx context.Context) error { return nil }

func (r *GRPCServer) Reset(ctx context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.stubs = map[string]grpcStub{}
	r.calls = nil
	return nil
}

func (r *GRPCServer) Cleanup(ctx context.Context) error {
	if r.server != nil {
		r.server.Stop()
	}
	return nil
}

// Address is the dial target of the stub server, e.g. "localhost:50061".
func (r *GRPCServer) Address() string { return fmt.Sprintf("localhost:%d", r.port) }

func (r *GRPCServer) RegisterSteps(ctx *godog.ScenarioContext) {
	RegisterStepsToGodog(ctx, r.name, r.Steps())
}

// Steps returns the structured step definitions for the gRPC server handler
func (r *GRPCServer) Steps() StepCategory {
	return StepCategory{
		Name:        "gRPC Server",
		Description: "Steps for stubbing gRPC services the app depends on",
		Steps: []StepDef{
			{
				Group:       "Stub Setup",
				Pattern:     `^"{resource}" stub "([^"]*)" returns:$`,
				Description: "Stubs a unary method to return a JSON-encoded response",
				Example:     "\"{resource}\" stub \"payments.v1.Payments/Authorize\" returns:\n  \"\"\"\n  {\"approved\": true, \"authorizationId\": \"auth-1\"}\n  \"\"\"",
				Handler:     r.stubReturns,
			},
			{
				Group:       "Stub Setup",
				Pattern:     `^"{resource}" stub "([^"]*)" returns status "([^"]*)"$`,
				Description: "Stubs a method to fail with a gRPC status code",
				Example:     `"{resource}" stub "payments.v1.Payments/Authorize" returns status "UNAVAILABLE"`,
				Handler:     r.stubStatus,
			},
			{
				Group:       "Stub Setup",
				Pattern:     `^"{resource}" stub "([^"]*)" returns status "([^"]*)" with message "([^"]*)"$`,
				Description: "Stubs a method to fail with a status code and message",
				Example:     `"{resource}" stub "payments.v1.Payments/Authorize" returns status "FAILED_PRECONDITION" with message "card expired"`,
				Handler:     r.stubStatusWithMessage,
			},
			{
				Group:       "Request Verification",
				Pattern:     `^"{resource}" received "([^"]*)"$`,
				Description: "Asserts the method was called at least once",
				Example:     `"{resource}" received "payments.v1.Payments/Authorize"`,
				Handler:     r.receivedMethod,
			},
			{
				Group:       "Request Verification",
				Pattern:     `^"{resource}" received "([^"]*)" "(\d+)" times$`,
				Description: "Asserts the method was called exactly N times",
				Example:     `"{resource}" received "payments.v1.Payments/Authorize" "2" times`,
				Handler:     r.receivedMethodTimes,
			},
			{
				Group:       "Request Verification",
				Pattern:     `^"{resource}" did not receive "([^"]*)"$`,
				Description: "Asserts the method was never called",
				Example:     `"{resource}" did not receive "payments.v1.Payments/Refund"`,
				Handler:     r.didNotReceiveMethod,
			},
			{
				Group:       "Request Verification",
				Pattern:     `^"{resource}" received "([^"]*)" with json:$`,
				Description: "Asserts the last call to the method contains the JSON fields",
				Example:     "\"{resource}\" received \"payments.v1.Payments/Authorize\" with json:\n  \"\"\"\n  {\"orderId\": \"order-1\", \"amount\": \"4200\"}\n  \"\"\"",
				Handler:     r.receivedMethodWithJSON,
			},
			{
				Group:       "Request Verification",
				Pattern:     `^"{resource}" received metadata "([^"]*)" containing "([^"]*)"$`,
				Description: "Asserts a call carried metadata (a header) containing the value",
				Example:     `"{resource}" received metadata "authorization" containing "Bearer"`,
				Handler:     r.receivedMetadata,
			},
			{
				Group:       "Request Verification",
				Pattern:     `^"{resource}" received "(\d+)" requests$`,
				Description: "Asserts the total number of calls received",
				Example:     `"{resource}" received "3" requests`,
				Handler:     r.receivedRequests,
			},
			{
				Group:       "Utilities",
				Pattern:     `^"{resource}" address is stored in "([^"]*)"$`,
				Description: "Stores the server's host:port in a variable",
				Example:     `"{resource}" address is stored in "PAYMENTS_ADDR"`,
				Handler:     r.storeAddress,
			},
		},
	}
}

func (r *GRPCServer) setStub(target string, stub grpcStub) error {
	key := strings.TrimPrefix(ReplaceVariables(target), "/")
	md, err := r.method(key)
	if err != nil {
		return err
	}
	if md.IsStreamingClient() || md.IsStreamingServer() {
		return fmt.Errorf("%s is a streaming method; only unary methods can be stubbed", key)
	}
	if len(stub.response) > 0 {
		if err := protojson.Unmarshal(stub.response, dynamicpb.NewMessage(md.Output())); err != nil {
			return fmt.Errorf("stubbed response is not a valid %s: %w", md.Output().FullName(), err)
		}
	}
	r.mu.Lock()
	r.stubs[key] = stub
	r.mu.Unlock()
	return nil
}

func (r *GRPCServer) stubReturns(target string, doc *godog.DocString) error {
	return r.setStub(target, grpcStub{code: codes.OK, response: []byte(ReplaceVariables(doc.Content))})
}

func (r *GRPCServer) stubStatus(target, code string) error {
	return r.stubStatusWithMessage(target, code, "")
}

func (r *GRPCServer) stubStatusWithMessage(target, code, message string) error {
	c, err := parseCode(code)
	if err != nil {
		return err
	}
	if message == "" {
		message = "stubbed " + code
	}
	return r.setStub(target, grpcStub{code: c, message: ReplaceVariables(message)})
}

// parseCode accepts a status name ("NOT_FOUND", "not_found", "NotFound") or number.
func parseCode(s string) (codes.Code, error) {
	var c codes.Code
	if n, err := strconv.Atoi(strings.TrimSpace(s)); err == nil && n >= 0 && n <= int(codes.Unauthenticated) {
		return codes.Code(n), nil
	}
	norm := strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(s), " ", "_"))
	if err := c.UnmarshalJSON([]byte(`"` + norm + `"`)); err == nil {
		return c, nil
	}
	for i := codes.OK; i <= codes.Unauthenticated; i++ {
		if strings.EqualFold(strings.ReplaceAll(i.String(), "_", ""), strings.ReplaceAll(s, "_", "")) {
			return i, nil
		}
	}
	return 0, fmt.Errorf("unknown gRPC status %q", s)
}

func (r *GRPCServer) callsTo(target string) []grpcCall {
	key := strings.TrimPrefix(ReplaceVariables(target), "/")
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []grpcCall
	for _, c := range r.calls {
		if c.method == key {
			out = append(out, c)
		}
	}
	return out
}

func (r *GRPCServer) receivedMethod(target string) error {
	if len(r.callsTo(target)) == 0 {
		return fmt.Errorf("expected a call to %s, got none (received: %s)", target, r.receivedSummary())
	}
	return nil
}

func (r *GRPCServer) receivedMethodTimes(target string, n int) error {
	if got := len(r.callsTo(target)); got != n {
		return fmt.Errorf("expected %d calls to %s, got %d", n, target, got)
	}
	return nil
}

func (r *GRPCServer) didNotReceiveMethod(target string) error {
	if got := len(r.callsTo(target)); got > 0 {
		return fmt.Errorf("expected no calls to %s, got %d", target, got)
	}
	return nil
}

func (r *GRPCServer) receivedMethodWithJSON(target string, doc *godog.DocString) error {
	calls := r.callsTo(target)
	if len(calls) == 0 {
		return fmt.Errorf("expected a call to %s, got none (received: %s)", target, r.receivedSummary())
	}
	var expected, actual any
	if err := json.Unmarshal([]byte(ReplaceVariables(doc.Content)), &expected); err != nil {
		return fmt.Errorf("expected JSON is invalid: %w", err)
	}
	last := calls[len(calls)-1]
	if err := json.Unmarshal(last.body, &actual); err != nil {
		return err
	}
	if err := CompareJSON(expected, actual, "", true); err != nil {
		return fmt.Errorf("%w\nlast request: %s", err, last.body)
	}
	return nil
}

func (r *GRPCServer) receivedMetadata(key, value string) error {
	key = strings.ToLower(key)
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, c := range r.calls {
		for _, v := range c.metadata.Get(key) {
			if strings.Contains(v, value) {
				return nil
			}
		}
	}
	return fmt.Errorf("no call carried metadata %q containing %q", key, value)
}

func (r *GRPCServer) receivedRequests(n int) error {
	r.mu.Lock()
	got := len(r.calls)
	r.mu.Unlock()
	if got != n {
		return fmt.Errorf("expected %d requests, got %d", n, got)
	}
	return nil
}

func (r *GRPCServer) storeAddress(name string) error {
	SetVariable(name, r.Address())
	return nil
}

func (r *GRPCServer) receivedSummary() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.calls) == 0 {
		return "nothing"
	}
	seen := map[string]int{}
	for _, c := range r.calls {
		seen[c.method]++
	}
	names := make([]string, 0, len(seen))
	for m, n := range seen {
		names = append(names, fmt.Sprintf("%s×%d", m, n))
	}
	sort.Strings(names)
	return strings.Join(names, ", ")
}

var (
	_ Handler                        = (*GRPCServer)(nil)
	_ reflection.ServiceInfoProvider = (*GRPCServer)(nil)
)
