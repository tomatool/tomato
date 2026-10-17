// Package httpserver provides tomato's `http-server` resource: stubbing the HTTP services the
// application under test calls.
package httpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/cucumber/godog"
	"github.com/tomatool/tomato/internal/config"
	"github.com/tomatool/tomato/internal/container"
	"github.com/tomatool/tomato/internal/jsonmatch"
	"github.com/tomatool/tomato/internal/resource"
)

// HTTPServer provides a mock HTTP server for testing
type HTTPServer struct {
	name      string
	config    config.Resource
	container *container.Manager

	server   *http.Server
	listener net.Listener
	port     int
	// refusing is true while "refuses connections" holds the port closed.
	refusing  bool
	listenMu  sync.Mutex
	serveDone chan struct{}

	stubs   []*HTTPStub
	calls   []*RecordedCall
	stubsMu sync.RWMutex
	callsMu sync.RWMutex
}

// HTTPStub is one stubbed endpoint: what a request must match, and the
// responses it gets, in order. The last response repeats once the others are
// used up, so a single response answers every request.
type HTTPStub struct {
	Method      string
	Path        string
	PathPattern *regexp.Regexp
	Conditions  []condition
	Responses   []stubResponse
	served      int
}

// stubResponse is one answer a stub gives.
type stubResponse struct {
	Status  int
	Headers map[string]string
	Body    string
	Delay   time.Duration
}

// condition is one thing besides method and path a request must match.
type condition struct {
	kind  string // header, query or body
	key   string
	value string
}

func (c condition) matches(req *http.Request, body string) bool {
	switch c.kind {
	case "header":
		return req.Header.Get(c.key) == c.value
	case "query":
		return req.URL.Query().Get(c.key) == c.value
	case "body":
		return strings.Contains(body, c.value)
	}
	return false
}

func (c condition) String() string {
	if c.kind == "body" {
		return fmt.Sprintf("body containing %q", c.value)
	}
	return fmt.Sprintf("%s %q %q", c.kind, c.key, c.value)
}

// RecordedCall represents a recorded HTTP request
type RecordedCall struct {
	Method  string
	Path    string
	Query   string
	Headers http.Header
	Body    string
	Time    time.Time
}

func New(name string, cfg config.Resource, cm *container.Manager) (*HTTPServer, error) {
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

	mux := http.NewServeMux()
	mux.HandleFunc("/", r.handleRequest)
	r.server = &http.Server{Handler: mux}

	return r.listen(port)
}

// listen opens the port and serves on it. After the first time the port is
// fixed, so the app keeps the URL it was given.
func (r *HTTPServer) listen(port int) error {
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return fmt.Errorf("creating listener: %w", err)
	}
	r.listener = listener
	r.port = listener.Addr().(*net.TCPAddr).Port

	done := make(chan struct{})
	r.serveDone = done
	go func() {
		defer close(done)
		// Serve returns when the listener closes, on shutdown or while
		// refusing connections; neither is an error.
		_ = r.server.Serve(listener)
	}()
	return nil
}

func (r *HTTPServer) handleRequest(w http.ResponseWriter, req *http.Request) {
	// Record the call. One Read returns whatever has arrived so far, which
	// can be part of the body; read it to the end (up to 1MB).
	body := ""
	if req.Body != nil {
		b, _ := io.ReadAll(io.LimitReader(req.Body, 1024*1024))
		body = string(b)
	}

	r.callsMu.Lock()
	r.calls = append(r.calls, &RecordedCall{
		Method:  req.Method,
		Path:    req.URL.Path,
		Query:   req.URL.RawQuery,
		Headers: req.Header.Clone(),
		Body:    body,
		Time:    time.Now(),
	})
	r.callsMu.Unlock()

	resp, ok := r.respond(req, body)
	if !ok {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(fmt.Sprintf("No stub found for %s %s", req.Method, req.URL.Path)))
		return
	}

	if resp.Delay > 0 {
		select {
		case <-time.After(resp.Delay):
		case <-req.Context().Done():
			return // the client gave up, as a timing-out app does
		}
	}
	for k, v := range resp.Headers {
		w.Header().Set(k, v)
	}
	w.WriteHeader(resp.Status)
	w.Write([]byte(resp.Body))
}

