package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/cucumber/godog"
	"github.com/tomatool/tomato/internal/config"
	"github.com/tomatool/tomato/internal/container"
)

type HTTP struct {
	name      string
	config    config.Resource
	container *container.Manager
	client    *http.Client
	baseURL   string

	requestHeaders map[string]string
	requestBody    []byte
	requestParams  url.Values

	lastResponse *http.Response
	lastBody     []byte
}

func NewHTTP(name string, cfg config.Resource, cm *container.Manager) (*HTTP, error) {
	return &HTTP{
		name:           name,
		config:         cfg,
		container:      cm,
		requestHeaders: make(map[string]string),
		requestParams:  make(url.Values),
	}, nil
}

func (r *HTTP) Name() string { return r.name }

func (r *HTTP) Init(ctx context.Context) error {
	timeout := 30 * time.Second
	if t, ok := r.config.Options["timeout"].(string); ok {
		if d, err := time.ParseDuration(t); err == nil {
			timeout = d
		}
	}

	r.client = &http.Client{
		Timeout: timeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if noRedirect, ok := r.config.Options["no_redirect"].(bool); ok && noRedirect {
				return http.ErrUseLastResponse
			}
			return nil
		},
	}

	if r.config.BaseURL != "" {
		r.baseURL = r.config.BaseURL
	} else if r.config.Container != "" {
		host, err := r.container.GetHost(ctx, r.config.Container)
		if err != nil {
			return fmt.Errorf("getting container host: %w", err)
		}

		port := "8080"
		if p, ok := r.config.Options["port"].(string); ok {
			port = p
		}

		mappedPort, err := r.container.GetPort(ctx, r.config.Container, port+"/tcp")
		if err != nil {
			return fmt.Errorf("getting container port: %w", err)
		}

		scheme := "http"
		if s, ok := r.config.Options["scheme"].(string); ok {
			scheme = s
		}

		r.baseURL = fmt.Sprintf("%s://%s:%s", scheme, host, mappedPort)
	}

	return nil
}

