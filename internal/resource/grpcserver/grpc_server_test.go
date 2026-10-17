package grpcserver

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/cucumber/godog"
	messages "github.com/cucumber/messages/go/v21"
	"github.com/tomatool/tomato/internal/config"
	"github.com/tomatool/tomato/internal/grpcfixture"
	"github.com/tomatool/tomato/internal/resource"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	reflectionv1 "google.golang.org/grpc/reflection/grpc_reflection_v1"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/dynamicpb"
)

const (
	getUser    = grpcfixture.Service + "/GetUser"
	listUsers  = grpcfixture.Service + "/ListUsers"
	watchUsers = grpcfixture.Service + "/WatchUsers"
	importUser = grpcfixture.Service + "/ImportUsers"
)

// descriptorSet writes the fixture's FileDescriptorSet where a suite keeps
// it, and returns its path.
func descriptorSet(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "users.binpb")
	if err := os.WriteFile(path, grpcfixture.SetBytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func startServer(t *testing.T) *GRPCServer {
	t.Helper()
	srv, err := New("users", config.Resource{Type: "grpc-server", Options: map[string]any{
		"descriptor_set": descriptorSet(t),
	}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := srv.Init(context.Background()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = srv.Cleanup(context.Background()) })
	return srv
}

// client is an app calling the stubbed service.
type client struct {
	t      *testing.T
	conn   *grpc.ClientConn
	schema *GRPCServer
}

func dial(t *testing.T, srv *GRPCServer) *client {
	t.Helper()
	conn, err := grpc.NewClient(srv.Address(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	return &client{t: t, conn: conn, schema: srv}
}

func (c *client) method(name string) protoreflect.MethodDescriptor {
	c.t.Helper()
	md, err := c.schema.schema.FindMethod(name)
	if err != nil {
		c.t.Fatal(err)
	}
	return md
}

// call makes a unary call with a JSON request and returns the JSON response
// and the status.
func (c *client) call(method, req string, md ...string) (string, *status.Status) {
	c.t.Helper()
	return c.callCtx(context.Background(), method, req, md...)
}

func (c *client) callCtx(ctx context.Context, method, req string, md ...string) (string, *status.Status) {
	c.t.Helper()
	desc := c.method(method)
	in := dynamicpb.NewMessage(desc.Input())
	if err := protojson.Unmarshal([]byte(req), in); err != nil {
		c.t.Fatal(err)
	}
	out := dynamicpb.NewMessage(desc.Output())
	if len(md) > 0 {
		ctx = metadata.AppendToOutgoingContext(ctx, md...)
	}
	err := c.conn.Invoke(ctx, "/"+method, in, out)
	if err != nil {
		return "", status.Convert(err)
	}
	b, _ := protojson.Marshal(out)
	return string(b), status.New(codes.OK, "")
}

// stream calls a server-streaming method and returns the messages and the
// status that ended the stream.
func (c *client) stream(method, req string) ([]string, *status.Status) {
	c.t.Helper()
	desc := c.method(method)
	s, err := c.conn.NewStream(context.Background(), &grpc.StreamDesc{ServerStreams: true}, "/"+method)
	if err != nil {
		c.t.Fatal(err)
	}
	in := dynamicpb.NewMessage(desc.Input())
	if err := protojson.Unmarshal([]byte(req), in); err != nil {
		c.t.Fatal(err)
	}
	if err := s.SendMsg(in); err != nil {
		c.t.Fatal(err)
	}
	_ = s.CloseSend()
	var got []string
	for {
		out := dynamicpb.NewMessage(desc.Output())
		err := s.RecvMsg(out)
		if err == io.EOF {
			return got, status.New(codes.OK, "")
		}
		if err != nil {
			return got, status.Convert(err)
		}
		b, _ := protojson.Marshal(out)
		got = append(got, string(b))
	}
}

func doc(s string) *godog.DocString { return &godog.DocString{Content: s} }

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

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func wantErr(t *testing.T, err error, contains ...string) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected an error containing %q", contains)
	}
	for _, c := range contains {
		if !strings.Contains(err.Error(), c) {
			t.Errorf("error %q does not contain %q", err, c)
		}
	}
}

func TestStubReturnsJSON(t *testing.T) {
	srv := startServer(t)
	c := dial(t, srv)

	must(t, srv.stubReturnsJSON(getUser, "", doc(`{"id": "42", "name": "Alice", "role": "ROLE_ADMIN", "createdAt": "2026-10-17T10:00:00Z"}`)))
	got, st := c.call(getUser, `{"id": "42"}`)
	if st.Code() != codes.OK {
		t.Fatalf("status %v", st)
	}
	for _, want := range []string{`"name":"Alice"`, `"role":"ROLE_ADMIN"`, `"createdAt":"2026-10-17T10:00:00Z"`} {
		if !strings.Contains(strings.ReplaceAll(got, " ", ""), want) {
			t.Errorf("response %s lacks %s", got, want)
		}
	}
}

func TestNoStubIsUnimplementedNamingTheMethod(t *testing.T) {
	srv := startServer(t)
	c := dial(t, srv)
	_, st := c.call(getUser, `{"id": "1"}`)
	if st.Code() != codes.Unimplemented || !strings.Contains(st.Message(), getUser) {
		t.Fatalf("got %v, want UNIMPLEMENTED naming %s", st, getUser)
	}
	// The call still counts.
	must(t, srv.received(getUser))
}

func TestUnknownServiceOrMethodListsWhatExists(t *testing.T) {
	srv := startServer(t)
	wantErr(t, srv.stubReturnsJSON("tomato.test.v1.UserServce/GetUser", "", doc(`{}`)),
		"not in the descriptor set", grpcfixture.Service)
	wantErr(t, srv.stubReturnsJSON(grpcfixture.Service+"/GetUsr", "", doc(`{}`)),
		`has no method "GetUsr"`, "GetUser, ListUsers, WatchUsers, ImportUsers")
	wantErr(t, srv.received("no-slash"), "package.Service/Method")
	wantErr(t, srv.stubReturnsJSON(getUser, "", doc(`{"nmae": "x"}`)), "tomato.test.v1.User", "nmae")
}

func TestConditionsPickTheStub(t *testing.T) {
	srv := startServer(t)
	c := dial(t, srv)

	must(t, srv.stubReturnsJSON(getUser, "", doc(`{"name": "anyone"}`)))
	must(t, srv.stubReturnsJSON(getUser, ` with request json "id" "42"`, doc(`{"name": "forty-two"}`)))
	must(t, srv.stubReturnsJSON(getUser, ` with request json "id" "42" with metadata "Authorization" "Bearer t"`, doc(`{"name": "authorised"}`)))
	must(t, srv.stubReturnsJSON(getUser, ` with request json "id" "@regex:^9"`, doc(`{"name": "nines"}`)))

	cases := []struct {
		req  string
		md   []string
		want string
	}{
		{`{"id": "1"}`, nil, "anyone"},
		{`{"id": "42"}`, nil, "forty-two"},
		{`{"id": "42"}`, []string{"authorization", "Bearer t"}, "authorised"},
		{`{"id": "99"}`, nil, "nines"},
	}
	for _, tc := range cases {
		got, st := c.call(getUser, tc.req, tc.md...)
		if st.Code() != codes.OK || !strings.Contains(got, tc.want) {
			t.Errorf("%s %v: got %s %v, want %s", tc.req, tc.md, got, st, tc.want)
		}
	}

	// The most recent of two equally specific stubs wins.
	must(t, srv.stubReturnsJSON(getUser, "", doc(`{"name": "override"}`)))
	if got, _ := c.call(getUser, `{"id": "1"}`); !strings.Contains(got, "override") {
		t.Errorf("got %s, want the later stub", got)
	}
}

func TestNoMatchingStubSaysWhatTheStubsNeed(t *testing.T) {
	srv := startServer(t)
	c := dial(t, srv)
	must(t, srv.stubReturnsJSON(getUser, ` with metadata "x-team" "payments"`, doc(`{}`)))
	_, st := c.call(getUser, `{"id": "1"}`)
	if st.Code() != codes.Unimplemented || !strings.Contains(st.Message(), `metadata "x-team" "payments"`) {
		t.Fatalf("got %v", st)
	}
}

func TestStubReturnsStatus(t *testing.T) {
	srv := startServer(t)
	c := dial(t, srv)

	resource.SetVariable("who", "404")
	must(t, srv.stubReturnsStatus(getUser, ` with request json "id" "{{who}}"`, "NOT_FOUND", "no user {{who}}"))
	must(t, srv.stubReturnsStatus(getUser, "", "PermissionDenied", ""))

	if _, st := c.call(getUser, `{"id": "404"}`); st.Code() != codes.NotFound || st.Message() != "no user 404" {
		t.Errorf("got %v", st)
	}
	if _, st := c.call(getUser, `{"id": "1"}`); st.Code() != codes.PermissionDenied {
		t.Errorf("got %v", st)
	}
	wantErr(t, srv.stubReturnsStatus(getUser, "", "NOPE", ""), "unknown status", "NOT_FOUND")
	wantErr(t, srv.stubReturnsStatus(getUser, "", "OK", ""), "returns json")
	wantErr(t, srv.stubReturnsStatus(importUser, "", "INTERNAL", ""), "client-streaming")
}

func TestParseCode(t *testing.T) {
	for in, want := range map[string]codes.Code{
		"NOT_FOUND": codes.NotFound, "NotFound": codes.NotFound, "not_found": codes.NotFound,
		"CANCELLED": codes.Canceled, "Canceled": codes.Canceled, "14": codes.Unavailable, " ok ": codes.OK,
	} {
		got, err := parseCode(in)
		if err != nil || got != want {
			t.Errorf("parseCode(%q) = %v, %v; want %v", in, got, err, want)
		}
	}
	if _, err := parseCode("17"); err == nil {
		t.Error("17 is not a code")
	}
}

func TestSequenceRepeatsTheLastRow(t *testing.T) {
	srv := startServer(t)
	c := dial(t, srv)
	must(t, srv.stubReturnsSequence(getUser, "", table(
		[]string{"status", "message", "json"},
		[]string{"UNAVAILABLE", "warming up", ""},
		[]string{"", "", `{"name": "first"}`},
		[]string{"OK", "", `{"name": "second"}`},
	)))
	if _, st := c.call(getUser, `{}`); st.Code() != codes.Unavailable || st.Message() != "warming up" {
		t.Errorf("call 1: %v", st)
	}
	for i, want := range []string{"first", "second", "second"} {
		if got, st := c.call(getUser, `{}`); st.Code() != codes.OK || !strings.Contains(got, want) {
			t.Errorf("call %d: %s %v, want %s", i+2, got, st, want)
		}
	}
}

func TestSequenceDelayRespectsTheDeadline(t *testing.T) {
	srv := startServer(t)
	c := dial(t, srv)
	must(t, srv.stubReturnsSequence(getUser, "", table(
		[]string{"json", "delay"},
		[]string{`{"name": "slow"}`, "2s"},
		[]string{`{"name": "quick"}`, "20ms"},
	)))
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	start := time.Now()
	if _, st := c.callCtx(ctx, getUser, `{}`); st.Code() != codes.DeadlineExceeded {
		t.Errorf("got %v, want DEADLINE_EXCEEDED", st)
	}
	if time.Since(start) > time.Second {
		t.Errorf("the stub kept the call for %s after the deadline", time.Since(start))
	}
	if got, st := c.call(getUser, `{}`); st.Code() != codes.OK || !strings.Contains(got, "quick") {
		t.Errorf("got %s %v", got, st)
	}
	// Both calls were recorded, the abandoned one too.
	must(t, srv.receivedTimes(getUser, 2))
}

func TestSequenceTableErrors(t *testing.T) {
	srv := startServer(t)
	cases := map[string]*godog.Table{
		"header row":          table([]string{"json"}),
		`unknown column "x"`:  table([]string{"x"}, []string{"1"}),
		"invalid delay":       table([]string{"json", "delay"}, []string{"{}", "soon"}),
		"sends no message":    table([]string{"status", "json"}, []string{"INTERNAL", `{"id": "1"}`}),
		"an OK row has none":  table([]string{"json", "message"}, []string{"{}", "hi"}),
		"unknown status":      table([]string{"status"}, []string{"MAYBE"}),
		"not a valid tomato.": table([]string{"json"}, []string{`{"nope": 1}`}),
	}
	for want, tbl := range cases {
		wantErr(t, srv.stubReturnsSequence(getUser, "", tbl), want)
	}
}

func TestStreams(t *testing.T) {
	srv := startServer(t)
	c := dial(t, srv)

	must(t, srv.stubStreams(watchUsers, ` with request json "team" "payments"`, table(
		[]string{"json", "delay"},
		[]string{`{"id": "1"}`, ""},
		[]string{`{"id": "2"}`, "10ms"},
	)))
	must(t, srv.stubStreams(watchUsers, ` with request json "team" "broken"`, table(
		[]string{"json", "status", "message", "delay"},
		[]string{`{"id": "1"}`, "", "", ""},
		[]string{"", "DATA_LOSS", "gone", "10ms"},
	)))
	must(t, srv.stubReturnsJSON(watchUsers, ` with request json "team" "one"`, doc(`{"id": "only"}`)))

	got, st := c.stream(watchUsers, `{"team": "payments"}`)
	if st.Code() != codes.OK || len(got) != 2 || !strings.Contains(got[1], `"2"`) {
		t.Errorf("payments: %v %v", got, st)
	}
	got, st = c.stream(watchUsers, `{"team": "broken"}`)
	if st.Code() != codes.DataLoss || st.Message() != "gone" || len(got) != 1 {
		t.Errorf("broken: %v %v", got, st)
	}
	got, st = c.stream(watchUsers, `{"team": "one"}`)
	if st.Code() != codes.OK || len(got) != 1 {
		t.Errorf("one: %v %v", got, st)
	}
	must(t, srv.lastRequestJSONContains(watchUsers, doc(`{"team": "one"}`)))

	wantErr(t, srv.stubStreams(getUser, "", table([]string{"json"}, []string{"{}"})), "unary method")
	wantErr(t, srv.stubStreams(watchUsers, "", table(
		[]string{"status", "json"}, []string{"INTERNAL", ""}, []string{"", "{}"})), "last row")
	wantErr(t, srv.stubStreams(watchUsers, "", table([]string{"json"}, []string{`{"x": 1}`})), "row 1")
	wantErr(t, srv.stubStreams(watchUsers, "", table([]string{"nope"}, []string{"1"})), "unknown column")
	wantErr(t, srv.stubStreams(importUser, "", table([]string{"json"}, []string{"{}"})), "client-streaming")
}

func TestClientStreamingIsRefusedAndRecorded(t *testing.T) {
	srv := startServer(t)
	c := dial(t, srv)
	wantErr(t, srv.stubReturnsJSON(importUser, "", doc(`{}`)), "client-streaming")

	s, err := c.conn.NewStream(context.Background(), &grpc.StreamDesc{ClientStreams: true}, "/"+importUser)
	must(t, err)
	_ = s.CloseSend()
	err = s.RecvMsg(dynamicpb.NewMessage(c.method(importUser).Output()))
	if st := status.Convert(err); st.Code() != codes.Unimplemented || !strings.Contains(st.Message(), "client-streaming") {
		t.Fatalf("got %v", err)
	}
	must(t, srv.received(importUser))
}

func TestRefuseAndAcceptConnections(t *testing.T) {
	srv := startServer(t)
	c := dial(t, srv)
	must(t, srv.stubReturnsJSON(getUser, "", doc(`{"name": "up"}`)))
	if _, st := c.call(getUser, `{}`); st.Code() != codes.OK {
		t.Fatalf("before: %v", st)
	}

	must(t, srv.refuseConnections())
	must(t, srv.refuseConnections()) // twice is fine
	// The open connection is closed too, not only the port.
	if _, st := c.call(getUser, `{}`); st.Code() != codes.Unavailable {
		t.Fatalf("refusing: got %v, want UNAVAILABLE", st)
	}

	must(t, srv.acceptConnections())
	must(t, srv.acceptConnections())
	// A fresh client, as the app's channel reconnects on its own backoff.
	if got, st := dial(t, srv).call(getUser, `{}`); st.Code() != codes.OK || !strings.Contains(got, "up") {
		t.Fatalf("accepting again, with the stub kept: %s %v", got, st)
	}

	// Reset opens the port again for the next scenario.
	must(t, srv.refuseConnections())
	must(t, srv.Reset(context.Background()))
	if _, st := dial(t, srv).call(getUser, `{}`); st.Code() != codes.Unimplemented {
		t.Fatalf("after reset the stubs are gone and the server answers: %v", st)
	}
}

func TestVerification(t *testing.T) {
	srv := startServer(t)
	c := dial(t, srv)
	must(t, srv.stubReturnsJSON(getUser, "", doc(`{}`)))
	must(t, srv.stubReturnsJSON(listUsers, "", doc(`{}`)))

	wantErr(t, srv.received(getUser), "no tomato.test.v1.UserService/GetUser call received", "no calls received")
	c.call(getUser, `{"id": "1"}`, "authorization", "Bearer a")
	c.call(getUser, `{"id": "2"}`, "authorization", "Bearer b", "x-request-id", "r-2")
	c.call(listUsers, `{"pageSize": 50}`)

	must(t, srv.received(getUser))
	must(t, srv.receivedTimes(getUser, 2))
	wantErr(t, srv.receivedTimes(getUser, 3), "expected 3", "got 2")
	must(t, srv.receivedWithMetadata(getUser, "Authorization", "Bearer a"))
	wantErr(t, srv.receivedWithMetadata(getUser, "authorization", "Bearer c"), `"Bearer a"`, `"Bearer b"`)
	wantErr(t, srv.receivedWithMetadata(watchUsers, "authorization", "x"), "no tomato.test.v1.UserService/WatchUsers call")
	must(t, srv.didNotReceive(watchUsers))
	wantErr(t, srv.didNotReceive(getUser), "unexpected")
	must(t, srv.receivedTotal(3))
	wantErr(t, srv.receivedTotal(4), "expected 4 calls, got 3", "GetUser ×2", "ListUsers ×1")
	wantErr(t, srv.received(watchUsers), "GetUser ×2")

	// The last call is the one asserted on.
	must(t, srv.lastRequestMetadataIs(getUser, "authorization", "Bearer b"))
	wantErr(t, srv.lastRequestMetadataIs(getUser, "authorization", "Bearer a"), `"Bearer b"`)
	wantErr(t, srv.lastRequestMetadataIs(listUsers, "authorization", "x"), "no authorization metadata", "content-type")
	must(t, srv.saveLastRequestMetadata(getUser, "x-request-id", "rid"))
	if v, _ := resource.GetVariable("rid"); v != "r-2" {
		t.Errorf("rid = %q", v)
	}
	wantErr(t, srv.saveLastRequestMetadata(getUser, "x-missing", "rid"), "no x-missing metadata")

	must(t, srv.saveLastRequestJSON(getUser, "id", "uid"))
	must(t, srv.saveLastRequestJSON(listUsers, "pageSize", "size"))
	if v, _ := resource.GetVariable("uid"); v != "2" {
		t.Errorf("uid = %q", v)
	}
	if v, _ := resource.GetVariable("size"); v != "50" {
		t.Errorf("size = %q", v)
	}
	wantErr(t, srv.saveLastRequestJSON(getUser, "nope.deeper", "x"), "request")
	wantErr(t, srv.saveLastRequestJSON(watchUsers, "team", "x"), "no tomato.test.v1.UserService/WatchUsers call")

	// Zero values are there to assert on, as in the grpc client's responses.
	must(t, srv.lastRequestJSONContains(listUsers, doc(`{"pageSize": "@gt:10", "pageToken": ""}`)))
	wantErr(t, srv.lastRequestJSONContains(listUsers, doc(`{"pageSize": 10}`)), "expected 10")
	wantErr(t, srv.lastRequestJSONContains(listUsers, doc(`not json`)), "invalid expected JSON")

	must(t, srv.Reset(context.Background()))
	must(t, srv.receivedTotal(0))
}

func TestReflectionListsTheStubbedServices(t *testing.T) {
	srv := startServer(t)
	c := dial(t, srv)
	stream, err := reflectionv1.NewServerReflectionClient(c.conn).ServerReflectionInfo(context.Background())
	must(t, err)
	must(t, stream.Send(&reflectionv1.ServerReflectionRequest{MessageRequest: &reflectionv1.ServerReflectionRequest_ListServices{}}))
	resp, err := stream.Recv()
	must(t, err)
	var names []string
	for _, s := range resp.GetListServicesResponse().GetService() {
		names = append(names, s.GetName())
	}
	if !strings.Contains(strings.Join(names, ","), grpcfixture.Service) {
		t.Errorf("services = %v", names)
	}

	// And the file behind it, with its import from tomato's own descriptors.
	for _, symbol := range []string{grpcfixture.Service, "google.protobuf.Timestamp"} {
		must(t, stream.Send(&reflectionv1.ServerReflectionRequest{MessageRequest: &reflectionv1.ServerReflectionRequest_FileContainingSymbol{FileContainingSymbol: symbol}}))
		resp, err = stream.Recv()
		must(t, err)
		files := resp.GetFileDescriptorResponse().GetFileDescriptorProto()
		if len(files) == 0 {
			t.Fatalf("no file for %s: %v", symbol, resp.GetErrorResponse())
		}
		fd := &descriptorpb.FileDescriptorProto{}
		must(t, proto.Unmarshal(files[0], fd))
	}
}

func TestOptions(t *testing.T) {
	path := descriptorSet(t)
	newSrv := func(opts map[string]any) *GRPCServer {
		srv, _ := New("users", config.Resource{Type: "grpc-server", Options: opts}, nil)
		return srv
	}
	wantErr(t, newSrv(nil).ValidateOptions(), "needs options.descriptor_set", "buf build")
	wantErr(t, newSrv(map[string]any{"descriptor_set": "missing.binpb"}).ValidateOptions(), "missing.binpb")
	wantErr(t, newSrv(map[string]any{"descriptor_set": []any{path, 3}}).ValidateOptions(), "must be file paths")
	wantErr(t, newSrv(map[string]any{"descriptor_set": path, "port": "9995"}).ValidateOptions(), "port number")
	wantErr(t, newSrv(map[string]any{"descriptor_set": path, "port": 70000}).Init(context.Background()), "port number")
	must(t, newSrv(map[string]any{"descriptor_set": []any{path}, "port": 0}).ValidateOptions())

	notASet := filepath.Join(t.TempDir(), "users.proto")
	must(t, os.WriteFile(notASet, []byte("syntax = \"proto3\";\n"), 0o644))
	wantErr(t, newSrv(map[string]any{"descriptor_set": notASet}).ValidateOptions(), "FileDescriptorSet")

	// A set with no services has nothing to serve.
	noServices := filepath.Join(t.TempDir(), "empty.binpb")
	b, _ := proto.Marshal(&descriptorpb.FileDescriptorSet{File: []*descriptorpb.FileDescriptorProto{{
		Name: proto.String("empty.proto"), Package: proto.String("empty"), Syntax: proto.String("proto3"),
	}}})
	must(t, os.WriteFile(noServices, b, 0o644))
	wantErr(t, newSrv(map[string]any{"descriptor_set": noServices}).ValidateOptions(), "declares no services")
}

func TestFixedPortAndInitTwice(t *testing.T) {
	// Find a free port, then ask for it.
	probe := startServer(t)
	port := probe.port
	must(t, probe.Cleanup(context.Background()))

	srv, _ := New("users", config.Resource{Type: "grpc-server", Options: map[string]any{
		"descriptor_set": descriptorSet(t), "port": port,
	}}, nil)
	must(t, srv.Init(context.Background()))
	t.Cleanup(func() { _ = srv.Cleanup(context.Background()) })
	// Before the app, then with every resource: the second Init keeps the server.
	must(t, srv.Init(context.Background()))
	must(t, srv.Ready(context.Background()))
	if srv.port != port || srv.Address() != "localhost:"+strconv.Itoa(port) {
		t.Errorf("address = %s, want port %d", srv.Address(), port)
	}
	if env := srv.AppEnv(); len(env.Set) != 0 {
		t.Errorf("AppEnv sets %v", env.Set)
	}
	must(t, srv.storeAddress("USERS_ADDR"))
	if v, _ := resource.GetVariable("USERS_ADDR"); v != srv.Address() {
		t.Errorf("stored %q", v)
	}

	// The port is taken now.
	other, _ := New("other", config.Resource{Type: "grpc-server", Options: map[string]any{
		"descriptor_set": descriptorSet(t), "port": port,
	}}, nil)
	wantErr(t, other.Init(context.Background()), "listening on port")

	notInit, _ := New("x", config.Resource{}, nil)
	wantErr(t, notInit.Ready(context.Background()), "not initialised")
	wantErr(t, notInit.received(getUser), "not initialised")
}

func TestStepsAreWellFormed(t *testing.T) {
	srv, _ := New("resource", resource.DummyConfig(), nil)
	seen := map[string]bool{}
	for _, s := range srv.Steps().Steps {
		if s.Pattern == "" || s.Description == "" || s.Example == "" || s.Handler == nil || s.Group == "" {
			t.Errorf("incomplete step definition: %+v", s)
		}
		if !strings.Contains(s.Pattern, `"{resource}"`) {
			t.Errorf("pattern must be resource-scoped: %q", s.Pattern)
		}
		if seen[s.Pattern] {
			t.Errorf("duplicate pattern %q", s.Pattern)
		}
		seen[s.Pattern] = true
	}
}
