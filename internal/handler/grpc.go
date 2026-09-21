package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/cucumber/godog"
	"github.com/tomatool/tomato/internal/config"
	"github.com/tomatool/tomato/internal/container"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/dynamicpb"
)

// GRPC calls unary gRPC methods and asserts on what comes back.
//
// Methods are resolved through server reflection, so a feature file names a
// method the way a human does — "helloworld.Greeter/SayHello" — and no
// .proto file or generated stub has to be checked in next to the tests. The
// server already knows its own schema; keeping a second copy of it beside
// the suite is how that copy goes stale.
//
// Requests and responses are written as JSON and converted with protojson,
// which is what makes every existing JSON assertion in tomato — paths,
// matchers, contains — work unchanged against a protobuf response.
//
// Unary only, for now. Streaming needs its own vocabulary for opening a
// stream, sending over time and asserting on a sequence, which is a
// different design rather than more steps on this one.
type GRPC struct {
	name      string
	config    config.Resource
	container *container.Manager

	target  string
	timeout time.Duration
	conn    *grpc.ClientConn
	reflect *reflectClient

	requestMetadata map[string]string
	requestBody     []byte

	lastStatus   *status.Status
	lastBody     []byte
	lastHeaders  metadata.MD
	lastDuration time.Duration
}

func NewGRPC(name string, cfg config.Resource, cm *container.Manager) (*GRPC, error) {
	return &GRPC{
		name:            name,
		config:          cfg,
		container:       cm,
		requestMetadata: make(map[string]string),
	}, nil
}

func (r *GRPC) Name() string { return r.name }

func (r *GRPC) Init(ctx context.Context) error {
	r.timeout = 30 * time.Second
	if t, ok := r.config.Options["timeout"].(string); ok {
		d, err := time.ParseDuration(t)
		if err != nil {
			return fmt.Errorf("invalid timeout %q: %w", t, err)
		}
		r.timeout = d
	}

	target, err := r.resolveTarget(ctx)
	if err != nil {
		return err
	}
	r.target = target

	creds := insecure.NewCredentials()
	if tls, ok := r.config.Options["tls"].(bool); ok && tls {
		creds = credentials.NewClientTLSFromCert(nil, "")
	}

	conn, err := grpc.NewClient(target, grpc.WithTransportCredentials(creds))
	if err != nil {
		return fmt.Errorf("dialing %s: %w", target, err)
	}
	r.conn = conn
	r.reflect = newReflectClient(conn)
	return nil
}

// resolveTarget picks the host:port to dial: an explicit address, or the
// mapped port of a managed container.
func (r *GRPC) resolveTarget(ctx context.Context) (string, error) {
	if r.config.Address != "" {
		return r.config.Address, nil
	}
	if r.config.Container == "" {
		return "", fmt.Errorf("resource %q needs either an 'address' or a 'container'", r.name)
	}

	host, err := r.container.GetHost(ctx, r.config.Container)
	if err != nil {
		return "", fmt.Errorf("getting container host: %w", err)
	}
	port := "9090"
	if p, ok := r.config.Options["port"].(string); ok {
		port = p
	}
	mapped, err := r.container.GetPort(ctx, r.config.Container, port+"/tcp")
	if err != nil {
		return "", fmt.Errorf("getting container port: %w", err)
	}
	return fmt.Sprintf("%s:%s", host, mapped), nil
}

// Ready reports whether the server answers reflection, which is both a
// liveness check and a check that the one feature this handler depends on is
// actually switched on — a server without it fails here, with an
// explanation, rather than on someone's first call step.
func (r *GRPC) Ready(ctx context.Context) error {
	if r.conn == nil {
		return fmt.Errorf("resource %q is not initialised", r.name)
	}
	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	if _, err := r.reflect.listServices(ctx); err != nil {
		return fmt.Errorf("server reflection check against %s failed: %w"+
			" (the server must register reflection for this resource to resolve methods)", r.target, err)
	}
	return nil
}

