// Package grpcserver provides tomato's `grpc-server` resource: stubbing the
// gRPC services the application under test calls, the way `http-server`
// stubs the HTTP ones.
package grpcserver

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/cucumber/godog"
	"github.com/tomatool/tomato/internal/config"
	"github.com/tomatool/tomato/internal/container"
	"github.com/tomatool/tomato/internal/grpcschema"
	"github.com/tomatool/tomato/internal/jsonmatch"
	"github.com/tomatool/tomato/internal/resource"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/reflection"
	reflectionv1 "google.golang.org/grpc/reflection/grpc_reflection_v1"
	reflectionv1alpha "google.golang.org/grpc/reflection/grpc_reflection_v1alpha"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/dynamicpb"
)

// GRPCServer serves the services of a descriptor set and answers their
// methods with stubs a scenario sets up.
//
// The grpc client resolves methods through reflection, because the server it
// calls already knows its schema. A stub has nobody to ask, so it is handed
// the schema as a FileDescriptorSet — what `buf build -o` or `protoc
// --include_imports --descriptor_set_out` write. Knowing the schema is what
// lets stubs and assertions be written as JSON, converted with protojson as
// the client does, and what makes a misspelt method fail at the step that
// names it rather than as a call nobody answers.
//
// It serves reflection itself, from that same schema, so grpcurl and tomato's
// own grpc client can call it.
type GRPCServer struct {
	name      string
	config    config.Resource
	container *container.Manager

	schema *grpcschema.Schema

	listenMu  sync.Mutex
	server    *grpc.Server
	listener  net.Listener
	port      int
	refusing  bool
	serveDone chan struct{}

	stubsMu sync.Mutex
	stubs   []*stub

	callsMu sync.RWMutex
	calls   []*call
}

// stub answers one method: the calls it matches, and its replies in order.
// The last reply repeats once the others are used up.
type stub struct {
	method     protoreflect.MethodDescriptor
	conditions []condition
	replies    []reply
	served     int
}

// reply is one answer: messages to send (one for a unary method, any number
// on a stream), then a status.
type reply struct {
	delay    time.Duration
	messages []streamMessage
	code     codes.Code
	message  string
	// endDelay is waited before the status that ends a stream.
	endDelay time.Duration
}

type streamMessage struct {
	delay time.Duration
	msg   proto.Message
}

// condition is one thing besides the method a call must match.
type condition struct {
	kind  string // metadata or json
	key   string
	value string
}

func (c condition) matches(cl *call) bool {
	switch c.kind {
	case "metadata":
		for _, v := range cl.metadata.Get(c.key) {
			if v == c.value {
				return true
			}
		}
		return false
	case "json":
		actual, err := jsonmatch.Path(cl.request, c.key)
		if err != nil {
			return false
		}
		return jsonmatch.Compare(c.value, actual, c.key, true) == nil
	}
	return false
}

func (c condition) String() string {
	if c.kind == "json" {
		return fmt.Sprintf("request json %q %q", c.key, c.value)
	}
	return fmt.Sprintf("%s %q %q", c.kind, c.key, c.value)
}

// call is a call the server received.
type call struct {
	method   string
	metadata metadata.MD
	request  []byte // the request message as JSON
	time     time.Time
}

func New(name string, cfg config.Resource, cm *container.Manager) (*GRPCServer, error) {
	return &GRPCServer{name: name, config: cfg, container: cm}, nil
}

func (r *GRPCServer) Name() string { return r.name }

// AppEnv makes the server start before the app: an app that dials its
// dependencies, or checks them, while it starts finds this one listening.
// It sets no environment; app.env names the address through
// {{.<name>.address}}.
func (r *GRPCServer) AppEnv() resource.AppEnv { return resource.AppEnv{} }

// ValidateOptions checks what Init will need, loading the descriptor set,
// so `tomato validate` refuses what a run would.
func ValidateOptions(options map[string]any) error {
	if _, err := portOption(options); err != nil {
		return err
	}
	_, err := loadSchema(options)
	return err
}

func portOption(options map[string]any) (int, error) {
	raw, ok := options["port"]
	if !ok || raw == nil {
		return 0, nil
	}
	p, ok := raw.(int)
	if !ok || p < 0 || p > 65535 {
		return 0, fmt.Errorf("options.port must be a port number, got %v", raw)
	}
	return p, nil
}

// descriptorSets reads options.descriptor_set, one path or a list of them.
func descriptorSets(options map[string]any) ([]string, error) {
	switch v := options["descriptor_set"].(type) {
	case string:
		if strings.TrimSpace(v) != "" {
			return []string{v}, nil
		}
	case []any:
		var out []string
		for _, p := range v {
			s, ok := p.(string)
			if !ok || strings.TrimSpace(s) == "" {
				return nil, fmt.Errorf("options.descriptor_set entries must be file paths, got %v", p)
			}
			out = append(out, s)
		}
		if len(out) > 0 {
			return out, nil
		}
	case []string:
		if len(v) > 0 {
			return v, nil
		}
	}
	return nil, fmt.Errorf("options.descriptor_set is required: the services to serve, as a " +
		"FileDescriptorSet (`buf build -o api.binpb`, or `protoc --include_imports --descriptor_set_out=api.binpb`)")
}

