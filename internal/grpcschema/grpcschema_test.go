package grpcschema

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/descriptorpb"
)

func writeSet(t *testing.T, files ...*descriptorpb.FileDescriptorProto) string {
	t.Helper()
	b, err := proto.Marshal(&descriptorpb.FileDescriptorSet{File: files})
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "api.binpb")
	if err := os.WriteFile(path, b, 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// greeter imports what it is given and declares one service.
func greeter(deps ...string) *descriptorpb.FileDescriptorProto {
	fd := &descriptorpb.FileDescriptorProto{
		Name:       proto.String("greet/v1/greet.proto"),
		Package:    proto.String("greet.v1"),
		Syntax:     proto.String("proto3"),
		Dependency: deps,
		MessageType: []*descriptorpb.DescriptorProto{{
			Name: proto.String("Hello"),
			Field: []*descriptorpb.FieldDescriptorProto{{
				Name: proto.String("name"), Number: proto.Int32(1),
				Label: descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
				Type:  descriptorpb.FieldDescriptorProto_TYPE_STRING.Enum(),
			}},
		}},
		Service: []*descriptorpb.ServiceDescriptorProto{{
			Name: proto.String("Greeter"),
			Method: []*descriptorpb.MethodDescriptorProto{
				{Name: proto.String("SayHello"), InputType: proto.String(".greet.v1.Hello"), OutputType: proto.String(".greet.v1.Hello")},
				{Name: proto.String("SayBye"), InputType: proto.String(".greet.v1.Hello"), OutputType: proto.String(".greet.v1.Hello")},
			},
		}},
	}
	return fd
}

func TestLoadDescriptorSets(t *testing.T) {
	// Built without --include_imports: the well-known type is borrowed from
	// what tomato has compiled in.
	schema, err := LoadDescriptorSets(writeSet(t, greeter("google/protobuf/timestamp.proto")))
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(schema.ServiceNames(), ","); got != "greet.v1.Greeter" {
		t.Errorf("services = %s", got)
	}
	md, err := schema.FindMethod("/greet.v1.Greeter/SayBye")
	if err != nil {
		t.Fatal(err)
	}
	if MethodName(md) != "greet.v1.Greeter/SayBye" {
		t.Errorf("MethodName = %s", MethodName(md))
	}
	if _, err := schema.Types.FindMessageByName("greet.v1.Hello"); err != nil {
		t.Errorf("message types are registered for Any: %v", err)
	}

	for target, want := range map[string]string{
		"greet.v1.Greeter/Wave":   `has no method "Wave" (has: SayHello, SayBye)`,
		"greet.Greeter/SayHello":  `"greet.Greeter" is not in the descriptor set; it has: greet.v1.Greeter`,
		"greet.v1.Hello/SayHello": `"greet.v1.Hello" is not a service`,
		"SayHello":                `"package.Service/Method"`,
	} {
		if _, err := schema.FindMethod(target); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("FindMethod(%q) = %v, want %q", target, err, want)
		}
	}
}

func TestLoadDescriptorSetsErrors(t *testing.T) {
	if _, err := LoadDescriptorSets(); err == nil {
		t.Error("no paths")
	}
	if _, err := LoadDescriptorSets(filepath.Join(t.TempDir(), "nope.binpb")); err == nil {
		t.Error("missing file")
	}
	if _, err := LoadDescriptorSets(writeSet(t, greeter("acme/money.proto"))); err == nil ||
		!strings.Contains(err.Error(), `imports "acme/money.proto"`) || !strings.Contains(err.Error(), "--include_imports") {
		t.Errorf("missing import: %v", err)
	}
	if _, err := LoadDescriptorSets(writeSet(t)); err == nil || !strings.Contains(err.Error(), "holds no files") {
		t.Errorf("empty set: %v", err)
	}
	garbage := filepath.Join(t.TempDir(), "x.binpb")
	_ = os.WriteFile(garbage, []byte("not a proto \xff\xff"), 0o644)
	if _, err := LoadDescriptorSets(garbage); err == nil || !strings.Contains(err.Error(), "buf build") {
		t.Errorf("garbage: %v", err)
	}
	bad := greeter()
	bad.Service[0].Method[0].InputType = proto.String(".greet.v1.Missing")
	if _, err := NewSchema(bad); err == nil {
		t.Error("an unresolvable type must fail")
	}
}

// Two sets naming the same file merge, the later one winning.
func TestLoadSeveralSets(t *testing.T) {
	a, b := writeSet(t, greeter()), writeSet(t, greeter())
	schema, err := LoadDescriptorSets(a, b)
	if err != nil {
		t.Fatal(err)
	}
	if len(schema.Services()) != 1 {
		t.Errorf("services = %v", schema.ServiceNames())
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
		svc, m, err := SplitMethod(tc.in)
		if tc.wantErr {
			if err == nil {
				t.Errorf("SplitMethod(%q): want an error", tc.in)
			}
			continue
		}
		if err != nil {
			t.Errorf("SplitMethod(%q): %v", tc.in, err)
			continue
		}
		if svc != tc.service || m != tc.method {
			t.Errorf("SplitMethod(%q) = %q, %q; want %q, %q", tc.in, svc, m, tc.service, tc.method)
		}
	}
}

func TestParseCode(t *testing.T) {
	for in, want := range map[string]codes.Code{
		"NOT_FOUND": codes.NotFound, "NotFound": codes.NotFound, "not_found": codes.NotFound,
		"CANCELLED": codes.Canceled, "Canceled": codes.Canceled, "14": codes.Unavailable, " ok ": codes.OK,
	} {
		got, err := ParseCode(in)
		if err != nil || got != want {
			t.Errorf("ParseCode(%q) = %v, %v; want %v", in, got, err, want)
		}
	}
	if _, err := ParseCode("17"); err == nil {
		t.Error("17 is not a code")
	}
}