// respond picks the stub for a request and takes its next response. The most
// specific stub wins (more conditions beat fewer), then the most recently
// added, so a scenario can override a stub its Background set.
func (r *HTTPServer) respond(req *http.Request, body string) (stubResponse, bool) {
	r.stubsMu.Lock()
	defer r.stubsMu.Unlock()

	var best *HTTPStub
	for _, stub := range r.stubs {
		if !stub.matches(req, body) {
			continue
		}
		if best == nil || len(stub.Conditions) >= len(best.Conditions) {
			best = stub
		}
	}
	if best == nil {
		return stubResponse{}, false
	}
	i := best.served
	if i >= len(best.Responses) {
		i = len(best.Responses) - 1
	}
	best.served++
	return best.Responses[i], true
}

func (s *HTTPStub) matches(req *http.Request, body string) bool {
	if s.Method != req.Method {
		return false
	}
	if s.PathPattern != nil {
		if !s.PathPattern.MatchString(req.URL.Path) {
			return false
		}
	} else if s.Path != req.URL.Path {
		return false
	}
	for _, c := range s.Conditions {
		if !c.matches(req, body) {
			return false
		}
	}
	return true
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

	// A scenario that refused connections must not leave the next one
	// without a server.
	return r.acceptConnections()
}

func (r *HTTPServer) RegisterSteps(ctx *godog.ScenarioContext) {
	resource.RegisterStepsToGodog(ctx, r.name, r.Steps())
}

// conditions is the optional part of a stub step between the path and
// "returns": any number of `with header "k" "v"`, `with query "k" "v"` and
// `with body containing "x"`, in any order.
const conditions = `((?: with (?:header|query) "[^"]*" "[^"]*"| with body containing "[^"]*")*)`

var conditionRe = regexp.MustCompile(` with (header|query) "([^"]*)" "([^"]*)"| with body containing "([^"]*)"`)

func parseConditions(s string) []condition {
	var out []condition
	for _, m := range conditionRe.FindAllStringSubmatch(s, -1) {
		if m[1] != "" {
			out = append(out, condition{kind: m[1], key: m[2], value: resource.ReplaceVariables(m[3])})
		} else {
			out = append(out, condition{kind: "body", value: resource.ReplaceVariables(m[4])})
		}
	}
	return out
}

