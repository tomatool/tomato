// Package grpcschema is what the gRPC resources share about protobuf schemas:
// naming a method the way a person writes it ("pkg.Service/Method"), and
// resolving it against a set of file descriptors with an error that lists
// what does exist.
//
// The grpc client gets its descriptors from the server it calls, over
// reflection. The grpc-server resource stands in for a server, so there is
// nothing to ask: it reads a FileDescriptorSet from disk instead, which is
// what LoadDescriptorSets is for.
package grpcschema

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/dynamicpb"

	// The well-known types, so a descriptor set that imports them without
	// carrying them still resolves.
	_ "google.golang.org/protobuf/types/known/anypb"
	_ "google.golang.org/protobuf/types/known/durationpb"
	_ "google.golang.org/protobuf/types/known/emptypb"
	_ "google.golang.org/protobuf/types/known/fieldmaskpb"
	_ "google.golang.org/protobuf/types/known/structpb"
	_ "google.golang.org/protobuf/types/known/timestamppb"
	_ "google.golang.org/protobuf/types/known/wrapperspb"
)

// SplitMethod splits "package.Service/Method" into its two halves. The
// leading slash grpc uses internally is accepted too, since that is what
// people copy out of logs.
func SplitMethod(target string) (service, method string, err error) {
	t := strings.TrimPrefix(strings.TrimSpace(target), "/")
	idx := strings.LastIndex(t, "/")
	if idx <= 0 || idx == len(t)-1 {
		return "", "", fmt.Errorf("method %q must be in the form \"package.Service/Method\"", target)
	}
	return t[:idx], t[idx+1:], nil
}

// MethodNames lists a service's methods, in declaration order.
func MethodNames(sd protoreflect.ServiceDescriptor) []string {
	out := make([]string, 0, sd.Methods().Len())
	for i := range sd.Methods().Len() {
		out = append(out, string(sd.Methods().Get(i).Name()))
	}
	return out
}

// Schema is a resolved set of file descriptors: the services they declare,
// and the message and extension types for converting JSON to protobuf.
type Schema struct {
	Files *protoregistry.Files
	Types *protoregistry.Types
}

// LoadDescriptorSets reads FileDescriptorSets, as `buf build -o x.binpb` or
// `protoc --descriptor_set_out=x.pb --include_imports` write them, and
// resolves them into one schema.
//
// An import missing from the sets is taken from the descriptors compiled into
// tomato when it has them — the well-known types, google.protobuf.Timestamp
// and friends — so a set built without --include_imports still loads when
// that is all it lacks. Anything else missing is an error naming the file.
func LoadDescriptorSets(paths ...string) (*Schema, error) {
	if len(paths) == 0 {
		return nil, fmt.Errorf("no descriptor set given")
	}
	byName := map[string]*descriptorpb.FileDescriptorProto{}
	var order []string
	for _, path := range paths {
		raw, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("reading descriptor set: %w", err)
		}
		set := &descriptorpb.FileDescriptorSet{}
		if err := proto.Unmarshal(raw, set); err != nil {
			return nil, fmt.Errorf("%s is not a FileDescriptorSet (build one with `buf build -o %s` "+
				"or `protoc --include_imports --descriptor_set_out=%s`): %w", path, path, path, err)
		}
		if len(set.GetFile()) == 0 {
			return nil, fmt.Errorf("%s holds no files; is it a FileDescriptorSet?", path)
		}
		for _, fd := range set.GetFile() {
			if _, dup := byName[fd.GetName()]; !dup {
				order = append(order, fd.GetName())
			}
			byName[fd.GetName()] = fd
		}
	}
	return newSchema(byName, order)
}

// NewSchema resolves file descriptors that are already in memory.
func NewSchema(files ...*descriptorpb.FileDescriptorProto) (*Schema, error) {
	byName := map[string]*descriptorpb.FileDescriptorProto{}
	var order []string
	for _, fd := range files {
		byName[fd.GetName()] = fd
		order = append(order, fd.GetName())
	}
	return newSchema(byName, order)
}