func (r *GRPC) Reset(ctx context.Context) error {
	r.requestMetadata = make(map[string]string)
	r.requestBody = nil
	r.lastStatus = nil
	r.lastBody = nil
	r.lastHeaders = nil
	r.lastDuration = 0
	return nil
}

func (r *GRPC) Cleanup(ctx context.Context) error {
	if r.conn != nil {
		return r.conn.Close()
	}
	return nil
}

func (r *GRPC) RegisterSteps(ctx *godog.ScenarioContext) {
	RegisterStepsToGodog(ctx, r.name, r.Steps())
}

// Steps returns the structured step definitions for the gRPC handler.
func (r *GRPC) Steps() StepCategory {
	return StepCategory{
		Name:        "gRPC",
		Description: "Steps for calling unary gRPC methods and validating responses",
		Steps: []StepDef{
			// Request Setup
			{
				Group:       "Request Setup",
				Pattern:     `^"{resource}" metadata "([^"]*)" is "([^"]*)"$`,
				Description: "Set a request metadata entry",
				Example:     `"grpc" metadata "authorization" is "Bearer token"`,
				Handler:     r.setMetadata,
			},
			{
				Group:       "Request Setup",
				Pattern:     `^"{resource}" metadata are:$`,
				Description: "Set multiple metadata entries from table",
				Example:     `"grpc" metadata are:`,
				Handler:     r.setMetadataTable,
			},
			{
				Group:       "Request Setup",
				Pattern:     `^"{resource}" request is:$`,
				Description: "Set the request message as JSON (docstring)",
				Example:     `"grpc" request is:`,
				Handler:     r.setRequestBody,
			},

			// Request Execution
			{
				Group:       "Request Execution",
				Pattern:     `^"{resource}" calls "([^"]*)"$`,
				Description: "Call a unary method using the previously set request",
				Example:     `"grpc" calls "helloworld.Greeter/SayHello"`,
				Handler:     r.call,
			},
			{
				Group:       "Request Execution",
				Pattern:     `^"{resource}" calls "([^"]*)" with:$`,
				Description: "Call a unary method with a JSON request (docstring)",
				Example:     `"grpc" calls "helloworld.Greeter/SayHello" with:`,
				Handler:     r.callWithBody,
			},

			// Response Status
			{
				Group:       "Response Status",
				Pattern:     `^"{resource}" response status is "([^"]*)"$`,
				Description: "Assert the gRPC status code by name (OK, NOT_FOUND, INVALID_ARGUMENT, ...)",
				Example:     `"grpc" response status is "OK"`,
				Handler:     r.responseStatusShouldBe,
			},
			{
				Group:       "Response Status",
				Pattern:     `^"{resource}" call succeeds$`,
				Description: "Assert the call returned OK",
				Example:     `"grpc" call succeeds`,
				Handler:     r.callShouldSucceed,
			},
			{
				Group:       "Response Status",
				Pattern:     `^"{resource}" call fails$`,
				Description: "Assert the call returned any non-OK status",
				Example:     `"grpc" call fails`,
				Handler:     r.callShouldFail,
			},
			{
				Group:       "Response Status",
				Pattern:     `^"{resource}" response error contains "([^"]*)"$`,
				Description: "Assert the status message contains a substring",
				Example:     `"grpc" response error contains "not found"`,
				Handler:     r.responseErrorShouldContain,
			},

			// Response Body
			{
				Group:       "Response Body",
				Pattern:     `^"{resource}" response contains "([^"]*)"$`,
				Description: "Assert the JSON-rendered response contains a substring",
				Example:     `"grpc" response contains "SERVING"`,
				Handler:     r.responseShouldContain,
			},
			{
				Group:       "Response Body",
				Pattern:     `^"{resource}" response does not contain "([^"]*)"$`,
				Description: "Assert the JSON-rendered response does not contain a substring",
				Example:     `"grpc" response does not contain "error"`,
				Handler:     r.responseShouldNotContain,
			},

			// Response JSON
			{
				Group:       "Response JSON",
				Pattern:     `^"{resource}" response json "([^"]*)" is "([^"]*)"$`,
				Description: "Assert a JSON path value on the response message",
				Example:     `"grpc" response json "status" is "SERVING"`,
				Handler:     r.responseJSONPathShouldBe,
			},
			{
				Group:       "Response JSON",
				Pattern:     `^"{resource}" response json "([^"]*)" exists$`,
				Description: "Assert a JSON path exists on the response message",
				Example:     `"grpc" response json "employees[0].email" exists`,
				Handler:     r.responseJSONPathShouldExist,
			},
			{
				Group:       "Response JSON",
				Pattern:     `^"{resource}" response json "([^"]*)" does not exist$`,
				Description: "Assert a JSON path is absent from the response message",
				Example:     `"grpc" response json "nextPageToken" does not exist`,
				Handler:     r.responseJSONPathShouldNotExist,
			},
			{
				Group:       "Response JSON",
				Pattern:     `^"{resource}" response json matches:$`,
				Description: "Assert the exact response structure, with the same matchers the HTTP handler supports",
				Example:     `"grpc" response json matches:`,
				Handler:     r.responseJSONShouldMatch,
			},
			{
				Group:       "Response JSON",
				Pattern:     `^"{resource}" response json contains:$`,
				Description: "Assert the response contains the given fields, ignoring extras",
				Example:     `"grpc" response json contains:`,
				Handler:     r.responseJSONShouldContain,
			},

			// Service Discovery
			{
				Group:       "Service Discovery",
				Pattern:     `^"{resource}" exposes service "([^"]*)"$`,
				Description: "Assert the server advertises a service via reflection",
				Example:     `"grpc" exposes service "grpc.health.v1.Health"`,
				Handler:     r.shouldExposeService,
			},
			{
				Group:       "Service Discovery",
				Pattern:     `^"{resource}" service "([^"]*)" has method "([^"]*)"$`,
				Description: "Assert a service exposes a method",
				Example:     `"grpc" service "grpc.health.v1.Health" has method "Check"`,
				Handler:     r.serviceShouldHaveMethod,
			},

			// Response Timing
			{
				Group:       "Response Timing",
				Pattern:     `^"{resource}" response time is less than "([^"]*)"$`,
				Description: "Assert the call completed within a time budget",
				Example:     `"grpc" response time is less than "500ms"`,
				Handler:     r.responseTimeShouldBeLessThan,
			},
		},
	}
}