func loadSchema(options map[string]any) (*grpcschema.Schema, error) {
	paths, err := descriptorSets(options)
	if err != nil {
		return nil, err
	}
	schema, err := grpcschema.LoadDescriptorSets(paths...)
	if err != nil {
		return nil, err
	}
	if len(schema.Services()) == 0 {
		return nil, fmt.Errorf("%s declares no services", strings.Join(paths, ", "))
	}
	return schema, nil
}

// Init loads the schema and starts serving. It runs twice — before the app
// starts and again with every resource — so the second time does nothing.
func (r *GRPCServer) Init(ctx context.Context) error {
	r.listenMu.Lock()
	defer r.listenMu.Unlock()
	if r.server != nil {
		return nil
	}
	port, err := portOption(r.config.Options)
	if err != nil {
		return fmt.Errorf("resource %q: %w", r.name, err)
	}
	schema, err := loadSchema(r.config.Options)
	if err != nil {
		return fmt.Errorf("resource %q: %w", r.name, err)
	}
	r.schema = schema
	return r.listen(port)
}

// listen opens the port and serves on it with a new grpc.Server: a stopped
// one cannot serve again. After the first time the port is fixed, so the app
// keeps the address it was given.
func (r *GRPCServer) listen(port int) error {
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return fmt.Errorf("resource %q: listening on port %d: %w", r.name, port, err)
	}
	srv := grpc.NewServer()
	for _, sd := range r.schema.Services() {
		srv.RegisterService(r.serviceDesc(sd), r)
	}
	// Reflection answers from the descriptor set, then from what tomato has
	// compiled in: the reflection service's own schema and the well-known
	// types.
	refl := reflection.ServerOptions{
		Services:           srv,
		DescriptorResolver: resolver{r.schema.Files},
		ExtensionResolver:  r.schema.Types,
	}
	reflectionv1.RegisterServerReflectionServer(srv, reflection.NewServerV1(refl))
	reflectionv1alpha.RegisterServerReflectionServer(srv, reflection.NewServer(refl))

	r.server = srv
	r.listener = lis
	r.port = lis.Addr().(*net.TCPAddr).Port
	done := make(chan struct{})
	r.serveDone = done
	go func() {
		defer close(done)
		_ = srv.Serve(lis)
	}()
	return nil
}

// resolver looks a file or symbol up in the descriptor set first, then in
// the descriptors compiled into tomato.
type resolver struct{ files *protoregistry.Files }

func (s resolver) FindFileByPath(path string) (protoreflect.FileDescriptor, error) {
	if fd, err := s.files.FindFileByPath(path); err == nil {
		return fd, nil
	}
	return protoregistry.GlobalFiles.FindFileByPath(path)
}

func (s resolver) FindDescriptorByName(name protoreflect.FullName) (protoreflect.Descriptor, error) {
	if d, err := s.files.FindDescriptorByName(name); err == nil {
		return d, nil
	}
	return protoregistry.GlobalFiles.FindDescriptorByName(name)
}

// serviceDesc registers every method of a service with handlers that look
// up the scenario's stubs. HandlerType is an empty interface, so the server
// accepts this resource as the implementation of any service.
func (r *GRPCServer) serviceDesc(sd protoreflect.ServiceDescriptor) *grpc.ServiceDesc {
	desc := &grpc.ServiceDesc{
		ServiceName: string(sd.FullName()),
		HandlerType: (*any)(nil),
		Metadata:    sd.ParentFile().Path(),
	}
	for i := range sd.Methods().Len() {
		md := sd.Methods().Get(i)
		switch {
		case md.IsStreamingClient():
			desc.Streams = append(desc.Streams, grpc.StreamDesc{
				StreamName:    string(md.Name()),
				Handler:       r.clientStreamHandler(md),
				ClientStreams: true,
				ServerStreams: md.IsStreamingServer(),
			})
		case md.IsStreamingServer():
			desc.Streams = append(desc.Streams, grpc.StreamDesc{
				StreamName:    string(md.Name()),
				Handler:       r.serverStreamHandler(md),
				ServerStreams: true,
			})
		default:
			desc.Methods = append(desc.Methods, grpc.MethodDesc{
				MethodName: string(md.Name()),
				Handler:    r.unaryHandler(md),
			})
		}
	}
	return desc
}