// Steps returns the structured step definitions for the HTTP server handler
func (r *HTTPServer) Steps() resource.StepCategory {
	return resource.StepCategory{
		Name:        "HTTP Server",
		Description: "Steps for stubbing HTTP services",
		Steps: []resource.StepDef{
			// Stub Setup
			{
				Group:       "Stub Setup",
				Pattern:     `^"{resource}" stub "([^"]*)" "([^"]*)"` + conditions + ` returns "(\d+)"$`,
				Description: "Creates a stub that returns a status code. Between the path and \"returns\", `with header \"k\" \"v\"`, `with query \"k\" \"v\"` and `with body containing \"x\"` narrow which requests it answers; the most specific stub wins, then the most recent",
				Example:     `"{resource}" stub "GET" "/users" returns "200"`,
				Handler:     r.stubReturnsStatus,
			},
			{
				Group:       "Stub Setup",
				Pattern:     `^"{resource}" stub "([^"]*)" "([^"]*)"` + conditions + ` returns "(\d+)" with body:$`,
				Description: "Creates a stub that returns a status code and body",
				Example:     "\"{resource}\" stub \"GET\" \"/users\" returns \"200\" with body:\n  \"\"\"\n  [{\"id\": 1}]\n  \"\"\"",
				Handler:     r.stubReturnsBody,
			},
			{
				Group:       "Stub Setup",
				Pattern:     `^"{resource}" stub "([^"]*)" "([^"]*)"` + conditions + ` returns "(\d+)" with json:$`,
				Description: "Creates a stub that returns JSON (auto sets Content-Type)",
				Example:     "\"{resource}\" stub \"POST\" \"/\" with header \"X-Amz-Target\" \"TrentService.Encrypt\" returns \"200\" with json:\n  \"\"\"\n  {\"KeyId\": \"test\"}\n  \"\"\"",
				Handler:     r.stubReturnsJSON,
			},
			{
				Group:       "Stub Setup",
				Pattern:     `^"{resource}" stub "([^"]*)" "([^"]*)"` + conditions + ` returns "(\d+)" with headers:$`,
				Description: "Creates a stub that returns with custom headers",
				Example:     "\"{resource}\" stub \"GET\" \"/users\" returns \"200\" with headers:\n  | header       | value            |\n  | X-Custom     | value            |",
				Handler:     r.stubReturnsHeaders,
			},
			{
				Group:       "Stub Setup",
				Pattern:     `^"{resource}" stub "([^"]*)" "([^"]*)"` + conditions + ` returns in sequence:$`,
				Description: "Creates a stub that answers each request with the next row, and repeats the last row once the rows are used up. Columns: status (required), json or body, and delay (e.g. 3s) to answer late, as a slow or timing-out dependency does",
				Example:     "\"{resource}\" stub \"POST\" \"/token\" returns in sequence:\n  | status | json                         | delay |\n  | 200    | {\"pending\": true}            | 5s    |\n  | 200    | {\"token\": \"t\"}               |       |",
				Handler:     r.stubReturnsSequence,
			},

			// Availability
			{
				Group:       "Availability",
				Pattern:     `^"{resource}" refuses connections$`,
				Description: "Close the server's port, so the app's connections are refused as when the service is down. Requests already in flight finish",
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
				Pattern:     `^"{resource}" received "([^"]*)" "([^"]*)"$`,
				Description: "Asserts a request was received",
				Example:     `"{resource}" received "GET" "/users"`,
				Handler:     r.receivedRequest,
			},
			{
				Group:       "Verification",
				Pattern:     `^"{resource}" received "([^"]*)" "([^"]*)" "(\d+)" times$`,
				Description: "Asserts a request was received N times",
				Example:     `"{resource}" received "GET" "/users" "2" times`,
				Handler:     r.receivedRequestTimes,
			},
			{
				Group:       "Verification",
				Pattern:     `^"{resource}" received "([^"]*)" "([^"]*)" with header "([^"]*)" "([^"]*)"$`,
				Description: "Asserts a request was received with this exact header value, e.g. which operation of a one-path API was called",
				Example:     `"{resource}" received "POST" "/" with header "X-Amz-Target" "TrentService.Encrypt"`,
				Handler:     r.receivedRequestWithExactHeader,
			},
			{
				Group:       "Verification",
				Pattern:     `^"{resource}" did not receive "([^"]*)" "([^"]*)"$`,
				Description: "Asserts a request was not received",
				Example:     `"{resource}" did not receive "DELETE" "/users"`,
				Handler:     r.didNotReceiveRequest,
			},
			{
				Group:       "Verification",
				Pattern:     `^"{resource}" received request with header "([^"]*)" containing "([^"]*)"$`,
				Description: "Asserts any request was received with header containing value",
				Example:     `"{resource}" received request with header "Authorization" containing "Bearer"`,
				Handler:     r.receivedRequestWithHeader,
			},
			{
				Group:       "Verification",
				Pattern:     `^"{resource}" received request with body containing "([^"]*)"$`,
				Description: "Asserts any request was received with body containing value",
				Example:     `"{resource}" received request with body containing "name"`,
				Handler:     r.receivedRequestWithBody,
			},
			{
				Group:       "Verification",
				Pattern:     `^"{resource}" received "(\d+)" requests$`,
				Description: "Asserts total number of requests received",
				Example:     `"{resource}" received "5" requests`,
				Handler:     r.receivedTotalRequests,
			},

			// Received request
			{
				Group:       "Received Request",
				Pattern:     `^"{resource}" last "([^"]*)" "([^"]*)" request json "([^"]*)" saved as "\{\{([^}]+)\}\}"$`,
				Description: "Save a field of the body of the last matching request the stub received, e.g. an id the app generated and later refers to",
				Example:     `"{resource}" last "POST" "/orders" request json "clientProcessId" saved as "{{process_id}}"`,
				Handler:     r.saveLastRequestJSON,
			},
			{
				Group:       "Received Request",
				Pattern:     `^"{resource}" last "([^"]*)" "([^"]*)" request header "([^"]*)" saved as "\{\{([^}]+)\}\}"$`,
				Description: "Save a header of the last matching request the stub received",
				Example:     `"{resource}" last "POST" "/orders" request header "Idempotency-Key" saved as "{{key}}"`,
				Handler:     r.saveLastRequestHeader,
			},
			{
				Group:       "Received Request",
				Pattern:     `^"{resource}" last "([^"]*)" "([^"]*)" request json contains:$`,
				Description: "Assert the body of the last matching request contains these JSON fields (extra fields ignored, same matchers as the http resource's \"response json contains\")",
				Example:     "\"{resource}\" last \"POST\" \"/orders\" request json contains:\n  \"\"\"\n  {\"orderSize\": \"10\"}\n  \"\"\"",
				Handler:     r.lastRequestJSONContains,
			},

			// Server Info
			{
				Group:       "Server Info",
				Pattern:     `^"{resource}" url is stored in "([^"]*)"$`,
				Description: "Stores the server URL in a variable for use in other steps",
				Example:     `"{resource}" url is stored in "SERVER_URL"`,
				Handler:     r.storeURL,
			},
		},
	}
}