// --- request setup ---

func (r *GRPC) setMetadata(key, value string) error {
	r.requestMetadata[ReplaceVariables(key)] = ReplaceVariables(value)
	return nil
}

func (r *GRPC) setMetadataTable(table *godog.Table) error {
	for _, row := range table.Rows {
		if len(row.Cells) < 2 {
			continue
		}
		key := ReplaceVariables(row.Cells[0].Value)
		// Skip a header row written as key/value, which is the usual way
		// people lay these tables out.
		if strings.EqualFold(key, "key") || strings.EqualFold(key, "metadata") {
			continue
		}
		r.requestMetadata[key] = ReplaceVariables(row.Cells[1].Value)
	}
	return nil
}

func (r *GRPC) setRequestBody(body *godog.DocString) error {
	r.requestBody = []byte(ReplaceVariables(body.Content))
	return nil
}

// --- execution ---

func (r *GRPC) call(target string) error {
	return r.invoke(target, r.requestBody)
}

func (r *GRPC) callWithBody(target string, body *godog.DocString) error {
	return r.invoke(target, []byte(ReplaceVariables(body.Content)))
}

// invoke resolves the method, converts the JSON request into a dynamic
// message, calls it, and stores the outcome.
//
// A non-OK status is recorded rather than returned as a step error: asserting
// on a failure is a normal thing for a test to do, and a step that blew up on
// NOT_FOUND would make "this call is rejected" untestable. Only a problem
// with the call itself — an unknown method, a request that is not valid for
// the schema — fails the step.
func (r *GRPC) invoke(target string, body []byte) error {
	if r.conn == nil {
		return fmt.Errorf("resource %q is not initialised", r.name)
	}
	service, method, err := splitMethod(ReplaceVariables(target))
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), r.timeout)
	defer cancel()

	md, err := r.reflect.findMethod(ctx, service, method)
	if err != nil {
		return err
	}
	if md.IsStreamingClient() || md.IsStreamingServer() {
		return fmt.Errorf("%s/%s is a streaming method; this resource supports unary calls only", service, method)
	}

	req := dynamicpb.NewMessage(md.Input())
	if len(strings.TrimSpace(string(body))) > 0 {
		if err := protojson.Unmarshal(body, req); err != nil {
			return fmt.Errorf("request is not valid for %s: %w", md.Input().FullName(), err)
		}
	}
	resp := dynamicpb.NewMessage(md.Output())

	if len(r.requestMetadata) > 0 {
		ctx = metadata.NewOutgoingContext(ctx, metadata.New(r.requestMetadata))
	}

	var header metadata.MD
	start := time.Now()
	callErr := r.conn.Invoke(ctx, "/"+service+"/"+method, req, resp, grpc.Header(&header))
	r.lastDuration = time.Since(start)
	r.lastHeaders = header
	// status.Convert returns nil for a nil error, which would leave a
	// successful call looking like one that never happened. An OK call has a
	// status too.
	if callErr == nil {
		r.lastStatus = status.New(codes.OK, "")
	} else {
		r.lastStatus = status.Convert(callErr)
	}

	if callErr != nil {
		r.lastBody = nil
		return nil
	}

	// EmitDefaultValues keeps zero-valued fields in the output. Without it a
	// field that is legitimately 0, "" or false vanishes, and an assertion
	// on it reads as "path does not exist" rather than "the value is zero".
	out, err := protojson.MarshalOptions{EmitDefaultValues: true}.Marshal(resp)
	if err != nil {
		return fmt.Errorf("rendering response as JSON: %w", err)
	}
	r.lastBody = out
	return nil
}