func (r *GRPCServer) unaryHandler(md protoreflect.MethodDescriptor) func(any, context.Context, func(any) error, grpc.UnaryServerInterceptor) (any, error) {
	return func(_ any, ctx context.Context, dec func(any) error, _ grpc.UnaryServerInterceptor) (any, error) {
		req := dynamicpb.NewMessage(md.Input())
		if err := dec(req); err != nil {
			return nil, err
		}
		rep, err := r.answer(ctx, md, req)
		if err != nil {
			return nil, err
		}
		if err := wait(ctx, rep.delay); err != nil {
			return nil, err
		}
		if rep.code != codes.OK {
			return nil, status.Error(rep.code, rep.message)
		}
		if len(rep.messages) == 0 {
			return dynamicpb.NewMessage(md.Output()), nil
		}
		if err := wait(ctx, rep.messages[0].delay); err != nil {
			return nil, err
		}
		return proto.Clone(rep.messages[0].msg), nil
	}
}

func (r *GRPCServer) serverStreamHandler(md protoreflect.MethodDescriptor) func(any, grpc.ServerStream) error {
	return func(_ any, stream grpc.ServerStream) error {
		ctx := stream.Context()
		req := dynamicpb.NewMessage(md.Input())
		if err := stream.RecvMsg(req); err != nil {
			return err
		}
		rep, err := r.answer(ctx, md, req)
		if err != nil {
			return err
		}
		if err := wait(ctx, rep.delay); err != nil {
			return err
		}
		for _, m := range rep.messages {
			if err := wait(ctx, m.delay); err != nil {
				return err
			}
			if err := stream.SendMsg(proto.Clone(m.msg)); err != nil {
				return err
			}
		}
		if err := wait(ctx, rep.endDelay); err != nil {
			return err
		}
		if rep.code != codes.OK {
			return status.Error(rep.code, rep.message)
		}
		return nil
	}
}

// clientStreamHandler records the call and refuses it: a client or
// bidirectional stream needs steps for what arrives over time, which this
// resource does not have yet.
func (r *GRPCServer) clientStreamHandler(md protoreflect.MethodDescriptor) func(any, grpc.ServerStream) error {
	return func(_ any, stream grpc.ServerStream) error {
		r.record(stream.Context(), md, nil)
		return status.Errorf(codes.Unimplemented,
			"tomato grpc-server %q: %s is a client-streaming method, which cannot be stubbed yet",
			r.name, grpcschema.MethodName(md))
	}
}

// wait sleeps for d, or until the caller gives up, as a timing-out
// dependency does: the client sees its own deadline expire.
func wait(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return nil
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-t.C:
		return nil
	case <-ctx.Done():
		return status.FromContextError(ctx.Err()).Err()
	}
}

// record keeps a call for the assertions, before it is answered, so a call
// the client gave up on still counts.
func (r *GRPCServer) record(ctx context.Context, md protoreflect.MethodDescriptor, req proto.Message) *call {
	in, _ := metadata.FromIncomingContext(ctx)
	c := &call{method: grpcschema.MethodName(md), metadata: in.Copy(), time: time.Now()}
	// The request is kept as JSON the way the grpc client renders responses:
	// with zero values, so a field that is 0, "" or false is there to match.
	if req != nil {
		c.request, _ = protojson.MarshalOptions{EmitDefaultValues: true, Resolver: r.schema.Types}.Marshal(req)
	}
	r.callsMu.Lock()
	r.calls = append(r.calls, c)
	r.callsMu.Unlock()
	return c
}

// answer records a call and picks its reply. The most specific stub wins
// (more conditions beat fewer), then the most recently added, so a scenario
// can override a stub its Background set.
func (r *GRPCServer) answer(ctx context.Context, md protoreflect.MethodDescriptor, req proto.Message) (reply, error) {
	c := r.record(ctx, md, req)

	r.stubsMu.Lock()
	defer r.stubsMu.Unlock()
	var best *stub
	var candidates []string
	for _, s := range r.stubs {
		if s.method.FullName() != md.FullName() {
			continue
		}
		if !s.matches(c) {
			candidates = append(candidates, s.describe())
			continue
		}
		if best == nil || len(s.conditions) >= len(best.conditions) {
			best = s
		}
	}
	if best == nil {
		if len(candidates) > 0 {
			return reply{}, status.Errorf(codes.Unimplemented,
				"tomato grpc-server %q: no stub for %s matches this call; stubs for it need: %s; the request was %s",
				r.name, c.method, strings.Join(candidates, "; "), c.request)
		}
		return reply{}, status.Errorf(codes.Unimplemented,
			"tomato grpc-server %q has no stub for %s", r.name, c.method)
	}
	i := min(best.served, len(best.replies)-1)
	best.served++
	return best.replies[i], nil
}

func (s *stub) matches(c *call) bool {
	for _, cond := range s.conditions {
		if !cond.matches(c) {
			return false
		}
	}
	return true
}