func (r *HTTP) Ready(ctx context.Context) error {
	if healthPath, ok := r.config.Options["health_path"].(string); ok {
		resp, err := r.client.Get(r.baseURL + healthPath)
		if err != nil {
			return fmt.Errorf("health check failed: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode >= 400 {
			return fmt.Errorf("health check returned status %d", resp.StatusCode)
		}
	}
	return nil
}

func (r *HTTP) Reset(ctx context.Context) error {
	r.requestHeaders = make(map[string]string)
	r.requestBody = nil
	r.requestParams = make(url.Values)
	r.lastResponse = nil
	r.lastBody = nil
	return nil
}

func (r *HTTP) RegisterSteps(ctx *godog.ScenarioContext) {
	RegisterStepsToGodog(ctx, r.name, r.Steps())
}

// Steps returns the structured step definitions for the HTTP handler
func (r *HTTP) Steps() StepCategory {
	return StepCategory{
		Name:        "HTTP",
		Description: "Steps for making HTTP requests and validating responses",
		Steps: []StepDef{
			// Request setup steps
			{
				Pattern:     `^I set "{resource}" header "([^"]*)" to "([^"]*)"$`,
				Description: "Sets a header for the next HTTP request",
				Example:     `I set "{resource}" header "Content-Type" to "application/json"`,
				Handler:     r.setHeader,
			},
			{
				Pattern:     `^I set "{resource}" headers:$`,
				Description: "Sets multiple headers for the next HTTP request using a table",
				Example:     "I set \"{resource}\" headers:\n  | header       | value            |\n  | Content-Type | application/json |",
				Handler:     r.setHeaders,
			},
			{
				Pattern:     `^I set "{resource}" query param "([^"]*)" to "([^"]*)"$`,
				Description: "Sets a query parameter for the next HTTP request",
				Example:     `I set "{resource}" query param "page" to "1"`,
				Handler:     r.setQueryParam,
			},
			{
				Pattern:     `^I set "{resource}" request body:$`,
				Description: "Sets the raw request body for the next HTTP request",
				Example:     "I set \"{resource}\" request body:\n  \"\"\"\n  raw body content\n  \"\"\"",
				Handler:     r.setRequestBody,
			},
			{
				Pattern:     `^I set "{resource}" JSON body:$`,
				Description: "Sets a JSON request body and automatically sets Content-Type header",
				Example:     "I set \"{resource}\" JSON body:\n  \"\"\"\n  {\"name\": \"test\"}\n  \"\"\"",
				Handler:     r.setJSONBody,
			},
			{
				Pattern:     `^I set "{resource}" form body:$`,
				Description: "Sets form-encoded body from a table and sets Content-Type header",
				Example:     "I set \"{resource}\" form body:\n  | field | value |\n  | name  | test  |",
				Handler:     r.setFormBody,
			},

			// Request execution steps
			{
				Pattern:     `^I send "([^"]*)" request to "{resource}" "([^"]*)"$`,
				Description: "Sends an HTTP request with the specified method to the given path",
				Example:     `I send "GET" request to "{resource}" "/api/users"`,
				Handler:     r.sendRequest,
			},
			{
				Pattern:     `^I send "([^"]*)" request to "{resource}" "([^"]*)" with body:$`,
				Description: "Sends an HTTP request with a raw body",
				Example:     "I send \"POST\" request to \"{resource}\" \"/api/users\" with body:\n  \"\"\"\n  raw body\n  \"\"\"",
				Handler:     r.sendRequestWithBody,
			},
			{
				Pattern:     `^I send "([^"]*)" request to "{resource}" "([^"]*)" with JSON:$`,
				Description: "Sends an HTTP request with a JSON body",
				Example:     "I send \"POST\" request to \"{resource}\" \"/api/users\" with JSON:\n  \"\"\"\n  {\"name\": \"John\"}\n  \"\"\"",
				Handler:     r.sendRequestWithJSON,
			},

			// Response status steps
			{
				Pattern:     `^"{resource}" response status should be "(\d+)"$`,
				Description: "Asserts the response has the exact HTTP status code",
				Example:     `"{resource}" response status should be "200"`,
				Handler:     r.responseStatusShouldBe,
			},
			{
				Pattern:     `^"{resource}" response status should be (success|redirect|client error|server error)$`,
				Description: "Asserts the response status is in the given class (2xx, 3xx, 4xx, 5xx)",
				Example:     `"{resource}" response status should be success`,
				Handler:     r.responseStatusClassShouldBe,
			},

			// Response header steps
			{
				Pattern:     `^"{resource}" response header "([^"]*)" should be "([^"]*)"$`,
				Description: "Asserts a response header has the exact value",
				Example:     `"{resource}" response header "Content-Type" should be "application/json"`,
				Handler:     r.responseHeaderShouldBe,
			},
			{
				Pattern:     `^"{resource}" response header "([^"]*)" should contain "([^"]*)"$`,
				Description: "Asserts a response header contains a substring",
				Example:     `"{resource}" response header "Content-Type" should contain "json"`,
				Handler:     r.responseHeaderShouldContain,
			},
			{
				Pattern:     `^"{resource}" response header "([^"]*)" should exist$`,
				Description: "Asserts a response header exists",
				Example:     `"{resource}" response header "X-Request-Id" should exist`,
				Handler:     r.responseHeaderShouldExist,
			},

			// Response body steps
			{
				Pattern:     `^"{resource}" response body should be:$`,
				Description: "Asserts the response body matches exactly",
				Example:     "\"{resource}\" response body should be:\n  \"\"\"\n  expected body\n  \"\"\"",
				Handler:     r.responseBodyShouldBe,
			},
			{
				Pattern:     `^"{resource}" response body should contain "([^"]*)"$`,
				Description: "Asserts the response body contains a substring",
				Example:     `"{resource}" response body should contain "success"`,
				Handler:     r.responseBodyShouldContain,
			},
			{
				Pattern:     `^"{resource}" response body should not contain "([^"]*)"$`,
				Description: "Asserts the response body does not contain a substring",
				Example:     `"{resource}" response body should not contain "error"`,
				Handler:     r.responseBodyShouldNotContain,
			},
			{
				Pattern:     `^"{resource}" response body should be empty$`,
				Description: "Asserts the response body is empty",
				Example:     `"{resource}" response body should be empty`,
				Handler:     r.responseBodyShouldBeEmpty,
			},

			// Response JSON steps
			{
				Pattern:     `^"{resource}" response JSON "([^"]*)" should be "([^"]*)"$`,
				Description: "Asserts a JSON path in the response has the expected value",
				Example:     `"{resource}" response JSON "data.id" should be "123"`,
				Handler:     r.responseJSONPathShouldBe,
			},
			{
				Pattern:     `^"{resource}" response JSON "([^"]*)" should exist$`,
				Description: "Asserts a JSON path exists in the response",
				Example:     `"{resource}" response JSON "data.id" should exist`,
				Handler:     r.responseJSONPathShouldExist,
			},
			{
				Pattern:     `^"{resource}" response JSON "([^"]*)" should not exist$`,
				Description: "Asserts a JSON path does not exist in the response",
				Example:     `"{resource}" response JSON "data.deleted" should not exist`,
				Handler:     r.responseJSONPathShouldNotExist,
			},
			{
				Pattern:     `^"{resource}" response JSON should match:$`,
				Description: "Asserts the response JSON matches the expected structure. Use @string, @number, @boolean, @array, @object, @any, @null, @notnull as type matchers",
				Example:     "\"{resource}\" response JSON should match:\n  \"\"\"\n  {\"id\": \"@number\", \"name\": \"@string\"}\n  \"\"\"",
				Handler:     r.responseJSONShouldMatch,
			},

			// Response timing steps
			{
				Pattern:     `^"{resource}" response time should be less than "([^"]*)"$`,
				Description: "Asserts the response was received within the given duration",
				Example:     `"{resource}" response time should be less than "500ms"`,
				Handler:     r.responseTimeShouldBeLessThan,
			},
		},
	}
}

func (r *HTTP) setHeader(key, value string) error {
	r.requestHeaders[key] = value
	return nil
}

func (r *HTTP) setHeaders(table *godog.Table) error {
	for _, row := range table.Rows[1:] {
		if len(row.Cells) >= 2 {
			r.requestHeaders[row.Cells[0].Value] = row.Cells[1].Value
		}
	}
	return nil
}

func (r *HTTP) setQueryParam(key, value string) error {
	r.requestParams.Set(key, value)
	return nil
}

func (r *HTTP) setRequestBody(doc *godog.DocString) error {
	r.requestBody = []byte(doc.Content)
	return nil
}

func (r *HTTP) setJSONBody(doc *godog.DocString) error {
	var js json.RawMessage
	if err := json.Unmarshal([]byte(doc.Content), &js); err != nil {
		return fmt.Errorf("invalid JSON: %w", err)
	}
	r.requestBody = []byte(doc.Content)
	if r.requestHeaders["Content-Type"] == "" {
		r.requestHeaders["Content-Type"] = "application/json"
	}
	return nil
}

func (r *HTTP) setFormBody(table *godog.Table) error {
	form := url.Values{}
	for _, row := range table.Rows[1:] {
		if len(row.Cells) >= 2 {
			form.Set(row.Cells[0].Value, row.Cells[1].Value)
		}
	}
	r.requestBody = []byte(form.Encode())
	if r.requestHeaders["Content-Type"] == "" {
		r.requestHeaders["Content-Type"] = "application/x-www-form-urlencoded"
	}
	return nil
}

func (r *HTTP) sendRequest(method, path string) error {
	return r.doRequest(method, path, nil)
}

func (r *HTTP) sendRequestWithBody(method, path string, doc *godog.DocString) error {
	return r.doRequest(method, path, []byte(doc.Content))
}

func (r *HTTP) sendRequestWithJSON(method, path string, doc *godog.DocString) error {
	var js json.RawMessage
	if err := json.Unmarshal([]byte(doc.Content), &js); err != nil {
		return fmt.Errorf("invalid JSON: %w", err)
	}
	if r.requestHeaders["Content-Type"] == "" {
		r.requestHeaders["Content-Type"] = "application/json"
	}
	return r.doRequest(method, path, []byte(doc.Content))
}

func (r *HTTP) doRequest(method, path string, body []byte) error {
	reqURL := r.baseURL + path
	if len(r.requestParams) > 0 {
		reqURL += "?" + r.requestParams.Encode()
	}

	var bodyReader io.Reader
	if body != nil {
		bodyReader = bytes.NewReader(body)
	} else if r.requestBody != nil {
		bodyReader = bytes.NewReader(r.requestBody)
	}

	req, err := http.NewRequest(method, reqURL, bodyReader)
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}

	for k, v := range r.requestHeaders {
		req.Header.Set(k, v)
	}

	start := time.Now()
	resp, err := r.client.Do(req)
	if err != nil {
		return fmt.Errorf("sending request: %w", err)
	}

	r.lastResponse = resp
	r.lastBody, _ = io.ReadAll(resp.Body)
	resp.Body.Close()

	r.lastResponse.Header.Set("X-Response-Time", time.Since(start).String())

	r.requestHeaders = make(map[string]string)
	r.requestBody = nil
	r.requestParams = make(url.Values)

	return nil
}