// --- status assertions ---

func (r *GRPC) responseStatusShouldBe(expected string) error {
	if r.lastStatus == nil {
		return errNoGRPCResponse
	}
	want := strings.ToUpper(strings.TrimSpace(expected))
	got := r.lastStatus.Code().String()
	if !strings.EqualFold(got, want) {
		return fmt.Errorf("expected status %s, got %s: %s", want, got, r.lastStatus.Message())
	}
	return nil
}

func (r *GRPC) callShouldSucceed() error {
	if r.lastStatus == nil {
		return errNoGRPCResponse
	}
	if r.lastStatus.Code() != codes.OK {
		return fmt.Errorf("expected the call to succeed, got %s: %s",
			r.lastStatus.Code(), r.lastStatus.Message())
	}
	return nil
}

func (r *GRPC) callShouldFail() error {
	if r.lastStatus == nil {
		return errNoGRPCResponse
	}
	if r.lastStatus.Code() == codes.OK {
		return fmt.Errorf("expected the call to fail, but it returned OK")
	}
	return nil
}

func (r *GRPC) responseErrorShouldContain(substr string) error {
	if r.lastStatus == nil {
		return errNoGRPCResponse
	}
	msg := r.lastStatus.Message()
	if !strings.Contains(msg, ReplaceVariables(substr)) {
		return fmt.Errorf("expected the error message to contain %q, got %q", substr, msg)
	}
	return nil
}

// --- body assertions ---

func (r *GRPC) responseShouldContain(substr string) error {
	if err := r.haveBody(); err != nil {
		return err
	}
	if !strings.Contains(string(r.lastBody), ReplaceVariables(substr)) {
		return fmt.Errorf("expected the response to contain %q, got: %s", substr, r.lastBody)
	}
	return nil
}

func (r *GRPC) responseShouldNotContain(substr string) error {
	if err := r.haveBody(); err != nil {
		return err
	}
	if strings.Contains(string(r.lastBody), ReplaceVariables(substr)) {
		return fmt.Errorf("expected the response not to contain %q, got: %s", substr, r.lastBody)
	}
	return nil
}

