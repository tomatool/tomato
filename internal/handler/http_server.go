package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/cucumber/godog"
	"github.com/tomatool/tomato/internal/config"
	"github.com/tomatool/tomato/internal/container"
)

// HTTPServer provides a mock HTTP server for testing
type HTTPServer struct {
	name      string
	config    config.Resource
	container *container.Manager

	server   *http.Server
	listener net.Listener
	port     int

	stubs    []*HTTPStub
	calls    []*RecordedCall
	stubsMu  sync.RWMutex
	callsMu  sync.RWMutex
}

// HTTPStub represents a stub configuration
type HTTPStub struct {
	Method      string
	Path        string
	PathPattern *regexp.Regexp
	Status      int
	Headers     map[string]string
	Body        string
}

// RecordedCall represents a recorded HTTP request
type RecordedCall struct {
	Method  string
	Path    string
	Headers http.Header
	Body    string
	Time    time.Time
}

func NewHTTPServer(name string, cfg config.Resource, cm *container.Manager) (*HTTPServer, error) {
	return &HTTPServer{
		name:      name,
		config:    cfg,
		container: cm,
		stubs:     make([]*HTTPStub, 0),
		calls:     make([]*RecordedCall, 0),
	}, nil
}

func (r *HTTPServer) Name() string { return r.name }

func (r *HTTPServer) Init(ctx context.Context) error {
	port := 0 // Let system assign a free port
	if p, ok := r.config.Options["port"].(int); ok {
		port = p
	}

	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return fmt.Errorf("creating listener: %w", err)
	}
	r.listener = listener
	r.port = listener.Addr().(*net.TCPAddr).Port

	mux := http.NewServeMux()
	mux.HandleFunc("/", r.handleRequest)

	r.server = &http.Server{Handler: mux}

	go func() {
		if err := r.server.Serve(listener); err != nil && err != http.ErrServerClosed {
			// Log error but don't fail - server might be shutting down
		}
	}()

	return nil
}

func (r *HTTPServer) handleRequest(w http.ResponseWriter, req *http.Request) {
	// Record the call
	body := ""
	if req.Body != nil {
		buf := make([]byte, 1024*1024) // 1MB max
		n, _ := req.Body.Read(buf)
		body = string(buf[:n])
	}

	r.callsMu.Lock()
	r.calls = append(r.calls, &RecordedCall{
		Method:  req.Method,
		Path:    req.URL.Path,
		Headers: req.Header.Clone(),
		Body:    body,
		Time:    time.Now(),
	})
	r.callsMu.Unlock()

	// Find matching stub
	r.stubsMu.RLock()
	var matchedStub *HTTPStub
	for _, stub := range r.stubs {
		if stub.Method != req.Method {
			continue
		}
		if stub.PathPattern != nil {
			if stub.PathPattern.MatchString(req.URL.Path) {
				matchedStub = stub
				break
			}
		} else if stub.Path == req.URL.Path {
			matchedStub = stub
			break
		}
	}
	r.stubsMu.RUnlock()

	if matchedStub == nil {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(fmt.Sprintf("No stub found for %s %s", req.Method, req.URL.Path)))
		return
	}

	for k, v := range matchedStub.Headers {
		w.Header().Set(k, v)
	}
	w.WriteHeader(matchedStub.Status)
	w.Write([]byte(matchedStub.Body))
}

func (r *HTTPServer) Ready(ctx context.Context) error {
	return nil
}

func (r *HTTPServer) Reset(ctx context.Context) error {
	r.stubsMu.Lock()
	r.stubs = make([]*HTTPStub, 0)
	r.stubsMu.Unlock()

	r.callsMu.Lock()
	r.calls = make([]*RecordedCall, 0)
	r.callsMu.Unlock()

	return nil
}

func (r *HTTPServer) RegisterSteps(ctx *godog.ScenarioContext) {
	RegisterStepsToGodog(ctx, r.name, r.Steps())
}