func (s *stub) describe() string {
	parts := make([]string, len(s.conditions))
	for i, c := range s.conditions {
		parts[i] = c.String()
	}
	return strings.Join(parts, " and ")
}

func (r *GRPCServer) Ready(ctx context.Context) error {
	r.listenMu.Lock()
	defer r.listenMu.Unlock()
	if r.server == nil {
		return fmt.Errorf("resource %q is not initialised", r.name)
	}
	return nil
}

// Reset clears the stubs and the recorded calls, and opens the port again if
// the scenario before closed it.
func (r *GRPCServer) Reset(ctx context.Context) error {
	r.stubsMu.Lock()
	r.stubs = nil
	r.stubsMu.Unlock()

	r.callsMu.Lock()
	r.calls = nil
	r.callsMu.Unlock()

	return r.acceptConnections()
}

func (r *GRPCServer) RegisterSteps(ctx *godog.ScenarioContext) {
	resource.RegisterStepsToGodog(ctx, r.name, r.Steps())
}

func (r *GRPCServer) Cleanup(ctx context.Context) error {
	r.listenMu.Lock()
	defer r.listenMu.Unlock()
	if r.server != nil && !r.refusing {
		r.server.Stop()
		<-r.serveDone
	}
	return nil
}

// conditions is the optional part of a stub step between the method and
// "returns" or "streams": any number of `with metadata "k" "v"` and
// `with request json "field" "value"`, in any order.
const conditions = `((?: with metadata "[^"]*" "[^"]*"| with request json "[^"]*" "[^"]*")*)`

var conditionRe = regexp.MustCompile(` with (metadata|request json) "([^"]*)" "([^"]*)"`)

func parseConditions(s string) []condition {
	var out []condition
	for _, m := range conditionRe.FindAllStringSubmatch(s, -1) {
		c := condition{kind: "metadata", key: strings.ToLower(resource.ReplaceVariables(m[2])), value: resource.ReplaceVariables(m[3])}
		if m[1] == "request json" {
			c = condition{kind: "json", key: resource.ReplaceVariables(m[2]), value: resource.ReplaceVariables(m[3])}
		}
		out = append(out, c)
	}
	return out
}