func (r *HTTPServer) addStub(method, path, conds string, responses ...stubResponse) {
	r.stubsMu.Lock()
	defer r.stubsMu.Unlock()
	r.stubs = append(r.stubs, &HTTPStub{
		Method:     method,
		Path:       resource.ReplaceVariables(path),
		Conditions: parseConditions(conds),
		Responses:  responses,
	})
}

func (r *HTTPServer) stubReturnsStatus(method, path, conds string, status int) error {
	r.addStub(method, path, conds, stubResponse{Status: status})
	return nil
}

func (r *HTTPServer) stubReturnsBody(method, path, conds string, status int, doc *godog.DocString) error {
	r.addStub(method, path, conds, stubResponse{Status: status, Body: doc.Content})
	return nil
}

func (r *HTTPServer) stubReturnsJSON(method, path, conds string, status int, doc *godog.DocString) error {
	var js json.RawMessage
	if err := json.Unmarshal([]byte(doc.Content), &js); err != nil {
		return fmt.Errorf("invalid JSON: %w", err)
	}
	r.addStub(method, path, conds, stubResponse{
		Status:  status,
		Headers: map[string]string{"Content-Type": "application/json"},
		Body:    doc.Content,
	})
	return nil
}

func (r *HTTPServer) stubReturnsHeaders(method, path, conds string, status int, table *godog.Table) error {
	headers := make(map[string]string)
	for _, row := range table.Rows[1:] {
		if len(row.Cells) >= 2 {
			headers[row.Cells[0].Value] = row.Cells[1].Value
		}
	}
	r.addStub(method, path, conds, stubResponse{Status: status, Headers: headers})
	return nil
}

func (r *HTTPServer) stubReturnsSequence(method, path, conds string, table *godog.Table) error {
	if len(table.Rows) < 2 {
		return fmt.Errorf("the table needs a header row and at least one response")
	}
	cols := map[string]int{}
	for i, c := range table.Rows[0].Cells {
		cols[strings.TrimSpace(c.Value)] = i
	}
	if _, ok := cols["status"]; !ok {
		return fmt.Errorf("the table needs a status column")
	}
	_, hasJSON := cols["json"]
	_, hasBody := cols["body"]
	if hasJSON && hasBody {
		return fmt.Errorf("use a json or a body column, not both")
	}
	for name := range cols {
		if name != "status" && name != "json" && name != "body" && name != "delay" {
			return fmt.Errorf("unknown column %q: use status, json or body, and delay", name)
		}
	}

	var responses []stubResponse
	for i, row := range table.Rows[1:] {
		cell := func(name string) string {
			if idx, ok := cols[name]; ok {
				return strings.TrimSpace(row.Cells[idx].Value)
			}
			return ""
		}
		status, err := strconv.Atoi(cell("status"))
		if err != nil {
			return fmt.Errorf("row %d: status %q is not a number", i+1, cell("status"))
		}
		resp := stubResponse{Status: status, Body: cell("body")}
		if j := cell("json"); j != "" {
			var js json.RawMessage
			if err := json.Unmarshal([]byte(j), &js); err != nil {
				return fmt.Errorf("row %d: invalid JSON: %w", i+1, err)
			}
			resp.Body = j
			resp.Headers = map[string]string{"Content-Type": "application/json"}
		}
		if d := cell("delay"); d != "" {
			if resp.Delay, err = time.ParseDuration(d); err != nil {
				return fmt.Errorf("row %d: invalid delay: %w", i+1, err)
			}
		}
		responses = append(responses, resp)
	}
	r.addStub(method, path, conds, responses...)
	return nil
}