// Steps returns the structured step definitions for the HTTP server handler
func (r *HTTPServer) Steps() StepCategory {
	return StepCategory{
		Name:        "HTTP Server",
		Description: "Steps for stubbing HTTP services",
		Steps: []StepDef{
			// Stub setup
			{
				Pattern:     `^"{resource}" stub "([^"]*)" "([^"]*)" returns "(\d+)"$`,
				Description: "Creates a stub that returns a status code",
				Example:     `"{resource}" stub "GET" "/users" returns "200"`,
				Handler:     r.stubReturnsStatus,
			},
			{
				Pattern:     `^"{resource}" stub "([^"]*)" "([^"]*)" returns "(\d+)" with body:$`,
				Description: "Creates a stub that returns a status code and body",
				Example:     "\"{resource}\" stub \"GET\" \"/users\" returns \"200\" with body:\n  \"\"\"\n  [{\"id\": 1}]\n  \"\"\"",
				Handler:     r.stubReturnsBody,
			},
			{
				Pattern:     `^"{resource}" stub "([^"]*)" "([^"]*)" returns "(\d+)" with json:$`,
				Description: "Creates a stub that returns JSON (auto sets Content-Type)",
				Example:     "\"{resource}\" stub \"GET\" \"/users\" returns \"200\" with json:\n  \"\"\"\n  [{\"id\": 1}]\n  \"\"\"",
				Handler:     r.stubReturnsJSON,
			},
			{
				Pattern:     `^"{resource}" stub "([^"]*)" "([^"]*)" returns "(\d+)" with headers:$`,
				Description: "Creates a stub that returns with custom headers",
				Example:     "\"{resource}\" stub \"GET\" \"/users\" returns \"200\" with headers:\n  | header       | value            |\n  | X-Custom     | value            |",
				Handler:     r.stubReturnsHeaders,
			},

			// Verification
			{
				Pattern:     `^"{resource}" received "([^"]*)" "([^"]*)"$`,
				Description: "Asserts a request was received",
				Example:     `"{resource}" received "GET" "/users"`,
				Handler:     r.receivedRequest,
			},
			{
				Pattern:     `^"{resource}" received "([^"]*)" "([^"]*)" "(\d+)" times$`,
				Description: "Asserts a request was received N times",
				Example:     `"{resource}" received "GET" "/users" "2" times`,
				Handler:     r.receivedRequestTimes,
			},
			{
				Pattern:     `^"{resource}" did not receive "([^"]*)" "([^"]*)"$`,
				Description: "Asserts a request was not received",
				Example:     `"{resource}" did not receive "DELETE" "/users"`,
				Handler:     r.didNotReceiveRequest,
			},
			{
				Pattern:     `^"{resource}" received request with header "([^"]*)" containing "([^"]*)"$`,
				Description: "Asserts any request was received with header containing value",
				Example:     `"{resource}" received request with header "Authorization" containing "Bearer"`,
				Handler:     r.receivedRequestWithHeader,
			},
			{
				Pattern:     `^"{resource}" received request with body containing "([^"]*)"$`,
				Description: "Asserts any request was received with body containing value",
				Example:     `"{resource}" received request with body containing "name"`,
				Handler:     r.receivedRequestWithBody,
			},
			{
				Pattern:     `^"{resource}" received "(\d+)" requests$`,
				Description: "Asserts total number of requests received",
				Example:     `"{resource}" received "5" requests`,
				Handler:     r.receivedTotalRequests,
			},

			// Server info
			{
				Pattern:     `^"{resource}" url is stored in "([^"]*)"$`,
				Description: "Stores the server URL in a variable for use in other steps",
				Example:     `"{resource}" url is stored in "SERVER_URL"`,
				Handler:     r.storeURL,
			},
		},
	}
}

func (r *HTTPServer) stubReturnsStatus(method, path string, status int) error {
	r.stubsMu.Lock()
	defer r.stubsMu.Unlock()

	r.stubs = append(r.stubs, &HTTPStub{
		Method: method,
		Path:   path,
		Status: status,
	})
	return nil
}

