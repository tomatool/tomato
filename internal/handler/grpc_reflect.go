package handler

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	v1 "google.golang.org/grpc/reflection/grpc_reflection_v1"
	v1alpha "google.golang.org/grpc/reflection/grpc_reflection_v1alpha"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/descriptorpb"
)

// reflectClient resolves method descriptors from a running server over the
// gRPC server-reflection API.
//
// Reflection is what lets this handler work without a copy of anyone's
// .proto files: the server already knows its own schema, so the test suite
// asks it rather than being handed a descriptor set that has to be kept in
// sync with the service it describes.
//
// Descriptors are cached for the life of the resource. A server does not
// change its schema while a test run is in flight, and re-fetching a file
// per step turns every assertion into extra round trips.
type reflectClient struct {
	conn *grpc.ClientConn

	files    map[string]*descriptorpb.FileDescriptorProto
	resolved *protoregistry.Files
}

func newReflectClient(conn *grpc.ClientConn) *reflectClient {
	return &reflectClient{conn: conn, files: map[string]*descriptorpb.FileDescriptorProto{}}
}

// reflectStream is the slice of the reflection API this needs, so the v1 and
// v1alpha wire versions can be driven by one piece of logic.
//
// Both exist because the API was stabilised after servers had already
// shipped: grpc-go registers both, but a grpc-java server may still offer
// only v1alpha. The messages are wire-identical — only the generated Go types
// differ — so the split stops here rather than spreading through the caller.
type reflectStream interface {
	fileContainingSymbol(symbol string) ([][]byte, error)
	fileByFilename(name string) ([][]byte, error)
	listServices() ([]string, error)
	Close()
}

// openStream returns a reflection stream, preferring v1 and falling back to
// v1alpha when the server does not implement it.
func (c *reflectClient) openStream(ctx context.Context) (reflectStream, error) {
	s, err := v1.NewServerReflectionClient(c.conn).ServerReflectionInfo(ctx)
	if err == nil {
		stream := &v1Stream{stream: s}
		// The stream opens lazily, so an Unimplemented server is only
		// discovered on first use. Probe with the cheapest call there is.
		if _, probeErr := stream.listServices(); probeErr == nil {
			return stream, nil
		} else if status.Code(probeErr) != codes.Unimplemented {
			return nil, probeErr
		}
		_ = s.CloseSend()
	}

	a, err := v1alpha.NewServerReflectionClient(c.conn).ServerReflectionInfo(ctx)
	if err != nil {
		return nil, fmt.Errorf("server reflection is not available (tried v1 and v1alpha): %w", err)
	}
	return &v1alphaStream{stream: a}, nil
}

// findMethod resolves "package.Service/Method" to its descriptor, fetching
// whatever files it needs and everything they import.
func (c *reflectClient) findMethod(ctx context.Context, service, method string) (protoreflect.MethodDescriptor, error) {
	if err := c.ensureSymbol(ctx, service); err != nil {
		return nil, err
	}

	desc, err := c.resolved.FindDescriptorByName(protoreflect.FullName(service))
	if err != nil {
		return nil, fmt.Errorf("service %q not found on the server: %w", service, err)
	}
	sd, ok := desc.(protoreflect.ServiceDescriptor)
	if !ok {
		return nil, fmt.Errorf("%q is not a service", service)
	}

	md := sd.Methods().ByName(protoreflect.Name(method))
	if md == nil {
		return nil, fmt.Errorf("service %q has no method %q (has: %s)",
			service, method, strings.Join(methodNames(sd), ", "))
	}
	return md, nil
}

// ensureSymbol makes sure the file defining symbol, and every file it
// transitively imports, is in the cache and the registry has been rebuilt.
func (c *reflectClient) ensureSymbol(ctx context.Context, symbol string) error {
	if c.resolved != nil {
		if _, err := c.resolved.FindDescriptorByName(protoreflect.FullName(symbol)); err == nil {
			return nil
		}
	}

	stream, err := c.openStream(ctx)
	if err != nil {
		return err
	}
	defer stream.Close()

	raw, err := stream.fileContainingSymbol(symbol)
	if err != nil {
		return fmt.Errorf("resolving %q: %w", symbol, err)
	}
	if err := c.addFiles(raw); err != nil {
		return err
	}
	// A FileContainingSymbol response is not required to carry the imports,
	// and protodesc refuses to build a set with a dangling dependency, so
	// walk what is missing until the set closes over itself.
	if err := c.fetchMissingDeps(stream); err != nil {
		return err
	}
	return c.rebuild()
}