func (r *HTTP) responseStatusShouldBe(expected int) error {
	if r.lastResponse == nil {
		return fmt.Errorf("no response received")
	}
	if r.lastResponse.StatusCode != expected {
		return fmt.Errorf("expected status %d, got %d\nBody: %s", expected, r.lastResponse.StatusCode, string(r.lastBody))
	}
	return nil
}

func (r *HTTP) responseStatusClassShouldBe(class string) error {
	if r.lastResponse == nil {
		return fmt.Errorf("no response received")
	}

	status := r.lastResponse.StatusCode
	var ok bool

	switch class {
	case "success":
		ok = status >= 200 && status < 300
	case "redirect":
		ok = status >= 300 && status < 400
	case "client error":
		ok = status >= 400 && status < 500
	case "server error":
		ok = status >= 500 && status < 600
	default:
		return fmt.Errorf("unknown status class: %s", class)
	}

	if !ok {
		return fmt.Errorf("expected %s status, got %d", class, status)
	}
	return nil
}

func (r *HTTP) responseHeaderShouldBe(header, expected string) error {
	if r.lastResponse == nil {
		return fmt.Errorf("no response received")
	}
	actual := r.lastResponse.Header.Get(header)
	if actual != expected {
		return fmt.Errorf("header %q: expected %q, got %q", header, expected, actual)
	}
	return nil
}

