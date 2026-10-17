// Package grpcfixture is the gRPC service the grpc-server resource's tests
// stub: tomato.test.v1.UserService, a user directory.
//
// It is written as a descriptor rather than compiled from a .proto, so the
// repository needs neither protoc nor buf. The committed copies under
// tests/testdata/grpc — the FileDescriptorSet tomato.yml points at, and a
// .proto rendering of it to read — are checked against this by a test, and
// regenerated with: go test ./internal/grpcfixture -update
//
// The test app calls the service through it too, as an application calls a
// dependency, so the integration suite stubs a real call path.
package grpcfixture

import (
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// Names the tests and the test app use.
const (
	File    = "tomato/test/v1/users.proto"
	Package = "tomato.test.v1"
	Service = Package + ".UserService"
)

// UsersFile is the descriptor of tomato/test/v1/users.proto:
//
//	service UserService {
//	  rpc GetUser(GetUserRequest) returns (User);
//	  rpc ListUsers(ListUsersRequest) returns (ListUsersResponse);
//	  rpc WatchUsers(WatchUsersRequest) returns (stream User);
//	  rpc ImportUsers(stream User) returns (ImportUsersResponse);
//	}
func UsersFile() *descriptorpb.FileDescriptorProto {
	return &descriptorpb.FileDescriptorProto{
		Name:       proto.String(File),
		Package:    proto.String(Package),
		Dependency: []string{"google/protobuf/timestamp.proto"},
		Syntax:     proto.String("proto3"),
		EnumType: []*descriptorpb.EnumDescriptorProto{{
			Name: proto.String("Role"),
			Value: []*descriptorpb.EnumValueDescriptorProto{
				{Name: proto.String("ROLE_UNSPECIFIED"), Number: proto.Int32(0)},
				{Name: proto.String("ROLE_MEMBER"), Number: proto.Int32(1)},
				{Name: proto.String("ROLE_ADMIN"), Number: proto.Int32(2)},
			},
		}},
		MessageType: []*descriptorpb.DescriptorProto{
			message("User",
				field("id", 1, descriptorpb.FieldDescriptorProto_TYPE_STRING, ""),
				field("name", 2, descriptorpb.FieldDescriptorProto_TYPE_STRING, ""),
				field("email", 3, descriptorpb.FieldDescriptorProto_TYPE_STRING, ""),
				field("role", 4, descriptorpb.FieldDescriptorProto_TYPE_ENUM, ".tomato.test.v1.Role"),
				repeated(field("tags", 5, descriptorpb.FieldDescriptorProto_TYPE_STRING, "")),
				field("created_at", 6, descriptorpb.FieldDescriptorProto_TYPE_MESSAGE, ".google.protobuf.Timestamp"),
			),
			message("GetUserRequest",
				field("id", 1, descriptorpb.FieldDescriptorProto_TYPE_STRING, ""),
			),
			message("ListUsersRequest",
				field("page_size", 1, descriptorpb.FieldDescriptorProto_TYPE_INT32, ""),
				field("page_token", 2, descriptorpb.FieldDescriptorProto_TYPE_STRING, ""),
			),
			message("ListUsersResponse",
				repeated(field("users", 1, descriptorpb.FieldDescriptorProto_TYPE_MESSAGE, ".tomato.test.v1.User")),
				field("next_page_token", 2, descriptorpb.FieldDescriptorProto_TYPE_STRING, ""),
			),
			message("WatchUsersRequest",
				field("team", 1, descriptorpb.FieldDescriptorProto_TYPE_STRING, ""),
			),
			message("ImportUsersResponse",
				field("imported", 1, descriptorpb.FieldDescriptorProto_TYPE_INT32, ""),
			),
		},
		Service: []*descriptorpb.ServiceDescriptorProto{{
			Name: proto.String("UserService"),
			Method: []*descriptorpb.MethodDescriptorProto{
				method("GetUser", "GetUserRequest", "User", false, false),
				method("ListUsers", "ListUsersRequest", "ListUsersResponse", false, false),
				method("WatchUsers", "WatchUsersRequest", "User", false, true),
				method("ImportUsers", "User", "ImportUsersResponse", true, false),
			},
		}},
	}
}

// Set is the FileDescriptorSet `buf build -o` or `protoc --include_imports
// --descriptor_set_out` writes for users.proto: the file and what it imports.
func Set() *descriptorpb.FileDescriptorSet {
	return &descriptorpb.FileDescriptorSet{File: []*descriptorpb.FileDescriptorProto{
		protodesc.ToFileDescriptorProto(timestamppb.File_google_protobuf_timestamp_proto),
		UsersFile(),
	}}
}

// SetBytes is Set in the wire format, as committed to tests/testdata/grpc.
func SetBytes() []byte {
	b, err := proto.MarshalOptions{Deterministic: true}.Marshal(Set())
	if err != nil {
		panic(err)
	}
	return b
}

func message(name string, fields ...*descriptorpb.FieldDescriptorProto) *descriptorpb.DescriptorProto {
	return &descriptorpb.DescriptorProto{Name: proto.String(name), Field: fields}
}

func field(name string, number int32, typ descriptorpb.FieldDescriptorProto_Type, typeName string) *descriptorpb.FieldDescriptorProto {
	f := &descriptorpb.FieldDescriptorProto{
		Name:     proto.String(name),
		Number:   proto.Int32(number),
		Label:    descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
		Type:     typ.Enum(),
		JsonName: proto.String(jsonName(name)),
	}
	if typeName != "" {
		f.TypeName = proto.String(typeName)
	}
	return f
}

func repeated(f *descriptorpb.FieldDescriptorProto) *descriptorpb.FieldDescriptorProto {
	f.Label = descriptorpb.FieldDescriptorProto_LABEL_REPEATED.Enum()
	return f
}

func method(name, in, out string, clientStreaming, serverStreaming bool) *descriptorpb.MethodDescriptorProto {
	m := &descriptorpb.MethodDescriptorProto{
		Name:       proto.String(name),
		InputType:  proto.String("." + Package + "." + in),
		OutputType: proto.String("." + Package + "." + out),
	}
	if clientStreaming {
		m.ClientStreaming = proto.Bool(true)
	}
	if serverStreaming {
		m.ServerStreaming = proto.Bool(true)
	}
	return m
}

// jsonName is protoc's lowerCamelCase of a field name, which it records in
// every descriptor it writes.
func jsonName(name string) string {
	out := make([]byte, 0, len(name))
	upper := false
	for i := 0; i < len(name); i++ {
		c := name[i]
		if c == '_' {
			upper = true
			continue
		}
		if upper && 'a' <= c && c <= 'z' {
			c -= 'a' - 'A'
		}
		upper = false
		out = append(out, c)
	}
	return string(out)
}