func newSchema(byName map[string]*descriptorpb.FileDescriptorProto, order []string) (*Schema, error) {
	// Close the set over its imports, borrowing what tomato itself has.
	for i := 0; i < len(order); i++ {
		for _, dep := range byName[order[i]].GetDependency() {
			if _, ok := byName[dep]; ok {
				continue
			}
			fd, err := protoregistry.GlobalFiles.FindFileByPath(dep)
			if err != nil {
				return nil, fmt.Errorf("%s imports %q, which the descriptor set does not contain; "+
					"build it with its imports (`buf build` includes them, protoc needs --include_imports)",
					order[i], dep)
			}
			byName[dep] = protodesc.ToFileDescriptorProto(fd)
			order = append(order, dep)
		}
	}

	set := &descriptorpb.FileDescriptorSet{}
	for _, name := range order {
		set.File = append(set.File, byName[name])
	}
	files, err := protodesc.NewFiles(set)
	if err != nil {
		return nil, fmt.Errorf("resolving the descriptor set: %w", err)
	}

	// Message types for google.protobuf.Any in JSON, extension types for the
	// reflection service to answer with.
	types := new(protoregistry.Types)
	var regErr error
	files.RangeFiles(func(fd protoreflect.FileDescriptor) bool {
		regErr = registerTypes(types, fd.Messages(), fd.Extensions())
		return regErr == nil
	})
	if regErr != nil {
		return nil, regErr
	}
	return &Schema{Files: files, Types: types}, nil
}

func registerTypes(types *protoregistry.Types, msgs protoreflect.MessageDescriptors, exts protoreflect.ExtensionDescriptors) error {
	for i := range exts.Len() {
		if err := types.RegisterExtension(dynamicpb.NewExtensionType(exts.Get(i))); err != nil {
			return err
		}
	}
	for i := range msgs.Len() {
		md := msgs.Get(i)
		if !md.IsMapEntry() {
			if err := types.RegisterMessage(dynamicpb.NewMessageType(md)); err != nil {
				return err
			}
		}
		if err := registerTypes(types, md.Messages(), md.Extensions()); err != nil {
			return err
		}
	}
	return nil
}

// Services lists every service the schema declares, sorted by full name.
func (s *Schema) Services() []protoreflect.ServiceDescriptor {
	var out []protoreflect.ServiceDescriptor
	s.Files.RangeFiles(func(fd protoreflect.FileDescriptor) bool {
		for i := range fd.Services().Len() {
			out = append(out, fd.Services().Get(i))
		}
		return true
	})
	sort.Slice(out, func(i, j int) bool { return out[i].FullName() < out[j].FullName() })
	return out
}

// ServiceNames lists the full names of every service the schema declares.
func (s *Schema) ServiceNames() []string {
	var out []string
	for _, sd := range s.Services() {
		out = append(out, string(sd.FullName()))
	}
	return out
}

// FindMethod resolves "package.Service/Method". A service or method that is
// not there fails with the ones that are, since the usual cause is a typo or
// a package left off.
func (s *Schema) FindMethod(target string) (protoreflect.MethodDescriptor, error) {
	service, method, err := SplitMethod(target)
	if err != nil {
		return nil, err
	}
	desc, err := s.Files.FindDescriptorByName(protoreflect.FullName(service))
	if err != nil {
		return nil, fmt.Errorf("service %q is not in the descriptor set; it has: %s",
			service, strings.Join(s.ServiceNames(), ", "))
	}
	sd, ok := desc.(protoreflect.ServiceDescriptor)
	if !ok {
		return nil, fmt.Errorf("%q is not a service; the descriptor set has: %s",
			service, strings.Join(s.ServiceNames(), ", "))
	}
	md := sd.Methods().ByName(protoreflect.Name(method))
	if md == nil {
		return nil, fmt.Errorf("service %q has no method %q (has: %s)",
			service, method, strings.Join(MethodNames(sd), ", "))
	}
	return md, nil
}

// MethodName is the "package.Service/Method" form of a method.
func MethodName(md protoreflect.MethodDescriptor) string {
	return string(md.Parent().FullName()) + "/" + string(md.Name())
}