// fetchMissingDeps repeatedly fetches any dependency named by a cached file
// but not itself cached, until the set is closed. Bounded so a server that
// keeps naming files it will not return cannot spin here forever.
func (c *reflectClient) fetchMissingDeps(stream reflectStream) error {
	for range 100 {
		missing := c.missingDeps()
		if len(missing) == 0 {
			return nil
		}
		for _, name := range missing {
			raw, err := stream.fileByFilename(name)
			if err != nil {
				return fmt.Errorf("fetching imported file %q: %w", name, err)
			}
			if err := c.addFiles(raw); err != nil {
				return err
			}
			// Guard against a server that answers without supplying the file:
			// without this the outer loop would ask for it again forever.
			if _, ok := c.files[name]; !ok {
				return fmt.Errorf("server did not return imported file %q", name)
			}
		}
	}
	return fmt.Errorf("gave up resolving imports after 100 rounds; the server's descriptors may be inconsistent")
}

func (c *reflectClient) missingDeps() []string {
	var missing []string
	seen := map[string]struct{}{}
	for _, fd := range c.files {
		for _, dep := range fd.GetDependency() {
			if _, have := c.files[dep]; have {
				continue
			}
			if _, dup := seen[dep]; dup {
				continue
			}
			seen[dep] = struct{}{}
			missing = append(missing, dep)
		}
	}
	sort.Strings(missing) // deterministic fetch order, for reproducible errors
	return missing
}

func (c *reflectClient) addFiles(raw [][]byte) error {
	for _, b := range raw {
		fd := &descriptorpb.FileDescriptorProto{}
		if err := proto.Unmarshal(b, fd); err != nil {
			return fmt.Errorf("decoding file descriptor: %w", err)
		}
		c.files[fd.GetName()] = fd
	}
	return nil
}

func (c *reflectClient) rebuild() error {
	set := &descriptorpb.FileDescriptorSet{}
	names := make([]string, 0, len(c.files))
	for name := range c.files {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		set.File = append(set.File, c.files[name])
	}

	resolved, err := protodesc.NewFiles(set)
	if err != nil {
		return fmt.Errorf("building descriptors from the server's reflection response: %w", err)
	}
	c.resolved = resolved
	return nil
}

// listServices returns the full names of every service the server exposes.
func (c *reflectClient) listServices(ctx context.Context) ([]string, error) {
	stream, err := c.openStream(ctx)
	if err != nil {
		return nil, err
	}
	defer stream.Close()

	services, err := stream.listServices()
	if err != nil {
		return nil, err
	}
	sort.Strings(services)
	return services, nil
}

func methodNames(sd protoreflect.ServiceDescriptor) []string {
	out := make([]string, 0, sd.Methods().Len())
	for i := range sd.Methods().Len() {
		out = append(out, string(sd.Methods().Get(i).Name()))
	}
	return out
}

// splitMethod splits "package.Service/Method" into its two halves. The
// leading slash grpc uses internally is accepted too, since that is what
// people copy out of logs.
func splitMethod(target string) (service, method string, err error) {
	t := strings.TrimPrefix(strings.TrimSpace(target), "/")
	idx := strings.LastIndex(t, "/")
	if idx <= 0 || idx == len(t)-1 {
		return "", "", fmt.Errorf("method %q must be in the form \"package.Service/Method\"", target)
	}
	return t[:idx], t[idx+1:], nil
}

// --- v1 ---

type v1Stream struct {
	stream grpc.BidiStreamingClient[v1.ServerReflectionRequest, v1.ServerReflectionResponse]
}

func (s *v1Stream) Close() { _ = s.stream.CloseSend() }