func (r *HTTP) responseHeaderShouldContain(header, substr string) error {
	if r.lastResponse == nil {
		return fmt.Errorf("no response received")
	}
	actual := r.lastResponse.Header.Get(header)
	if !strings.Contains(actual, substr) {
		return fmt.Errorf("header %q value %q does not contain %q", header, actual, substr)
	}
	return nil
}

func (r *HTTP) responseHeaderShouldExist(header string) error {
	if r.lastResponse == nil {
		return fmt.Errorf("no response received")
	}
	if r.lastResponse.Header.Get(header) == "" {
		return fmt.Errorf("header %q does not exist", header)
	}
	return nil
}

func (r *HTTP) responseBodyShouldBe(doc *godog.DocString) error {
	if r.lastResponse == nil {
		return fmt.Errorf("no response received")
	}
	expected := strings.TrimSpace(doc.Content)
	actual := strings.TrimSpace(string(r.lastBody))
	if actual != expected {
		return fmt.Errorf("body mismatch:\nexpected: %s\nactual: %s", expected, actual)
	}
	return nil
}

func (r *HTTP) responseBodyShouldContain(substr string) error {
	if r.lastResponse == nil {
		return fmt.Errorf("no response received")
	}
	if !strings.Contains(string(r.lastBody), substr) {
		return fmt.Errorf("body does not contain %q\nbody: %s", substr, string(r.lastBody))
	}
	return nil
}