// Steps returns the step definitions of the grpc-server resource.
func (r *GRPCServer) Steps() resource.StepCategory {
	return resource.StepCategory{
		Name:        "gRPC Server",
		Description: "Steps for stubbing the gRPC services the application calls",
		Steps: []resource.StepDef{
			// Stub Setup
			{
				Group:       "Stub Setup",
				Pattern:     `^"{resource}" stub "([^"]*)"` + conditions + ` returns json:$`,
				Description: "Creates a stub that answers a method with this response message, written as JSON (protojson). Between the method and \"returns\", `with metadata \"k\" \"v\"` and `with request json \"field\" \"value\"` (a path into the request, with the same @ matchers as json assertions) narrow which calls it answers; the most specific stub wins, then the most recent. On a server-streaming method the message is a stream of one",
				Example:     "\"{resource}\" stub \"users.v1.UserService/GetUser\" with request json \"id\" \"42\" returns json:\n  \"\"\"\n  {\"id\": \"42\", \"name\": \"Alice\"}\n  \"\"\"",
				Handler:     r.stubReturnsJSON,
			},
			{
				Group:       "Stub Setup",
				Pattern:     `^"{resource}" stub "([^"]*)"` + conditions + ` returns status "([^"]*)"(?: with message "([^"]*)")?$`,
				Description: "Creates a stub that fails a method with a gRPC status (NOT_FOUND, PERMISSION_DENIED, UNAVAILABLE, ...) and an optional message",
				Example:     `"{resource}" stub "users.v1.UserService/GetUser" returns status "NOT_FOUND" with message "no user 42"`,
				Handler:     r.stubReturnsStatus,
			},
			{
				Group:       "Stub Setup",
				Pattern:     `^"{resource}" stub "([^"]*)"` + conditions + ` returns in sequence:$`,
				Description: "Creates a stub that answers each call with the next row, and repeats the last row once the rows are used up. Columns: json (the response message), status (default OK) and message for a failure, and delay (e.g. 3s) to answer late, as a slow dependency does; a client whose deadline passes first gets DEADLINE_EXCEEDED",
				Example:     "\"{resource}\" stub \"users.v1.UserService/GetUser\" returns in sequence:\n  | status      | json                         | delay |\n  | UNAVAILABLE |                              |       |\n  | OK          | {\"id\": \"42\"}                 | 2s    |",
				Handler:     r.stubReturnsSequence,
			},
			{
				Group:       "Stub Setup",
				Pattern:     `^"{resource}" stub "([^"]*)"` + conditions + ` streams:$`,
				Description: "Creates a stub that answers a server-streaming method with one message per row, in order, then ends the stream. Columns: json, and delay before a message; a last row with a status (and message) instead of json ends the stream with that status",
				Example:     "\"{resource}\" stub \"users.v1.UserService/WatchUsers\" streams:\n  | json                 | delay |\n  | {\"id\": \"1\"}          |       |\n  | {\"id\": \"2\"}          | 100ms |",
				Handler:     r.stubStreams,
			},

			// Availability
			{
				Group:       "Availability",
				Pattern:     `^"{resource}" refuses connections$`,
				Description: "Close the server's port and its open connections, so the app's calls fail with UNAVAILABLE as when the service is down. Calls already in flight finish",
				Example:     `"{resource}" refuses connections`,
				Handler:     r.refuseConnections,
			},
			{
				Group:       "Availability",
				Pattern:     `^"{resource}" accepts connections$`,
				Description: "Open the port again on the same address, with the stubs kept. Every scenario starts accepting",
				Example:     `"{resource}" accepts connections`,
				Handler:     r.acceptConnections,
			},

			// Verification
			{
				Group:       "Verification",
				Pattern:     `^"{resource}" received "([^"]*)"$`,
				Description: "Asserts a method was called",
				Example:     `"{resource}" received "users.v1.UserService/GetUser"`,
				Handler:     r.received,
			},
			{
				Group:       "Verification",
				Pattern:     `^"{resource}" received "([^"]*)" "(\d+)" times$`,
				Description: "Asserts a method was called N times",
				Example:     `"{resource}" received "users.v1.UserService/GetUser" "2" times`,
				Handler:     r.receivedTimes,
			},
			{
				Group:       "Verification",
				Pattern:     `^"{resource}" received "([^"]*)" with metadata "([^"]*)" "([^"]*)"$`,
				Description: "Asserts a method was called with this exact metadata value, e.g. the user's token the app forwards",
				Example:     `"{resource}" received "users.v1.UserService/GetUser" with metadata "authorization" "Bearer {{token}}"`,
				Handler:     r.receivedWithMetadata,
			},
			{
				Group:       "Verification",
				Pattern:     `^"{resource}" did not receive "([^"]*)"$`,
				Description: "Asserts a method was not called",
				Example:     `"{resource}" did not receive "users.v1.UserService/DeleteUser"`,
				Handler:     r.didNotReceive,
			},
			{
				Group:       "Verification",
				Pattern:     `^"{resource}" received "(\d+)" calls$`,
				Description: "Asserts the total number of calls received, to any method",
				Example:     `"{resource}" received "3" calls`,
				Handler:     r.receivedTotal,
			},

			// Received Request
			{
				Group:       "Received Request",
				Pattern:     `^"{resource}" last "([^"]*)" request json "([^"]*)" saved as "\{\{([^}]+)\}\}"$`,
				Description: "Save a field of the last request message a method received, as JSON (protojson, so field names are lowerCamelCase), e.g. an id the app generated and later refers to",
				Example:     `"{resource}" last "orders.v1.OrderService/CreateOrder" request json "clientOrderId" saved as "{{order_id}}"`,
				Handler:     r.saveLastRequestJSON,
			},
			{
				Group:       "Received Request",
				Pattern:     `^"{resource}" last "([^"]*)" request metadata "([^"]*)" saved as "\{\{([^}]+)\}\}"$`,
				Description: "Save a metadata value of the last call a method received",
				Example:     `"{resource}" last "users.v1.UserService/GetUser" request metadata "x-request-id" saved as "{{request_id}}"`,
				Handler:     r.saveLastRequestMetadata,
			},
			{
				Group:       "Received Request",
				Pattern:     `^"{resource}" last "([^"]*)" request metadata "([^"]*)" is "([^"]*)"$`,
				Description: "Assert a metadata value of the last call a method received",
				Example:     `"{resource}" last "users.v1.UserService/GetUser" request metadata "authorization" is "Bearer {{token}}"`,
				Handler:     r.lastRequestMetadataIs,
			},
			{
				Group:       "Received Request",
				Pattern:     `^"{resource}" last "([^"]*)" request json contains:$`,
				Description: "Assert the last request message a method received contains these JSON fields (extra fields ignored, same matchers as the http resource's \"response json contains\")",
				Example:     "\"{resource}\" last \"users.v1.UserService/ListUsers\" request json contains:\n  \"\"\"\n  {\"pageSize\": 50}\n  \"\"\"",
				Handler:     r.lastRequestJSONContains,
			},

			// Server Info
			{
				Group:       "Server Info",
				Pattern:     `^"{resource}" address is stored in "([^"]*)"$`,
				Description: "Stores the server's address (localhost:port) in a variable for use in other steps",
				Example:     `"{resource}" address is stored in "USERS_ADDR"`,
				Handler:     r.storeAddress,
			},
		},
	}
}