func (r *GRPC) responseJSONPathShouldBe(path, expected string) error {
	if err := r.haveBody(); err != nil {
		return err
	}
	actual, err := jsonPathValue(r.lastBody, path)
	if err != nil {
		return err
	}
	if got := fmt.Sprintf("%v", actual); got != ReplaceVariables(expected) {
		return fmt.Errorf("JSON path %q: expected %q, got %q", path, expected, got)
	}
	return nil
}

func (r *GRPC) responseJSONPathShouldExist(path string) error {
	if err := r.haveBody(); err != nil {
		return err
	}
	_, err := jsonPathValue(r.lastBody, path)
	return err
}

func (r *GRPC) responseJSONPathShouldNotExist(path string) error {
	if err := r.haveBody(); err != nil {
		return err
	}
	if _, err := jsonPathValue(r.lastBody, path); err == nil {
		return fmt.Errorf("JSON path %q exists but should not", path)
	}
	return nil
}

func (r *GRPC) responseJSONShouldMatch(body *godog.DocString) error {
	if err := r.haveBody(); err != nil {
		return err
	}
	var expected, actual any
	if err := json.Unmarshal([]byte(ReplaceVariables(body.Content)), &expected); err != nil {
		return fmt.Errorf("expected JSON is invalid: %w", err)
	}
	if err := json.Unmarshal(r.lastBody, &actual); err != nil {
		return fmt.Errorf("response is not valid JSON: %w", err)
	}
	return CompareJSON(expected, actual, "", false)
}

func (r *GRPC) responseJSONShouldContain(body *godog.DocString) error {
	if err := r.haveBody(); err != nil {
		return err
	}
	var expected, actual any
	if err := json.Unmarshal([]byte(ReplaceVariables(body.Content)), &expected); err != nil {
		return fmt.Errorf("expected JSON is invalid: %w", err)
	}
	if err := json.Unmarshal(r.lastBody, &actual); err != nil {
		return fmt.Errorf("response is not valid JSON: %w", err)
	}
	return CompareJSON(expected, actual, "", true)
}

// --- discovery assertions ---

func (r *GRPC) shouldExposeService(name string) error {
	if r.conn == nil {
		return fmt.Errorf("resource %q is not initialised", r.name)
	}
	ctx, cancel := context.WithTimeout(context.Background(), r.timeout)
	defer cancel()

	services, err := r.reflect.listServices(ctx)
	if err != nil {
		return err
	}
	want := ReplaceVariables(name)
	if slices.Contains(services, want) {
		return nil
	}
	return fmt.Errorf("server does not expose %q; it exposes: %s", want, strings.Join(services, ", "))
}

func (r *GRPC) serviceShouldHaveMethod(service, method string) error {
	if r.conn == nil {
		return fmt.Errorf("resource %q is not initialised", r.name)
	}
	ctx, cancel := context.WithTimeout(context.Background(), r.timeout)
	defer cancel()

	_, err := r.reflect.findMethod(ctx, ReplaceVariables(service), ReplaceVariables(method))
	return err
}

// --- timing ---

func (r *GRPC) responseTimeShouldBeLessThan(duration string) error {
	if r.lastStatus == nil {
		return errNoGRPCResponse
	}
	budget, err := time.ParseDuration(duration)
	if err != nil {
		return fmt.Errorf("invalid duration %q: %w", duration, err)
	}
	if r.lastDuration > budget {
		return fmt.Errorf("call took %s, expected less than %s", r.lastDuration, budget)
	}
	return nil
}

// --- helpers ---

var errNoGRPCResponse = fmt.Errorf("no call has been made yet")

// haveBody reports why there is nothing to assert on, distinguishing "you
// have not called anything" from "the call you made failed" — the second is
// the common one, and saying so saves re-reading the feature file.
func (r *GRPC) haveBody() error {
	if r.lastStatus == nil {
		return errNoGRPCResponse
	}
	if r.lastBody == nil {
		return fmt.Errorf("the call returned %s (%s), so there is no response message to assert on",
			r.lastStatus.Code(), r.lastStatus.Message())
	}
	return nil
}