func (r *HTTP) responseBodyShouldNotContain(substr string) error {
	if r.lastResponse == nil {
		return fmt.Errorf("no response received")
	}
	if strings.Contains(string(r.lastBody), substr) {
		return fmt.Errorf("body should not contain %q\nbody: %s", substr, string(r.lastBody))
	}
	return nil
}

func (r *HTTP) responseBodyShouldBeEmpty() error {
	if r.lastResponse == nil {
		return fmt.Errorf("no response received")
	}
	if len(r.lastBody) > 0 {
		return fmt.Errorf("expected empty body, got: %s", string(r.lastBody))
	}
	return nil
}

func (r *HTTP) responseJSONPathShouldBe(path, expected string) error {
	if r.lastResponse == nil {
		return fmt.Errorf("no response received")
	}

	actual, err := r.getJSONPath(path)
	if err != nil {
		return err
	}

	actualStr := fmt.Sprintf("%v", actual)
	if actualStr != expected {
		return fmt.Errorf("JSON path %q: expected %q, got %q", path, expected, actualStr)
	}
	return nil
}

func (r *HTTP) responseJSONPathShouldExist(path string) error {
	if r.lastResponse == nil {
		return fmt.Errorf("no response received")
	}
	_, err := r.getJSONPath(path)
	return err
}

func (r *HTTP) responseJSONPathShouldNotExist(path string) error {
	if r.lastResponse == nil {
		return fmt.Errorf("no response received")
	}
	_, err := r.getJSONPath(path)
	if err == nil {
		return fmt.Errorf("JSON path %q exists but should not", path)
	}
	return nil
}

func (r *HTTP) responseJSONShouldMatch(doc *godog.DocString) error {
	if r.lastResponse == nil {
		return fmt.Errorf("no response received")
	}

	var expected, actual interface{}
	if err := json.Unmarshal([]byte(doc.Content), &expected); err != nil {
		return fmt.Errorf("invalid expected JSON: %w", err)
	}
	if err := json.Unmarshal(r.lastBody, &actual); err != nil {
		return fmt.Errorf("invalid response JSON: %w", err)
	}

	return r.compareJSON(expected, actual, "")
}

func (r *HTTP) compareJSON(expected, actual interface{}, path string) error {
	switch e := expected.(type) {
	case map[string]interface{}:
		a, ok := actual.(map[string]interface{})
		if !ok {
			return fmt.Errorf("at %s: expected object, got %T", path, actual)
		}
		for key, val := range e {
			newPath := path + "." + key
			if path == "" {
				newPath = key
			}
			actualVal, exists := a[key]
			if !exists {
				return fmt.Errorf("at %s: key %q not found", path, key)
			}
			if err := r.compareJSON(val, actualVal, newPath); err != nil {
				return err
			}
		}
	case []interface{}:
		a, ok := actual.([]interface{})
		if !ok {
			return fmt.Errorf("at %s: expected array, got %T", path, actual)
		}
		if len(e) != len(a) {
			return fmt.Errorf("at %s: expected array length %d, got %d", path, len(e), len(a))
		}
		for i, val := range e {
			newPath := fmt.Sprintf("%s[%d]", path, i)
			if err := r.compareJSON(val, a[i], newPath); err != nil {
				return err
			}
		}
	default:
		if str, ok := expected.(string); ok {
			if strings.HasPrefix(str, "@") {
				return r.matchSpecial(str, actual, path)
			}
		}
		if fmt.Sprintf("%v", expected) != fmt.Sprintf("%v", actual) {
			return fmt.Errorf("at %s: expected %v, got %v", path, expected, actual)
		}
	}
	return nil
}