// --- stub setup ---

// method resolves a step's method against the schema.
func (r *GRPCServer) method(target string) (protoreflect.MethodDescriptor, error) {
	if r.schema == nil {
		return nil, fmt.Errorf("resource %q is not initialised", r.name)
	}
	return r.schema.FindMethod(resource.ReplaceVariables(target))
}

func (r *GRPCServer) addStub(md protoreflect.MethodDescriptor, conds string, replies ...reply) {
	r.stubsMu.Lock()
	defer r.stubsMu.Unlock()
	r.stubs = append(r.stubs, &stub{method: md, conditions: parseConditions(conds), replies: replies})
}

// stubbable refuses methods a stub cannot answer yet.
func stubbable(md protoreflect.MethodDescriptor) error {
	if md.IsStreamingClient() {
		return fmt.Errorf("%s is a client-streaming method; grpc-server stubs unary and server-streaming methods only",
			grpcschema.MethodName(md))
	}
	return nil
}

// message parses a response message written as JSON, so a typo in a field
// name fails the step that wrote it rather than the app's call.
func (r *GRPCServer) message(md protoreflect.MethodDescriptor, js string) (proto.Message, error) {
	msg := dynamicpb.NewMessage(md.Output())
	if strings.TrimSpace(js) == "" {
		return msg, nil
	}
	opts := protojson.UnmarshalOptions{Resolver: r.schema.Types}
	if err := opts.Unmarshal([]byte(resource.ReplaceVariables(js)), msg); err != nil {
		return nil, fmt.Errorf("response is not a valid %s: %w", md.Output().FullName(), err)
	}
	return msg, nil
}

func (r *GRPCServer) stubReturnsJSON(target, conds string, doc *godog.DocString) error {
	md, err := r.method(target)
	if err != nil {
		return err
	}
	if err := stubbable(md); err != nil {
		return err
	}
	msg, err := r.message(md, doc.Content)
	if err != nil {
		return err
	}
	r.addStub(md, conds, reply{messages: []streamMessage{{msg: msg}}})
	return nil
}

func (r *GRPCServer) stubReturnsStatus(target, conds, code, message string) error {
	md, err := r.method(target)
	if err != nil {
		return err
	}
	if err := stubbable(md); err != nil {
		return err
	}
	c, err := grpcschema.ParseCode(code)
	if err != nil {
		return err
	}
	if c == codes.OK {
		return fmt.Errorf(`status OK answers with a message: use "returns json:"`)
	}
	r.addStub(md, conds, reply{code: c, message: resource.ReplaceVariables(message)})
	return nil
}

// columns reads a table's header and refuses columns it does not know.
func columns(table *godog.Table, allowed ...string) (map[string]int, error) {
	if len(table.Rows) < 2 {
		return nil, fmt.Errorf("the table needs a header row and at least one row")
	}
	cols := map[string]int{}
	for i, c := range table.Rows[0].Cells {
		name := strings.TrimSpace(c.Value)
		known := false
		for _, a := range allowed {
			known = known || a == name
		}
		if !known {
			return nil, fmt.Errorf("unknown column %q: use %s", name, strings.Join(allowed, ", "))
		}
		cols[name] = i
	}
	return cols, nil
}

func cellFn(cols map[string]int, row *godog.Table) func(i int, name string) string {
	return func(i int, name string) string {
		if idx, ok := cols[name]; ok && idx < len(row.Rows[i].Cells) {
			return strings.TrimSpace(row.Rows[i].Cells[idx].Value)
		}
		return ""
	}
}

func (r *GRPCServer) stubReturnsSequence(target, conds string, table *godog.Table) error {
	md, err := r.method(target)
	if err != nil {
		return err
	}
	if err := stubbable(md); err != nil {
		return err
	}
	cols, err := columns(table, "status", "json", "message", "delay")
	if err != nil {
		return err
	}
	cell := cellFn(cols, table)
	var replies []reply
	for i := 1; i < len(table.Rows); i++ {
		rep, err := r.row(md, cell, i)
		if err != nil {
			return err
		}
		if rep.code == codes.OK {
			// The delay is the reply's; the message goes as soon as it ends.
			rep.messages = []streamMessage{{msg: rep.messages[0].msg}}
		}
		replies = append(replies, rep)
	}
	r.addStub(md, conds, replies...)
	return nil
}