func (r *HTTPServer) stubReturnsBody(method, path string, status int, doc *godog.DocString) error {
	r.stubsMu.Lock()
	defer r.stubsMu.Unlock()

	r.stubs = append(r.stubs, &HTTPStub{
		Method: method,
		Path:   path,
		Status: status,
		Body:   doc.Content,
	})
	return nil
}

func (r *HTTPServer) stubReturnsJSON(method, path string, status int, doc *godog.DocString) error {
	var js json.RawMessage
	if err := json.Unmarshal([]byte(doc.Content), &js); err != nil {
		return fmt.Errorf("invalid JSON: %w", err)
	}

	r.stubsMu.Lock()
	defer r.stubsMu.Unlock()

	r.stubs = append(r.stubs, &HTTPStub{
		Method:  method,
		Path:    path,
		Status:  status,
		Headers: map[string]string{"Content-Type": "application/json"},
		Body:    doc.Content,
	})
	return nil
}

func (r *HTTPServer) stubReturnsHeaders(method, path string, status int, table *godog.Table) error {
	headers := make(map[string]string)
	for _, row := range table.Rows[1:] {
		if len(row.Cells) >= 2 {
			headers[row.Cells[0].Value] = row.Cells[1].Value
		}
	}

	r.stubsMu.Lock()
	defer r.stubsMu.Unlock()

	r.stubs = append(r.stubs, &HTTPStub{
		Method:  method,
		Path:    path,
		Status:  status,
		Headers: headers,
	})
	return nil
}

func (r *HTTPServer) receivedRequest(method, path string) error {
	r.callsMu.RLock()
	defer r.callsMu.RUnlock()

	for _, call := range r.calls {
		if call.Method == method && call.Path == path {
			return nil
		}
	}
	return fmt.Errorf("no %s %s request received", method, path)
}

func (r *HTTPServer) receivedRequestTimes(method, path string, times int) error {
	r.callsMu.RLock()
	defer r.callsMu.RUnlock()

	count := 0
	for _, call := range r.calls {
		if call.Method == method && call.Path == path {
			count++
		}
	}
	if count != times {
		return fmt.Errorf("expected %d %s %s requests, got %d", times, method, path, count)
	}
	return nil
}

func (r *HTTPServer) didNotReceiveRequest(method, path string) error {
	r.callsMu.RLock()
	defer r.callsMu.RUnlock()

	for _, call := range r.calls {
		if call.Method == method && call.Path == path {
			return fmt.Errorf("unexpected %s %s request received", method, path)
		}
	}
	return nil
}

func (r *HTTPServer) receivedRequestWithHeader(header, value string) error {
	r.callsMu.RLock()
	defer r.callsMu.RUnlock()

	for _, call := range r.calls {
		if strings.Contains(call.Headers.Get(header), value) {
			return nil
		}
	}
	return fmt.Errorf("no request received with header %q containing %q", header, value)
}

func (r *HTTPServer) receivedRequestWithBody(value string) error {
	r.callsMu.RLock()
	defer r.callsMu.RUnlock()

	for _, call := range r.calls {
		if strings.Contains(call.Body, value) {
			return nil
		}
	}
	return fmt.Errorf("no request received with body containing %q", value)
}

func (r *HTTPServer) receivedTotalRequests(count int) error {
	r.callsMu.RLock()
	defer r.callsMu.RUnlock()

	if len(r.calls) != count {
		return fmt.Errorf("expected %d requests, got %d", count, len(r.calls))
	}
	return nil
}

func (r *HTTPServer) storeURL(varName string) error {
	// This would need integration with a variable store
	// For now, we'll just return nil - in a real implementation,
	// this would store http://localhost:{port} in a shared context
	return nil
}

// GetURL returns the server URL for use by other handlers
func (r *HTTPServer) GetURL() string {
	return fmt.Sprintf("http://localhost:%d", r.port)
}

func (r *HTTPServer) Cleanup(ctx context.Context) error {
	if r.server != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return r.server.Shutdown(ctx)
	}
	return nil
}

var _ Handler = (*HTTPServer)(nil)