func (r *HTTP) matchSpecial(matcher string, actual interface{}, path string) error {
	switch matcher {
	case "@string":
		if _, ok := actual.(string); !ok {
			return fmt.Errorf("at %s: expected string, got %T", path, actual)
		}
	case "@number":
		if _, ok := actual.(float64); !ok {
			return fmt.Errorf("at %s: expected number, got %T", path, actual)
		}
	case "@boolean":
		if _, ok := actual.(bool); !ok {
			return fmt.Errorf("at %s: expected boolean, got %T", path, actual)
		}
	case "@array":
		if _, ok := actual.([]interface{}); !ok {
			return fmt.Errorf("at %s: expected array, got %T", path, actual)
		}
	case "@object":
		if _, ok := actual.(map[string]interface{}); !ok {
			return fmt.Errorf("at %s: expected object, got %T", path, actual)
		}
	case "@any":
		// Always matches
	case "@null":
		if actual != nil {
			return fmt.Errorf("at %s: expected null, got %v", path, actual)
		}
	case "@notnull":
		if actual == nil {
			return fmt.Errorf("at %s: expected non-null value", path)
		}
	default:
		return fmt.Errorf("unknown matcher: %s", matcher)
	}
	return nil
}

func (r *HTTP) getJSONPath(path string) (interface{}, error) {
	var data interface{}
	if err := json.Unmarshal(r.lastBody, &data); err != nil {
		return nil, fmt.Errorf("invalid JSON: %w", err)
	}

	parts := strings.Split(path, ".")
	current := data

	for _, part := range parts {
		if idx := strings.Index(part, "["); idx != -1 {
			key := part[:idx]
			indexStr := part[idx+1 : len(part)-1]
			index, err := strconv.Atoi(indexStr)
			if err != nil {
				return nil, fmt.Errorf("invalid array index: %s", indexStr)
			}

			if key != "" {
				obj, ok := current.(map[string]interface{})
				if !ok {
					return nil, fmt.Errorf("expected object at %s", key)
				}
				current = obj[key]
			}

			arr, ok := current.([]interface{})
			if !ok {
				return nil, fmt.Errorf("expected array at %s", part)
			}
			if index >= len(arr) {
				return nil, fmt.Errorf("array index out of bounds: %d", index)
			}
			current = arr[index]
		} else {
			obj, ok := current.(map[string]interface{})
			if !ok {
				return nil, fmt.Errorf("expected object at %s", part)
			}
			var exists bool
			current, exists = obj[part]
			if !exists {
				return nil, fmt.Errorf("key not found: %s", part)
			}
		}
	}

	return current, nil
}

func (r *HTTP) responseTimeShouldBeLessThan(duration string) error {
	if r.lastResponse == nil {
		return fmt.Errorf("no response received")
	}

	expected, err := time.ParseDuration(duration)
	if err != nil {
		return fmt.Errorf("invalid duration: %w", err)
	}

	actualStr := r.lastResponse.Header.Get("X-Response-Time")
	actual, err := time.ParseDuration(actualStr)
	if err != nil {
		return fmt.Errorf("invalid response time: %w", err)
	}

	if actual >= expected {
		return fmt.Errorf("response time %v exceeded %v", actual, expected)
	}
	return nil
}

func (r *HTTP) Cleanup(ctx context.Context) error {
	return nil
}

var _ Handler = (*HTTP)(nil)