// row reads one row of a sequence or stream table: a message when the status
// is OK, a failure otherwise.
func (r *GRPCServer) row(md protoreflect.MethodDescriptor, cell func(int, string) string, i int) (reply, error) {
	var rep reply
	code := codes.OK
	if s := cell(i, "status"); s != "" {
		c, err := grpcschema.ParseCode(s)
		if err != nil {
			return rep, fmt.Errorf("row %d: %w", i, err)
		}
		code = c
	}
	if d := cell(i, "delay"); d != "" {
		delay, err := time.ParseDuration(d)
		if err != nil {
			return rep, fmt.Errorf("row %d: invalid delay: %w", i, err)
		}
		rep.delay = delay
	}
	if code != codes.OK {
		if cell(i, "json") != "" {
			return rep, fmt.Errorf("row %d: a %s row sends no message; leave json empty", i, grpcschema.CodeName(code))
		}
		rep.code = code
		rep.message = resource.ReplaceVariables(cell(i, "message"))
		return rep, nil
	}
	if cell(i, "message") != "" {
		return rep, fmt.Errorf("row %d: message is the text of a failure status; an OK row has none", i)
	}
	msg, err := r.message(md, cell(i, "json"))
	if err != nil {
		return rep, fmt.Errorf("row %d: %w", i, err)
	}
	rep.messages = []streamMessage{{msg: msg, delay: rep.delay}}
	return rep, nil
}

func (r *GRPCServer) stubStreams(target, conds string, table *godog.Table) error {
	md, err := r.method(target)
	if err != nil {
		return err
	}
	if err := stubbable(md); err != nil {
		return err
	}
	if !md.IsStreamingServer() {
		return fmt.Errorf("%s is a unary method: stub it with \"returns json:\" or \"returns in sequence:\"",
			grpcschema.MethodName(md))
	}
	cols, err := columns(table, "json", "delay", "status", "message")
	if err != nil {
		return err
	}
	cell := cellFn(cols, table)
	var stream reply
	for i := 1; i < len(table.Rows); i++ {
		rep, err := r.row(md, cell, i)
		if err != nil {
			return err
		}
		if rep.code != codes.OK {
			if i != len(table.Rows)-1 {
				return fmt.Errorf("row %d: a status ends the stream, so it goes in the last row", i)
			}
			stream.code, stream.message, stream.endDelay = rep.code, rep.message, rep.delay
			continue
		}
		stream.messages = append(stream.messages, rep.messages...)
	}
	r.addStub(md, conds, stream)
	return nil
}

// --- availability ---

// refuseConnections stops the server: its port closes, and so do the open
// connections, which an app would otherwise keep calling through. Calls in
// flight finish first, as with http-server.
func (r *GRPCServer) refuseConnections() error {
	r.listenMu.Lock()
	defer r.listenMu.Unlock()
	if r.refusing || r.server == nil {
		return nil
	}
	r.server.GracefulStop()
	<-r.serveDone
	r.refusing = true
	return nil
}

func (r *GRPCServer) acceptConnections() error {
	r.listenMu.Lock()
	defer r.listenMu.Unlock()
	if !r.refusing {
		return nil
	}
	if err := r.listen(r.port); err != nil {
		return err
	}
	r.refusing = false
	return nil
}

// --- verification ---

// callsTo returns the calls a method received, after checking the method
// exists, so a misspelt name fails as one rather than as "not received".
func (r *GRPCServer) callsTo(target string) (string, []*call, error) {
	md, err := r.method(target)
	if err != nil {
		return "", nil, err
	}
	name := grpcschema.MethodName(md)
	r.callsMu.RLock()
	defer r.callsMu.RUnlock()
	var out []*call
	for _, c := range r.calls {
		if c.method == name {
			out = append(out, c)
		}
	}
	return name, out, nil
}

// summary names what was received instead, for a failure message.
func (r *GRPCServer) summary() string {
	r.callsMu.RLock()
	defer r.callsMu.RUnlock()
	if len(r.calls) == 0 {
		return "no calls received"
	}
	counts := map[string]int{}
	var order []string
	for _, c := range r.calls {
		if counts[c.method] == 0 {
			order = append(order, c.method)
		}
		counts[c.method]++
	}
	parts := make([]string, len(order))
	for i, m := range order {
		parts[i] = fmt.Sprintf("%s ×%d", m, counts[m])
	}
	return "received: " + strings.Join(parts, ", ")
}

func (r *GRPCServer) received(target string) error {
	name, calls, err := r.callsTo(target)
	if err != nil {
		return err
	}
	if len(calls) == 0 {
		return fmt.Errorf("no %s call received (%s)", name, r.summary())
	}
	return nil
}

func (r *GRPCServer) receivedTimes(target string, times int) error {
	name, calls, err := r.callsTo(target)
	if err != nil {
		return err
	}
	if len(calls) != times {
		return fmt.Errorf("expected %d %s calls, got %d", times, name, len(calls))
	}
	return nil
}

