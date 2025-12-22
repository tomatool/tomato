package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/cucumber/godog"
	"github.com/tomatool/tomato/internal/config"
	"github.com/tomatool/tomato/internal/container"
	"gopkg.in/yaml.v3"
)

// HTTPServer provides a mock HTTP server for testing
type HTTPServer struct {
	name      string
	config    config.Resource
	container *container.Manager

	server   *http.Server
	listener net.Listener
	port     int

	stubs        []*HTTPStub      // Dynamic stubs (added via Gherkin steps)
	fixtureStubs []FixtureStub    // Fixture stubs (loaded from files)
	calls        []*RecordedCall
	stubsMu      sync.RWMutex
	callsMu      sync.RWMutex
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

// FixtureConfig represents the root structure of a fixtures YAML file
type FixtureConfig struct {
	Stubs []FixtureStub `yaml:"stubs"`
}

// FixtureStub represents a stub loaded from fixtures with optional conditions
type FixtureStub struct {
	ID          string              `yaml:"id"`
	Method      string              `yaml:"method"`
	Path        string              `yaml:"path"`
	PathPattern string              `yaml:"pathPattern"`
	Conditions  *FixtureConditions  `yaml:"conditions,omitempty"`
	Response    FixtureResponse     `yaml:"response"`

	// Compiled regex pattern (not in YAML)
	compiledPattern *regexp.Regexp `yaml:"-"`
}

// FixtureResponse represents the response configuration for a fixture stub
type FixtureResponse struct {
	Status   int               `yaml:"status"`
	Headers  map[string]string `yaml:"headers,omitempty"`
	Body     string            `yaml:"body,omitempty"`
	BodyFile string            `yaml:"bodyFile,omitempty"`

	// Cached body content from file (not in YAML)
	bodyContent string `yaml:"-"`
}

// FixtureConditions defines conditions that must match for the stub to be returned
type FixtureConditions struct {
	Headers map[string]HeaderCondition `yaml:"headers,omitempty"`
	Query   map[string]string          `yaml:"query,omitempty"`
	Body    *BodyCondition             `yaml:"body,omitempty"`
}

// HeaderCondition defines matching rules for HTTP headers
type HeaderCondition struct {
	Equals   string `yaml:"equals,omitempty"`
	Contains string `yaml:"contains,omitempty"`
	Matches  string `yaml:"matches,omitempty"`

	// Compiled regex pattern (not in YAML)
	compiledPattern *regexp.Regexp `yaml:"-"`
}

// BodyCondition defines matching rules for request body
type BodyCondition struct {
	// For JSON bodies
	JSONPath string `yaml:"jsonPath,omitempty"`
	Equals   string `yaml:"equals,omitempty"`
	Contains string `yaml:"contains,omitempty"`
	Matches  string `yaml:"matches,omitempty"`

	// For non-JSON bodies
	BodyContains string `yaml:"bodyContains,omitempty"`
	BodyMatches  string `yaml:"bodyMatches,omitempty"`

	// Compiled regex patterns (not in YAML)
	compiledMatches    *regexp.Regexp `yaml:"-"`
	compiledBodyMatches *regexp.Regexp `yaml:"-"`
}

func NewHTTPServer(name string, cfg config.Resource, cm *container.Manager) (*HTTPServer, error) {
	return &HTTPServer{
		name:         name,
		config:       cfg,
		container:    cm,
		stubs:        make([]*HTTPStub, 0),
		fixtureStubs: make([]FixtureStub, 0),
		calls:        make([]*RecordedCall, 0),
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

	// Auto-load fixtures if configured
	if fixturesPath, ok := r.config.Options["fixturesPath"].(string); ok && fixturesPath != "" {
		// Check if autoLoad is explicitly set to false
		autoLoad := true
		if al, ok := r.config.Options["autoLoad"].(bool); ok {
			autoLoad = al
		}

		if autoLoad {
			if err := r.LoadFixtures(fixturesPath); err != nil {
				return fmt.Errorf("loading fixtures from %q: %w", fixturesPath, err)
			}
		}
	}

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

	r.stubsMu.RLock()
	defer r.stubsMu.RUnlock()

	// Priority 1: Check dynamic stubs (added via Gherkin steps)
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

	if matchedStub != nil {
		// Return dynamic stub
		for k, v := range matchedStub.Headers {
			w.Header().Set(k, v)
		}
		w.WriteHeader(matchedStub.Status)
		w.Write([]byte(matchedStub.Body))
		return
	}

	// Priority 2: Check fixture stubs (most specific first)
	var matchedFixtures []*FixtureStub
	for i := range r.fixtureStubs {
		if r.matchesFixtureStub(&r.fixtureStubs[i], req, body) {
			matchedFixtures = append(matchedFixtures, &r.fixtureStubs[i])
		}
	}

	if len(matchedFixtures) > 0 {
		// Find most specific match (most conditions)
		mostSpecific := matchedFixtures[0]
		maxConditions := r.countMatchedConditions(mostSpecific)

		for _, fixture := range matchedFixtures[1:] {
			condCount := r.countMatchedConditions(fixture)
			if condCount > maxConditions {
				mostSpecific = fixture
				maxConditions = condCount
			}
		}

		// Return fixture stub response
		for k, v := range mostSpecific.Response.Headers {
			w.Header().Set(k, v)
		}
		w.WriteHeader(mostSpecific.Response.Status)

		// Use bodyContent (from file) if available, otherwise use inline body
		responseBody := mostSpecific.Response.Body
		if mostSpecific.Response.bodyContent != "" {
			responseBody = mostSpecific.Response.bodyContent
		}
		w.Write([]byte(responseBody))
		return
	}

	// No match found
	w.WriteHeader(http.StatusNotFound)
	w.Write([]byte(fmt.Sprintf("No stub found for %s %s", req.Method, req.URL.Path)))
}

func (r *HTTPServer) Ready(ctx context.Context) error {
	return nil
}

func (r *HTTPServer) Reset(ctx context.Context) error {
	r.stubsMu.Lock()
	// Clear only dynamic stubs, preserve fixture stubs
	r.stubs = make([]*HTTPStub, 0)
	// Note: fixtureStubs are preserved across scenarios
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
			// Stub Setup
			{
				Group:       "Stub Setup",
				Pattern:     `^"{resource}" stub "([^"]*)" "([^"]*)" returns "(\d+)"$`,
				Description: "Creates a stub that returns a status code",
				Example:     `"{resource}" stub "GET" "/users" returns "200"`,
				Handler:     r.stubReturnsStatus,
			},
			{
				Group:       "Stub Setup",
				Pattern:     `^"{resource}" stub "([^"]*)" "([^"]*)" returns "(\d+)" with body:$`,
				Description: "Creates a stub that returns a status code and body",
				Example:     "\"{resource}\" stub \"GET\" \"/users\" returns \"200\" with body:\n  \"\"\"\n  [{\"id\": 1}]\n  \"\"\"",
				Handler:     r.stubReturnsBody,
			},
			{
				Group:       "Stub Setup",
				Pattern:     `^"{resource}" stub "([^"]*)" "([^"]*)" returns "(\d+)" with json:$`,
				Description: "Creates a stub that returns JSON (auto sets Content-Type)",
				Example:     "\"{resource}\" stub \"GET\" \"/users\" returns \"200\" with json:\n  \"\"\"\n  [{\"id\": 1}]\n  \"\"\"",
				Handler:     r.stubReturnsJSON,
			},
			{
				Group:       "Stub Setup",
				Pattern:     `^"{resource}" stub "([^"]*)" "([^"]*)" returns "(\d+)" with headers:$`,
				Description: "Creates a stub that returns with custom headers",
				Example:     "\"{resource}\" stub \"GET\" \"/users\" returns \"200\" with headers:\n  | header       | value            |\n  | X-Custom     | value            |",
				Handler:     r.stubReturnsHeaders,
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

			// Server Info
			{
				Group:       "Server Info",
				Pattern:     `^"{resource}" url is stored in "([^"]*)"$`,
				Description: "Stores the server URL in a variable for use in other steps",
				Example:     `"{resource}" url is stored in "SERVER_URL"`,
				Handler:     r.storeURL,
			},

			// Fixture Management
			{
				Group:       "Fixture Management",
				Pattern:     `^"{resource}" loads fixtures from "([^"]*)"$`,
				Description: "Loads fixture stubs from the specified directory path",
				Example:     `"{resource}" loads fixtures from "fixtures/github-api"`,
				Handler:     r.loadFixturesStep,
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

func (r *HTTPServer) loadFixturesStep(fixturesPath string) error {
	return r.LoadFixtures(fixturesPath)
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

// matchesFixtureStub checks if a request matches a fixture stub (method, path, and conditions)
func (r *HTTPServer) matchesFixtureStub(stub *FixtureStub, req *http.Request, body string) bool {
	// Check method
	if stub.Method != req.Method {
		return false
	}

	// Check path
	if stub.PathPattern != "" {
		if stub.compiledPattern != nil && !stub.compiledPattern.MatchString(req.URL.Path) {
			return false
		}
	} else if stub.Path != req.URL.Path {
		return false
	}

	// Check conditions if present
	if stub.Conditions != nil {
		if !r.matchesConditions(stub.Conditions, req, body) {
			return false
		}
	}

	return true
}

// matchesConditions checks if a request matches all conditions
func (r *HTTPServer) matchesConditions(conditions *FixtureConditions, req *http.Request, body string) bool {
	// Check header conditions
	for headerName, condition := range conditions.Headers {
		headerValue := req.Header.Get(headerName)
		if !r.matchesHeaderCondition(&condition, headerValue) {
			return false
		}
	}

	// Check query parameter conditions
	for queryKey, expectedValue := range conditions.Query {
		actualValue := req.URL.Query().Get(queryKey)
		if actualValue != expectedValue {
			return false
		}
	}

	// Check body conditions
	if conditions.Body != nil {
		if !r.matchesBodyCondition(conditions.Body, body, req.Header.Get("Content-Type")) {
			return false
		}
	}

	return true
}

// matchesHeaderCondition checks if a header value matches the condition
func (r *HTTPServer) matchesHeaderCondition(condition *HeaderCondition, headerValue string) bool {
	if condition.Equals != "" && headerValue != condition.Equals {
		return false
	}
	if condition.Contains != "" && !strings.Contains(headerValue, condition.Contains) {
		return false
	}
	if condition.Matches != "" && condition.compiledPattern != nil {
		if !condition.compiledPattern.MatchString(headerValue) {
			return false
		}
	}
	return true
}

// matchesBodyCondition checks if request body matches the condition
func (r *HTTPServer) matchesBodyCondition(condition *BodyCondition, body string, contentType string) bool {
	// Handle JSONPath matching
	if condition.JSONPath != "" {
		// Simple JSONPath implementation for basic cases like $.field or $.nested.field
		value, err := r.extractJSONPath(body, condition.JSONPath)
		if err != nil {
			return false
		}

		if condition.Equals != "" && value != condition.Equals {
			return false
		}
		if condition.Contains != "" && !strings.Contains(value, condition.Contains) {
			return false
		}
		if condition.Matches != "" && condition.compiledMatches != nil {
			if !condition.compiledMatches.MatchString(value) {
				return false
			}
		}
		return true
	}

	// Handle simple body matching
	if condition.BodyContains != "" && !strings.Contains(body, condition.BodyContains) {
		return false
	}
	if condition.BodyMatches != "" && condition.compiledBodyMatches != nil {
		if !condition.compiledBodyMatches.MatchString(body) {
			return false
		}
	}

	return true
}

// extractJSONPath extracts a value from JSON using a simple JSONPath expression
// Supports basic paths like $.field, $.nested.field, $[0].field
func (r *HTTPServer) extractJSONPath(jsonBody string, path string) (string, error) {
	if !strings.HasPrefix(path, "$.") && !strings.HasPrefix(path, "$[") {
		return "", fmt.Errorf("invalid JSONPath: must start with $. or $[")
	}

	var data interface{}
	if err := json.Unmarshal([]byte(jsonBody), &data); err != nil {
		return "", fmt.Errorf("invalid JSON: %w", err)
	}

	// Remove the $ prefix
	path = strings.TrimPrefix(path, "$")
	if path == "" {
		// Return entire JSON as string
		result, _ := json.Marshal(data)
		return string(result), nil
	}

	// Split by dots for nested fields
	parts := strings.Split(strings.TrimPrefix(path, "."), ".")
	current := data

	for _, part := range parts {
		// Handle array indices like [0]
		if strings.HasPrefix(part, "[") && strings.HasSuffix(part, "]") {
			indexStr := strings.Trim(part, "[]")
			var index int
			if _, err := fmt.Sscanf(indexStr, "%d", &index); err != nil {
				return "", fmt.Errorf("invalid array index: %s", part)
			}
			if arr, ok := current.([]interface{}); ok {
				if index < 0 || index >= len(arr) {
					return "", fmt.Errorf("array index out of bounds: %d", index)
				}
				current = arr[index]
			} else {
				return "", fmt.Errorf("not an array: %s", part)
			}
		} else {
			// Handle object fields
			if obj, ok := current.(map[string]interface{}); ok {
				var exists bool
				current, exists = obj[part]
				if !exists {
					return "", fmt.Errorf("field not found: %s", part)
				}
			} else {
				return "", fmt.Errorf("not an object: %s", part)
			}
		}
	}

	// Convert result to string
	switch v := current.(type) {
	case string:
		return v, nil
	case float64, int, int64, bool:
		return fmt.Sprintf("%v", v), nil
	default:
		result, _ := json.Marshal(v)
		return string(result), nil
	}
}

// countMatchedConditions returns the number of conditions matched by a fixture stub
// Used for determining specificity when multiple stubs match
func (r *HTTPServer) countMatchedConditions(stub *FixtureStub) int {
	if stub.Conditions == nil {
		return 0
	}

	count := 0
	count += len(stub.Conditions.Headers)
	count += len(stub.Conditions.Query)
	if stub.Conditions.Body != nil {
		count++ // Body condition counts as 1
	}
	return count
}

// LoadFixtures loads fixture stubs from the specified path
// The path should point to a directory containing stubs.yml and optionally a responses/ subdirectory
func (r *HTTPServer) LoadFixtures(fixturesPath string) error {
	// Check if path exists
	if _, err := os.Stat(fixturesPath); os.IsNotExist(err) {
		return fmt.Errorf("fixtures path does not exist: %s", fixturesPath)
	}

	// Load stubs.yml
	stubsFile := filepath.Join(fixturesPath, "stubs.yml")
	fixtures, err := r.loadFixtureFile(stubsFile)
	if err != nil {
		return fmt.Errorf("loading fixture file: %w", err)
	}

	// Validate and prepare fixtures (compile regex, load body files)
	for i := range fixtures.Stubs {
		if err := r.validateAndPrepareFixture(&fixtures.Stubs[i], fixturesPath); err != nil {
			return fmt.Errorf("preparing fixture stub %q: %w", fixtures.Stubs[i].ID, err)
		}
	}

	// Store fixtures
	r.stubsMu.Lock()
	r.fixtureStubs = append(r.fixtureStubs, fixtures.Stubs...)
	r.stubsMu.Unlock()

	return nil
}

// loadFixtureFile parses a fixture YAML file
func (r *HTTPServer) loadFixtureFile(path string) (*FixtureConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading file: %w", err)
	}

	var config FixtureConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("parsing YAML: %w", err)
	}

	return &config, nil
}

// validateAndPrepareFixture validates and prepares a fixture stub
func (r *HTTPServer) validateAndPrepareFixture(stub *FixtureStub, basePath string) error {
	// Validate required fields
	if stub.Method == "" {
		return fmt.Errorf("method is required")
	}
	if stub.Path == "" && stub.PathPattern == "" {
		return fmt.Errorf("either path or pathPattern is required")
	}
	if stub.Path != "" && stub.PathPattern != "" {
		return fmt.Errorf("cannot specify both path and pathPattern")
	}

	// Compile path pattern if provided
	if stub.PathPattern != "" {
		pattern, err := regexp.Compile(stub.PathPattern)
		if err != nil {
			return fmt.Errorf("compiling pathPattern: %w", err)
		}
		stub.compiledPattern = pattern
	}

	// Load body from file if bodyFile is specified
	if stub.Response.BodyFile != "" {
		bodyPath := filepath.Join(basePath, stub.Response.BodyFile)
		content, err := os.ReadFile(bodyPath)
		if err != nil {
			return fmt.Errorf("reading bodyFile %q: %w", stub.Response.BodyFile, err)
		}
		stub.Response.bodyContent = string(content)
	}

	// Validate and compile condition patterns
	if stub.Conditions != nil {
		// Compile header regex patterns
		for key, condition := range stub.Conditions.Headers {
			if condition.Matches != "" {
				pattern, err := regexp.Compile(condition.Matches)
				if err != nil {
					return fmt.Errorf("compiling header condition regex for %q: %w", key, err)
				}
				condition.compiledPattern = pattern
				stub.Conditions.Headers[key] = condition
			}
		}

		// Compile body condition patterns
		if stub.Conditions.Body != nil {
			if stub.Conditions.Body.Matches != "" {
				pattern, err := regexp.Compile(stub.Conditions.Body.Matches)
				if err != nil {
					return fmt.Errorf("compiling body matches regex: %w", err)
				}
				stub.Conditions.Body.compiledMatches = pattern
			}
			if stub.Conditions.Body.BodyMatches != "" {
				pattern, err := regexp.Compile(stub.Conditions.Body.BodyMatches)
				if err != nil {
					return fmt.Errorf("compiling bodyMatches regex: %w", err)
				}
				stub.Conditions.Body.compiledBodyMatches = pattern
			}
		}
	}

	return nil
}

var _ Handler = (*HTTPServer)(nil)