func (r *HTTPServer) refuseConnections() error {
	r.listenMu.Lock()
	defer r.listenMu.Unlock()
	if r.refusing {
		return nil
	}
	if err := r.listener.Close(); err != nil {
		return fmt.Errorf("closing the port: %w", err)
	}
	<-r.serveDone
	// Closing the port leaves idle keep-alive connections open, and an app
	// would reuse one and still get answers. This closes them too.
	r.server.SetKeepAlivesEnabled(false)
	r.refusing = true
	return nil
}

func (r *HTTPServer) acceptConnections() error {
	r.listenMu.Lock()
	defer r.listenMu.Unlock()
	if !r.refusing {
		return nil
	}
	if err := r.listen(r.port); err != nil {
		return fmt.Errorf("reopening port %d: %w", r.port, err)
	}
	r.server.SetKeepAlivesEnabled(true)
	r.refusing = false
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

func (r *HTTPServer) receivedRequestWithExactHeader(method, path, header, value string) error {
	value = resource.ReplaceVariables(value)
	r.callsMu.RLock()
	defer r.callsMu.RUnlock()

	var seen []string
	for _, call := range r.calls {
		if call.Method == method && call.Path == path {
			got := call.Headers.Get(header)
			if got == value {
				return nil
			}
			seen = append(seen, strconv.Quote(got))
		}
	}
	if len(seen) == 0 {
		return fmt.Errorf("no %s %s request received", method, path)
	}
	return fmt.Errorf("no %s %s request with %s %q; got %s", method, path, header, value, strings.Join(seen, ", "))
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

// lastCall returns the last request received for method and path. Path
// accepts variables, since the app often builds it from an id the scenario saved.
func (r *HTTPServer) lastCall(method, path string) (*RecordedCall, error) {
	path = resource.ReplaceVariables(path)
	r.callsMu.RLock()
	defer r.callsMu.RUnlock()
	for i := len(r.calls) - 1; i >= 0; i-- {
		if c := r.calls[i]; c.Method == method && c.Path == path {
			return c, nil
		}
	}
	return nil, fmt.Errorf("no %s %s request received (%d requests in total)", method, path, len(r.calls))
}

func (r *HTTPServer) saveLastRequestJSON(method, path, field, varName string) error {
	call, err := r.lastCall(method, path)
	if err != nil {
		return err
	}
	value, err := jsonmatch.Path([]byte(call.Body), field)
	if err != nil {
		return fmt.Errorf("%s %s request body: %w\nbody: %s", method, path, err, call.Body)
	}
	switch v := value.(type) {
	case string:
		resource.SetVariable(varName, v)
	case nil:
		return fmt.Errorf("%s %s request body: %q is null", method, path, field)
	case float64:
		// Without 'f', a large integer would be saved as 1.7919e+12.
		resource.SetVariable(varName, strconv.FormatFloat(v, 'f', -1, 64))
	case map[string]interface{}, []interface{}:
		b, _ := json.Marshal(v)
		resource.SetVariable(varName, string(b))
	default:
		resource.SetVariable(varName, fmt.Sprintf("%v", v))
	}
	return nil
}

func (r *HTTPServer) saveLastRequestHeader(method, path, header, varName string) error {
	call, err := r.lastCall(method, path)
	if err != nil {
		return err
	}
	value := call.Headers.Get(header)
	if value == "" {
		return fmt.Errorf("%s %s request has no %s header", method, path, header)
	}
	resource.SetVariable(varName, value)
	return nil
}

func (r *HTTPServer) lastRequestJSONContains(method, path string, doc *godog.DocString) error {
	call, err := r.lastCall(method, path)
	if err != nil {
		return err
	}
	expectedText := resource.ReplaceVariables(doc.Content)
	var expected, actual interface{}
	if err := json.Unmarshal([]byte(expectedText), &expected); err != nil {
		return fmt.Errorf("invalid expected JSON: %w", err)
	}
	if err := json.Unmarshal([]byte(call.Body), &actual); err != nil {
		return fmt.Errorf("%s %s request body is not JSON: %w\nbody: %s", method, path, err, call.Body)
	}
	if err := jsonmatch.CompareDocs("request body", expected, actual, strings.TrimSpace(expectedText), strings.TrimSpace(call.Body), true); err != nil {
		return fmt.Errorf("%s %s request body: %w\nbody: %s", method, path, err, call.Body)
	}
	return nil
}

func (r *HTTPServer) storeURL(varName string) error {
	resource.SetVariable(varName, r.GetURL())
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

var _ resource.Handler = (*HTTPServer)(nil)