func (r *GRPCServer) receivedWithMetadata(target, key, value string) error {
	name, calls, err := r.callsTo(target)
	if err != nil {
		return err
	}
	if len(calls) == 0 {
		return fmt.Errorf("no %s call received (%s)", name, r.summary())
	}
	key, value = strings.ToLower(resource.ReplaceVariables(key)), resource.ReplaceVariables(value)
	var seen []string
	for _, c := range calls {
		vals := c.metadata.Get(key)
		for _, v := range vals {
			if v == value {
				return nil
			}
		}
		if len(vals) == 0 {
			seen = append(seen, "none")
		} else {
			seen = append(seen, strconv.Quote(strings.Join(vals, ", ")))
		}
	}
	return fmt.Errorf("no %s call with metadata %s %q; got %s", name, key, value, strings.Join(seen, ", "))
}

func (r *GRPCServer) didNotReceive(target string) error {
	name, calls, err := r.callsTo(target)
	if err != nil {
		return err
	}
	if len(calls) > 0 {
		return fmt.Errorf("unexpected %s call received (%d of them)", name, len(calls))
	}
	return nil
}

func (r *GRPCServer) receivedTotal(count int) error {
	r.callsMu.RLock()
	n := len(r.calls)
	r.callsMu.RUnlock()
	if n != count {
		return fmt.Errorf("expected %d calls, got %d (%s)", count, n, r.summary())
	}
	return nil
}

// --- received request ---

func (r *GRPCServer) lastCall(target string) (string, *call, error) {
	name, calls, err := r.callsTo(target)
	if err != nil {
		return "", nil, err
	}
	if len(calls) == 0 {
		return "", nil, fmt.Errorf("no %s call received (%s)", name, r.summary())
	}
	return name, calls[len(calls)-1], nil
}

func (r *GRPCServer) saveLastRequestJSON(target, field, varName string) error {
	name, c, err := r.lastCall(target)
	if err != nil {
		return err
	}
	value, err := jsonmatch.Path(c.request, field)
	if err != nil {
		return fmt.Errorf("%s request: %w\nrequest: %s", name, err, c.request)
	}
	switch v := value.(type) {
	case nil:
		return fmt.Errorf("%s request: %q is null\nrequest: %s", name, field, c.request)
	case string:
		resource.SetVariable(varName, v)
	case float64:
		resource.SetVariable(varName, strconv.FormatFloat(v, 'f', -1, 64))
	case map[string]any, []any:
		b, _ := json.Marshal(v)
		resource.SetVariable(varName, string(b))
	default:
		resource.SetVariable(varName, fmt.Sprintf("%v", v))
	}
	return nil
}

func (r *GRPCServer) lastMetadata(target, key string) (string, string, error) {
	name, c, err := r.lastCall(target)
	if err != nil {
		return "", "", err
	}
	key = strings.ToLower(resource.ReplaceVariables(key))
	vals := c.metadata.Get(key)
	if len(vals) == 0 {
		return "", "", fmt.Errorf("the last %s call has no %s metadata (it has: %s)", name, key, strings.Join(sortedKeys(c.metadata), ", "))
	}
	return name, strings.Join(vals, ", "), nil
}

func sortedKeys(md metadata.MD) []string {
	keys := make([]string, 0, len(md))
	for k := range md {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func (r *GRPCServer) saveLastRequestMetadata(target, key, varName string) error {
	_, value, err := r.lastMetadata(target, key)
	if err != nil {
		return err
	}
	resource.SetVariable(varName, value)
	return nil
}

func (r *GRPCServer) lastRequestMetadataIs(target, key, expected string) error {
	name, value, err := r.lastMetadata(target, key)
	if err != nil {
		return err
	}
	if want := resource.ReplaceVariables(expected); value != want {
		return fmt.Errorf("the last %s call has %s %q, expected %q", name, strings.ToLower(key), value, want)
	}
	return nil
}

func (r *GRPCServer) lastRequestJSONContains(target string, doc *godog.DocString) error {
	name, c, err := r.lastCall(target)
	if err != nil {
		return err
	}
	expectedText := resource.ReplaceVariables(doc.Content)
	var expected, actual any
	if err := json.Unmarshal([]byte(expectedText), &expected); err != nil {
		return fmt.Errorf("invalid expected JSON: %w", err)
	}
	if err := json.Unmarshal(c.request, &actual); err != nil {
		return fmt.Errorf("%s request is not JSON: %w", name, err)
	}
	if err := jsonmatch.CompareDocs("request", expected, actual, strings.TrimSpace(expectedText), strings.TrimSpace(string(c.request)), true); err != nil {
		return fmt.Errorf("%s request: %w\nrequest: %s", name, err, c.request)
	}
	return nil
}

// --- server info ---

func (r *GRPCServer) storeAddress(varName string) error {
	resource.SetVariable(varName, r.Address())
	return nil
}

// Address is where the server listens, as a dial target.
func (r *GRPCServer) Address() string {
	return fmt.Sprintf("localhost:%d", r.port)
}

var (
	_ resource.Handler        = (*GRPCServer)(nil)
	_ resource.AppEnvProvider = (*GRPCServer)(nil)
)