func (s *v1Stream) roundTrip(req *v1.ServerReflectionRequest) (*v1.ServerReflectionResponse, error) {
	if err := s.stream.Send(req); err != nil {
		// Send returns a bare io.EOF when the server has already ended the
		// stream (e.g. Unimplemented on a v1alpha-only server); the real
		// status is only available from Recv. Returning EOF made the v1 probe
		// miss the Unimplemented and skip the v1alpha fallback under load.
		if errors.Is(err, io.EOF) {
			_, recvErr := s.stream.Recv()
			if recvErr != nil {
				return nil, recvErr
			}
		}
		return nil, err
	}
	resp, err := s.stream.Recv()
	if err != nil {
		return nil, err
	}
	if e := resp.GetErrorResponse(); e != nil {
		return nil, status.Error(codes.Code(e.GetErrorCode()), e.GetErrorMessage())
	}
	return resp, nil
}

func (s *v1Stream) fileContainingSymbol(symbol string) ([][]byte, error) {
	resp, err := s.roundTrip(&v1.ServerReflectionRequest{
		MessageRequest: &v1.ServerReflectionRequest_FileContainingSymbol{FileContainingSymbol: symbol},
	})
	if err != nil {
		return nil, err
	}
	return resp.GetFileDescriptorResponse().GetFileDescriptorProto(), nil
}

func (s *v1Stream) fileByFilename(name string) ([][]byte, error) {
	resp, err := s.roundTrip(&v1.ServerReflectionRequest{
		MessageRequest: &v1.ServerReflectionRequest_FileByFilename{FileByFilename: name},
	})
	if err != nil {
		return nil, err
	}
	return resp.GetFileDescriptorResponse().GetFileDescriptorProto(), nil
}

func (s *v1Stream) listServices() ([]string, error) {
	resp, err := s.roundTrip(&v1.ServerReflectionRequest{
		MessageRequest: &v1.ServerReflectionRequest_ListServices{},
	})
	if err != nil {
		return nil, err
	}
	var out []string
	for _, svc := range resp.GetListServicesResponse().GetService() {
		out = append(out, svc.GetName())
	}
	return out, nil
}

// --- v1alpha ---

type v1alphaStream struct {
	stream grpc.BidiStreamingClient[v1alpha.ServerReflectionRequest, v1alpha.ServerReflectionResponse]
}

func (s *v1alphaStream) Close() { _ = s.stream.CloseSend() }

func (s *v1alphaStream) roundTrip(req *v1alpha.ServerReflectionRequest) (*v1alpha.ServerReflectionResponse, error) {
	if err := s.stream.Send(req); err != nil {
		// Send returns a bare io.EOF when the server has already ended the
		// stream (e.g. Unimplemented on a v1alpha-only server); the real
		// status is only available from Recv. Returning EOF made the v1 probe
		// miss the Unimplemented and skip the v1alpha fallback under load.
		if errors.Is(err, io.EOF) {
			_, recvErr := s.stream.Recv()
			if recvErr != nil {
				return nil, recvErr
			}
		}
		return nil, err
	}
	resp, err := s.stream.Recv()
	if err != nil {
		return nil, err
	}
	if e := resp.GetErrorResponse(); e != nil {
		return nil, status.Error(codes.Code(e.GetErrorCode()), e.GetErrorMessage())
	}
	return resp, nil
}

func (s *v1alphaStream) fileContainingSymbol(symbol string) ([][]byte, error) {
	resp, err := s.roundTrip(&v1alpha.ServerReflectionRequest{
		MessageRequest: &v1alpha.ServerReflectionRequest_FileContainingSymbol{FileContainingSymbol: symbol},
	})
	if err != nil {
		return nil, err
	}
	return resp.GetFileDescriptorResponse().GetFileDescriptorProto(), nil
}

func (s *v1alphaStream) fileByFilename(name string) ([][]byte, error) {
	resp, err := s.roundTrip(&v1alpha.ServerReflectionRequest{
		MessageRequest: &v1alpha.ServerReflectionRequest_FileByFilename{FileByFilename: name},
	})
	if err != nil {
		return nil, err
	}
	return resp.GetFileDescriptorResponse().GetFileDescriptorProto(), nil
}

func (s *v1alphaStream) listServices() ([]string, error) {
	resp, err := s.roundTrip(&v1alpha.ServerReflectionRequest{
		MessageRequest: &v1alpha.ServerReflectionRequest_ListServices{},
	})
	if err != nil {
		return nil, err
	}
	var out []string
	for _, svc := range resp.GetListServicesResponse().GetService() {
		out = append(out, svc.GetName())
	}
	return out, nil
}
