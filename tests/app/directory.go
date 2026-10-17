package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/tomatool/tomato/internal/grpcfixture"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/dynamicpb"
)

// The user directory: an HTTP API in front of a gRPC dependency,
// tomato.test.v1.UserService at USERS_ADDR, which the grpc-server resource's
// feature stubs. It forwards the caller's Authorization header as gRPC
// metadata, gives each call a one-second deadline, and maps the status it
// gets back to HTTP, the way a gateway in front of a team's service does.

// directoryTimeout is the deadline of each call to the user service.
const directoryTimeout = time.Second

var userService = func() protoreflect.ServiceDescriptor {
	fd, err := protodesc.NewFile(grpcfixture.UsersFile(), protoregistry.GlobalFiles)
	if err != nil {
		log.Fatalf("user service descriptor: %v", err)
	}
	return fd.Services().ByName("UserService")
}()

func registerDirectory(mux *http.ServeMux) {
	mux.HandleFunc("/directory/users", func(w http.ResponseWriter, r *http.Request) {
		size, _ := strconv.Atoi(r.URL.Query().Get("page_size"))
		directoryCall(w, r, "ListUsers", map[string]any{"pageSize": size})
	})
	mux.HandleFunc("/directory/users/", func(w http.ResponseWriter, r *http.Request) {
		directoryCall(w, r, "GetUser", map[string]any{"id": strings.TrimPrefix(r.URL.Path, "/directory/users/")})
	})
	mux.HandleFunc("/directory/watch", directoryWatch)
}

// directoryConn dials the user service for one request. A fresh connection
// per request keeps a refused connection from leaving the next request
// waiting out a reconnect backoff.
func directoryConn() (*grpc.ClientConn, error) {
	addr := os.Getenv("USERS_ADDR")
	if addr == "" {
		return nil, errors.New("USERS_ADDR is not set")
	}
	return grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
}

// outgoing is the request's context with its deadline and the metadata the
// directory forwards.
func outgoing(r *http.Request) (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithTimeout(r.Context(), directoryTimeout)
	var md []string
	if v := r.Header.Get("Authorization"); v != "" {
		md = append(md, "authorization", v)
	}
	if v := r.Header.Get("X-Request-Id"); v != "" {
		md = append(md, "x-request-id", v)
	}
	if len(md) > 0 {
		ctx = metadata.AppendToOutgoingContext(ctx, md...)
	}
	return ctx, cancel
}

func request(method protoreflect.MethodDescriptor, fields map[string]any) (proto.Message, error) {
	in := dynamicpb.NewMessage(method.Input())
	b, _ := json.Marshal(fields)
	return in, protojson.Unmarshal(b, in)
}

func directoryCall(w http.ResponseWriter, r *http.Request, name string, fields map[string]any) {
	method := userService.Methods().ByName(protoreflect.Name(name))
	conn, err := directoryConn()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	defer conn.Close()
	in, err := request(method, fields)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	ctx, cancel := outgoing(r)
	defer cancel()
	out := dynamicpb.NewMessage(method.Output())
	if err := conn.Invoke(ctx, "/"+grpcfixture.Service+"/"+name, in, out); err != nil {
		writeStatus(w, status.Convert(err))
		return
	}
	b, _ := protojson.Marshal(out)
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(b)
}

// directoryWatch reads the WatchUsers stream to its end and answers with
// what arrived and how the stream ended.
func directoryWatch(w http.ResponseWriter, r *http.Request) {
	method := userService.Methods().ByName("WatchUsers")
	conn, err := directoryConn()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	defer conn.Close()
	in, _ := request(method, map[string]any{"team": r.URL.Query().Get("team")})
	ctx, cancel := outgoing(r)
	defer cancel()

	stream, err := conn.NewStream(ctx, &grpc.StreamDesc{ServerStreams: true}, "/"+grpcfixture.Service+"/WatchUsers")
	if err == nil {
		err = stream.SendMsg(in)
	}
	if err == nil {
		err = stream.CloseSend()
	}
	users := []json.RawMessage{}
	for err == nil {
		out := dynamicpb.NewMessage(method.Output())
		if err = stream.RecvMsg(out); err == nil {
			b, _ := protojson.Marshal(out)
			users = append(users, b)
		}
	}
	st := status.New(codes.OK, "")
	if !errors.Is(err, io.EOF) {
		st = status.Convert(err)
	}
	writeJSON(w, http.StatusOK, map[string]any{"users": users, "status": st.Code().String(), "message": st.Message()})
}

// writeStatus answers with the HTTP status a gateway maps the gRPC one to.
func writeStatus(w http.ResponseWriter, st *status.Status) {
	code := http.StatusBadGateway
	switch st.Code() {
	case codes.NotFound:
		code = http.StatusNotFound
	case codes.PermissionDenied:
		code = http.StatusForbidden
	case codes.Unauthenticated:
		code = http.StatusUnauthorized
	case codes.DeadlineExceeded:
		code = http.StatusGatewayTimeout
	case codes.Unavailable:
		code = http.StatusServiceUnavailable
	}
	writeJSON(w, code, map[string]string{"code": st.Code().String(), "error": st.Message()})
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}
