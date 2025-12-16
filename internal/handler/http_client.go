package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/cucumber/godog"
	"github.com/tomatool/tomato/internal/config"
	"github.com/tomatool/tomato/internal/container"
)

type HTTPClient struct {
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

func NewHTTPClient(name string, cfg config.Resource, cm *container.Manager) (*HTTPClient, error) {
	return &HTTPClient{
		name:           name,
		config:         cfg,
		container:      cm,
		requestHeaders: make(map[string]string),
		requestParams:  make(url.Values),
	}, nil
}

func (r *HTTPClient) Name() string { return r.name }

func (r *HTTPClient) Init(ctx context.Context) error {
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

func (r *HTTPClient) Ready(ctx context.Context) error {
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

func (r *HTTPClient) Reset(ctx context.Context) error {
	r.requestHeaders = make(map[string]string)
	r.requestBody = nil
	r.requestParams = make(url.Values)
	r.lastResponse = nil
	r.lastBody = nil
	return nil
}

func (r *HTTPClient) RegisterSteps(ctx *godog.ScenarioContext) {
	RegisterStepsToGodog(ctx, r.name, r.Steps())
}

// Steps returns the structured step definitions for the HTTP handler
func (r *HTTPClient) Steps() StepCategory {
	return StepCategory{
		Name:        "HTTP Client",
		Description: "Steps for making HTTP requests and validating responses",
		Steps: []StepDef{
			// Request Setup
			{
				Group:       "Request Setup",
				Pattern:     `^"{resource}" header "([^"]*)" is "([^"]*)"$`,
				Description: "Set a header",
				Example:     `"api" header "Content-Type" is "application/json"`,
				Handler:     r.setHeader,
			},
			{
				Group:       "Request Setup",
				Pattern:     `^"{resource}" headers are:$`,
				Description: "Set multiple headers from table",
				Example:     `"api" headers are:`,
				Handler:     r.setHeaders,
			},
			{
				Group:       "Request Setup",
				Pattern:     `^"{resource}" query param "([^"]*)" is "([^"]*)"$`,
				Description: "Set a query parameter",
				Example:     `"api" query param "page" is "1"`,
				Handler:     r.setQueryParam,
			},
			{
				Group:       "Request Setup",
				Pattern:     `^"{resource}" body is:$`,
				Description: "Set raw request body (docstring)",
				Example:     `"api" body is:`,
				Handler:     r.setRequestBody,
			},
			{
				Group:       "Request Setup",
				Pattern:     `^"{resource}" json body is:$`,
				Description: "Set JSON body + Content-Type header",
				Example:     `"api" json body is:`,
				Handler:     r.setJSONBody,
			},
			{
				Group:       "Request Setup",
				Pattern:     `^"{resource}" form body is:$`,
				Description: "Set form-encoded body from table",
				Example:     `"api" form body is:`,
				Handler:     r.setFormBody,
			},

			// Request Execution
			{
				Group:       "Request Execution",
				Pattern:     `^"{resource}" sends "([^"]*)" to "([^"]*)"$`,
				Description: "Send HTTP request",
				Example:     `"api" sends "GET" to "/users"`,
				Handler:     r.sendRequest,
			},
			{
				Group:       "Request Execution",
				Pattern:     `^"{resource}" sends "([^"]*)" to "([^"]*)" with body:$`,
				Description: "Send with raw body",
				Example:     `"api" sends "POST" to "/users" with body:`,
				Handler:     r.sendRequestWithBody,
			},
			{
				Group:       "Request Execution",
				Pattern:     `^"{resource}" sends "([^"]*)" to "([^"]*)" with json:$`,
				Description: "Send with JSON body",
				Example:     `"api" sends "POST" to "/users" with json:`,
				Handler:     r.sendRequestWithJSON,
			},

			// Response Status
			{
				Group:       "Response Status",
				Pattern:     `^"{resource}" response status is "(\d+)"$`,
				Description: "Assert exact status code",
				Example:     `"api" response status is "200"`,
				Handler:     r.responseStatusShouldBe,
			},
			{
				Group:       "Response Status",
				Pattern:     `^"{resource}" response status is (success|redirect|client error|server error)$`,
				Description: "Assert status class (2xx, 3xx, 4xx, 5xx)",
				Example:     `"api" response status is success`,
				Handler:     r.responseStatusClassShouldBe,
			},

			// Response Headers
			{
				Group:       "Response Headers",
				Pattern:     `^"{resource}" response header "([^"]*)" is "([^"]*)"$`,
				Description: "Assert exact header value",
				Example:     `"api" response header "Content-Type" is "application/json"`,
				Handler:     r.responseHeaderShouldBe,
			},
			{
				Group:       "Response Headers",
				Pattern:     `^"{resource}" response header "([^"]*)" contains "([^"]*)"$`,
				Description: "Assert header contains substring",
				Example:     `"api" response header "Content-Type" contains "json"`,
				Handler:     r.responseHeaderShouldContain,
			},
			{
				Group:       "Response Headers",
				Pattern:     `^"{resource}" response header "([^"]*)" exists$`,
				Description: "Assert header exists",
				Example:     `"api" response header "X-Request-Id" exists`,
				Handler:     r.responseHeaderShouldExist,
			},

			// Response Body
			{
				Group:       "Response Body",
				Pattern:     `^"{resource}" response body is:$`,
				Description: "Assert exact body match",
				Example:     `"api" response body is:`,
				Handler:     r.responseBodyShouldBe,
			},
			{
				Group:       "Response Body",
				Pattern:     `^"{resource}" response body contains "([^"]*)"$`,
				Description: "Assert body contains substring",
				Example:     `"api" response body contains "success"`,
				Handler:     r.responseBodyShouldContain,
			},
			{
				Group:       "Response Body",
				Pattern:     `^"{resource}" response body does not contain "([^"]*)"$`,
				Description: "Assert body doesn't contain substring",
				Example:     `"api" response body does not contain "error"`,
				Handler:     r.responseBodyShouldNotContain,
			},
			{
				Group:       "Response Body",
				Pattern:     `^"{resource}" response body is empty$`,
				Description: "Assert empty body",
				Example:     `"api" response body is empty`,
				Handler:     r.responseBodyShouldBeEmpty,
			},

			// Response JSON
			{
				Group:       "Response JSON",
				Pattern:     `^"{resource}" response json "([^"]*)" is "([^"]*)"$`,
				Description: "Assert JSON path value",
				Example:     `"api" response json "data.id" is "123"`,
				Handler:     r.responseJSONPathShouldBe,
			},
			{
				Group:       "Response JSON",
				Pattern:     `^"{resource}" response json "([^"]*)" exists$`,
				Description: "Assert JSON path exists",
				Example:     `"api" response json "data.id" exists`,
				Handler:     r.responseJSONPathShouldExist,
			},
			{
				Group:       "Response JSON",
				Pattern:     `^"{resource}" response json "([^"]*)" does not exist$`,
				Description: "Assert JSON path doesn't exist",
				Example:     `"api" response json "data.deleted" does not exist`,
				Handler:     r.responseJSONPathShouldNotExist,
			},
			{
				Group:       "Response JSON",
				Pattern:     `^"{resource}" response json matches:$`,
				Description: "Assert exact JSON structure with matchers: @string, @number, @boolean, @array, @object, @any, @null, @notnull, @empty, @notempty, @regex:pattern, @contains:text, @startswith:text, @endswith:text, @gt:n, @gte:n, @lt:n, @lte:n, @len:n",
				Example:     `"api" response json matches:`,
				Handler:     r.responseJSONShouldMatch,
			},
			{
				Group:       "Response JSON",
				Pattern:     `^"{resource}" response json contains:$`,
				Description: "Assert JSON contains specified fields (ignores extra fields). Supports same matchers as 'matches'",
				Example:     `"api" response json contains:`,
				Handler:     r.responseJSONShouldContain,
			},
			{
				Group:       "Response JSON",
				Pattern:     `^"{resource}" response json "([^"]*)" matches pattern "([^"]*)"$`,
				Description: "Assert JSON path value matches regex pattern",
				Example:     `"api" response json "id" matches pattern "^[0-9a-f-]{36}$"`,
				Handler:     r.responseJSONPathMatchesPattern,
			},
			{
				Group:       "Response JSON",
				Pattern:     `^"{resource}" response json "([^"]*)" is uuid$`,
				Description: "Assert JSON path value is a valid UUID",
				Example:     `"api" response json "id" is uuid`,
				Handler:     r.responseJSONPathIsUUID,
			},
			{
				Group:       "Response JSON",
				Pattern:     `^"{resource}" response json "([^"]*)" is email$`,
				Description: "Assert JSON path value is a valid email format",
				Example:     `"api" response json "email" is email`,
				Handler:     r.responseJSONPathIsEmail,
			},
			{
				Group:       "Response JSON",
				Pattern:     `^"{resource}" response json "([^"]*)" is iso-timestamp$`,
				Description: "Assert JSON path value is an ISO 8601 timestamp",
				Example:     `"api" response json "created_at" is iso-timestamp`,
				Handler:     r.responseJSONPathIsISOTimestamp,
			},

			// Response Timing
			{
				Group:       "Response Timing",
				Pattern:     `^"{resource}" response time is less than "([^"]*)"$`,
				Description: "Assert response time",
				Example:     `"api" response time is less than "500ms"`,
				Handler:     r.responseTimeShouldBeLessThan,
			},

			// Variable Capture
			{
				Group:       "Variable Capture",
				Pattern:     `^"{resource}" response json "([^"]*)" saved as "\{\{([^}]+)\}\}"$`,
				Description: "Save JSON path value to variable for use in subsequent requests",
				Example:     `"api" response json "id" saved as "{{user_id}}"`,
				Handler:     r.saveJSONPathToVariable,
			},
			{
				Group:       "Variable Capture",
				Pattern:     `^"{resource}" response header "([^"]*)" saved as "\{\{([^}]+)\}\}"$`,
				Description: "Save response header value to variable",
				Example:     `"api" response header "Location" saved as "{{location}}"`,
				Handler:     r.saveHeaderToVariable,
			},
		},
	}
}

func (r *HTTPClient) setHeader(key, value string) error {
	r.requestHeaders[key] = value
	return nil
}

func (r *HTTPClient) setHeaders(table *godog.Table) error {
	for _, row := range table.Rows[1:] {
		if len(row.Cells) >= 2 {
			r.requestHeaders[row.Cells[0].Value] = row.Cells[1].Value
		}
	}
	return nil
}

func (r *HTTPClient) setQueryParam(key, value string) error {
	r.requestParams.Set(key, value)
	return nil
}

func (r *HTTPClient) setRequestBody(doc *godog.DocString) error {
	r.requestBody = []byte(doc.Content)
	return nil
}

func (r *HTTPClient) setJSONBody(doc *godog.DocString) error {
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

func (r *HTTPClient) setFormBody(table *godog.Table) error {
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

func (r *HTTPClient) sendRequest(method, path string) error {
	return r.doRequest(method, path, nil)
}

func (r *HTTPClient) sendRequestWithBody(method, path string, doc *godog.DocString) error {
	return r.doRequest(method, path, []byte(doc.Content))
}

func (r *HTTPClient) sendRequestWithJSON(method, path string, doc *godog.DocString) error {
	var js json.RawMessage
	if err := json.Unmarshal([]byte(doc.Content), &js); err != nil {
		return fmt.Errorf("invalid JSON: %w", err)
	}
	if r.requestHeaders["Content-Type"] == "" {
		r.requestHeaders["Content-Type"] = "application/json"
	}
	return r.doRequest(method, path, []byte(doc.Content))
}

func (r *HTTPClient) doRequest(method, path string, body []byte) error {
	// Replace variables in path
	path = ReplaceVariables(path)

	reqURL := r.baseURL + path
	if len(r.requestParams) > 0 {
		reqURL += "?" + r.requestParams.Encode()
	}

	var bodyReader io.Reader
	if body != nil {
		// Replace variables in body
		body = []byte(ReplaceVariables(string(body)))
		bodyReader = bytes.NewReader(body)
	} else if r.requestBody != nil {
		// Replace variables in stored body
		replacedBody := []byte(ReplaceVariables(string(r.requestBody)))
		bodyReader = bytes.NewReader(replacedBody)
	}

	req, err := http.NewRequest(method, reqURL, bodyReader)
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}

	for k, v := range r.requestHeaders {
		// Replace variables in header values
		req.Header.Set(k, ReplaceVariables(v))
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

	// Clear single-use request data, but keep headers persistent within the scenario
	r.requestBody = nil
	r.requestParams = make(url.Values)

	return nil
}

func (r *HTTPClient) responseStatusShouldBe(expected int) error {
	if r.lastResponse == nil {
		return fmt.Errorf("no response received")
	}
	if r.lastResponse.StatusCode != expected {
		return fmt.Errorf("expected status %d, got %d\nBody: %s", expected, r.lastResponse.StatusCode, string(r.lastBody))
	}
	return nil
}

func (r *HTTPClient) responseStatusClassShouldBe(class string) error {
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

func (r *HTTPClient) responseHeaderShouldBe(header, expected string) error {
	if r.lastResponse == nil {
		return fmt.Errorf("no response received")
	}
	actual := r.lastResponse.Header.Get(header)
	if actual != expected {
		return fmt.Errorf("header %q: expected %q, got %q", header, expected, actual)
	}
	return nil
}

func (r *HTTPClient) responseHeaderShouldContain(header, substr string) error {
	if r.lastResponse == nil {
		return fmt.Errorf("no response received")
	}
	actual := r.lastResponse.Header.Get(header)
	if !strings.Contains(actual, substr) {
		return fmt.Errorf("header %q value %q does not contain %q", header, actual, substr)
	}
	return nil
}

func (r *HTTPClient) responseHeaderShouldExist(header string) error {
	if r.lastResponse == nil {
		return fmt.Errorf("no response received")
	}
	if r.lastResponse.Header.Get(header) == "" {
		return fmt.Errorf("header %q does not exist", header)
	}
	return nil
}

func (r *HTTPClient) responseBodyShouldBe(doc *godog.DocString) error {
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

func (r *HTTPClient) responseBodyShouldContain(substr string) error {
	if r.lastResponse == nil {
		return fmt.Errorf("no response received")
	}
	if !strings.Contains(string(r.lastBody), substr) {
		return fmt.Errorf("body does not contain %q\nbody: %s", substr, string(r.lastBody))
	}
	return nil
}

func (r *HTTPClient) responseBodyShouldNotContain(substr string) error {
	if r.lastResponse == nil {
		return fmt.Errorf("no response received")
	}
	if strings.Contains(string(r.lastBody), substr) {
		return fmt.Errorf("body should not contain %q\nbody: %s", substr, string(r.lastBody))
	}
	return nil
}

func (r *HTTPClient) responseBodyShouldBeEmpty() error {
	if r.lastResponse == nil {
		return fmt.Errorf("no response received")
	}
	if len(r.lastBody) > 0 {
		return fmt.Errorf("expected empty body, got: %s", string(r.lastBody))
	}
	return nil
}

func (r *HTTPClient) responseJSONPathShouldBe(path, expected string) error {
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

func (r *HTTPClient) responseJSONPathShouldExist(path string) error {
	if r.lastResponse == nil {
		return fmt.Errorf("no response received")
	}
	_, err := r.getJSONPath(path)
	return err
}

func (r *HTTPClient) responseJSONPathShouldNotExist(path string) error {
	if r.lastResponse == nil {
		return fmt.Errorf("no response received")
	}
	_, err := r.getJSONPath(path)
	if err == nil {
		return fmt.Errorf("JSON path %q exists but should not", path)
	}
	return nil
}

func (r *HTTPClient) responseJSONPathMatchesPattern(path, pattern string) error {
	if r.lastResponse == nil {
		return fmt.Errorf("no response received")
	}
	val, err := r.getJSONPath(path)
	if err != nil {
		return err
	}
	str, ok := val.(string)
	if !ok {
		return fmt.Errorf("JSON path %q is not a string: %T", path, val)
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		return fmt.Errorf("invalid regex pattern: %w", err)
	}
	if !re.MatchString(str) {
		return fmt.Errorf("JSON path %q value %q does not match pattern %q", path, str, pattern)
	}
	return nil
}

func (r *HTTPClient) responseJSONPathIsUUID(path string) error {
	return r.responseJSONPathMatchesPattern(path, `^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)
}

func (r *HTTPClient) responseJSONPathIsEmail(path string) error {
	return r.responseJSONPathMatchesPattern(path, `^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)
}

func (r *HTTPClient) responseJSONPathIsISOTimestamp(path string) error {
	return r.responseJSONPathMatchesPattern(path, `^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(.\d+)?(Z|[+-]\d{2}:\d{2})?$`)
}

func (r *HTTPClient) responseJSONShouldMatch(doc *godog.DocString) error {
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

	return CompareJSON(expected, actual, "", false)
}

func (r *HTTPClient) responseJSONShouldContain(doc *godog.DocString) error {
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

	return CompareJSON(expected, actual, "", true)
}

func (r *HTTPClient) compareJSON(expected, actual interface{}, path string) error {
	return CompareJSON(expected, actual, path, false)
}

// CompareJSON compares expected and actual JSON values.
// If partial is true, extra keys in actual objects are ignored (contains mode).
// If partial is false, objects must match exactly (matches mode).
// Exported for testing.
func CompareJSON(expected, actual interface{}, path string, partial bool) error {
	switch e := expected.(type) {
	case map[string]interface{}:
		a, ok := actual.(map[string]interface{})
		if !ok {
			return fmt.Errorf("at %s: expected object, got %T", path, actual)
		}
		// In non-partial mode, check for extra keys
		if !partial {
			for key := range a {
				if _, exists := e[key]; !exists {
					keyPath := key
					if path != "" {
						keyPath = path + "." + key
					}
					return fmt.Errorf("at %s: unexpected key %q", keyPath, key)
				}
			}
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
			if err := CompareJSON(val, actualVal, newPath, partial); err != nil {
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
			if err := CompareJSON(val, a[i], newPath, partial); err != nil {
				return err
			}
		}
	default:
		if str, ok := expected.(string); ok {
			if strings.HasPrefix(str, "@") {
				return MatchSpecial(str, actual, path)
			}
		}
		if fmt.Sprintf("%v", expected) != fmt.Sprintf("%v", actual) {
			return fmt.Errorf("at %s: expected %v, got %v", path, expected, actual)
		}
	}
	return nil
}

func (r *HTTPClient) matchSpecial(matcher string, actual interface{}, path string) error {
	return MatchSpecial(matcher, actual, path)
}

// MatchSpecial handles special matchers like @string, @regex:pattern, etc.
// Exported for testing.
func MatchSpecial(matcher string, actual interface{}, path string) error {
	// Handle parameterized matchers first
	if strings.HasPrefix(matcher, "@regex:") {
		pattern := strings.TrimPrefix(matcher, "@regex:")
		return matchRegex(pattern, actual, path)
	}
	if strings.HasPrefix(matcher, "@contains:") {
		substr := strings.TrimPrefix(matcher, "@contains:")
		return matchContains(substr, actual, path)
	}
	if strings.HasPrefix(matcher, "@startswith:") {
		prefix := strings.TrimPrefix(matcher, "@startswith:")
		return matchStartsWith(prefix, actual, path)
	}
	if strings.HasPrefix(matcher, "@endswith:") {
		suffix := strings.TrimPrefix(matcher, "@endswith:")
		return matchEndsWith(suffix, actual, path)
	}
	if strings.HasPrefix(matcher, "@gt:") {
		value := strings.TrimPrefix(matcher, "@gt:")
		return matchGreaterThan(value, actual, path)
	}
	if strings.HasPrefix(matcher, "@gte:") {
		value := strings.TrimPrefix(matcher, "@gte:")
		return matchGreaterThanOrEqual(value, actual, path)
	}
	if strings.HasPrefix(matcher, "@lt:") {
		value := strings.TrimPrefix(matcher, "@lt:")
		return matchLessThan(value, actual, path)
	}
	if strings.HasPrefix(matcher, "@lte:") {
		value := strings.TrimPrefix(matcher, "@lte:")
		return matchLessThanOrEqual(value, actual, path)
	}
	if strings.HasPrefix(matcher, "@len:") {
		value := strings.TrimPrefix(matcher, "@len:")
		return matchLength(value, actual, path)
	}

	// Handle simple type matchers
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
	case "@empty":
		return matchEmpty(actual, path)
	case "@notempty":
		return matchNotEmpty(actual, path)
	default:
		return fmt.Errorf("unknown matcher: %s", matcher)
	}
	return nil
}

func matchRegex(pattern string, actual interface{}, path string) error {
	str, ok := actual.(string)
	if !ok {
		return fmt.Errorf("at %s: @regex requires string value, got %T", path, actual)
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		return fmt.Errorf("at %s: invalid regex pattern %q: %w", path, pattern, err)
	}
	if !re.MatchString(str) {
		return fmt.Errorf("at %s: value %q does not match pattern %q", path, str, pattern)
	}
	return nil
}

func matchContains(substr string, actual interface{}, path string) error {
	str, ok := actual.(string)
	if !ok {
		return fmt.Errorf("at %s: @contains requires string value, got %T", path, actual)
	}
	if !strings.Contains(str, substr) {
		return fmt.Errorf("at %s: value %q does not contain %q", path, str, substr)
	}
	return nil
}

func matchStartsWith(prefix string, actual interface{}, path string) error {
	str, ok := actual.(string)
	if !ok {
		return fmt.Errorf("at %s: @startswith requires string value, got %T", path, actual)
	}
	if !strings.HasPrefix(str, prefix) {
		return fmt.Errorf("at %s: value %q does not start with %q", path, str, prefix)
	}
	return nil
}

func matchEndsWith(suffix string, actual interface{}, path string) error {
	str, ok := actual.(string)
	if !ok {
		return fmt.Errorf("at %s: @endswith requires string value, got %T", path, actual)
	}
	if !strings.HasSuffix(str, suffix) {
		return fmt.Errorf("at %s: value %q does not end with %q", path, str, suffix)
	}
	return nil
}

func matchGreaterThan(value string, actual interface{}, path string) error {
	expected, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return fmt.Errorf("at %s: @gt requires numeric value: %w", path, err)
	}
	num, ok := actual.(float64)
	if !ok {
		return fmt.Errorf("at %s: @gt requires numeric actual value, got %T", path, actual)
	}
	if num <= expected {
		return fmt.Errorf("at %s: expected value > %v, got %v", path, expected, num)
	}
	return nil
}

func matchGreaterThanOrEqual(value string, actual interface{}, path string) error {
	expected, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return fmt.Errorf("at %s: @gte requires numeric value: %w", path, err)
	}
	num, ok := actual.(float64)
	if !ok {
		return fmt.Errorf("at %s: @gte requires numeric actual value, got %T", path, actual)
	}
	if num < expected {
		return fmt.Errorf("at %s: expected value >= %v, got %v", path, expected, num)
	}
	return nil
}

func matchLessThan(value string, actual interface{}, path string) error {
	expected, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return fmt.Errorf("at %s: @lt requires numeric value: %w", path, err)
	}
	num, ok := actual.(float64)
	if !ok {
		return fmt.Errorf("at %s: @lt requires numeric actual value, got %T", path, actual)
	}
	if num >= expected {
		return fmt.Errorf("at %s: expected value < %v, got %v", path, expected, num)
	}
	return nil
}

func matchLessThanOrEqual(value string, actual interface{}, path string) error {
	expected, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return fmt.Errorf("at %s: @lte requires numeric value: %w", path, err)
	}
	num, ok := actual.(float64)
	if !ok {
		return fmt.Errorf("at %s: @lte requires numeric actual value, got %T", path, actual)
	}
	if num > expected {
		return fmt.Errorf("at %s: expected value <= %v, got %v", path, expected, num)
	}
	return nil
}

func matchLength(value string, actual interface{}, path string) error {
	expected, err := strconv.Atoi(value)
	if err != nil {
		return fmt.Errorf("at %s: @len requires integer value: %w", path, err)
	}

	var actualLen int
	switch v := actual.(type) {
	case string:
		actualLen = len(v)
	case []interface{}:
		actualLen = len(v)
	case map[string]interface{}:
		actualLen = len(v)
	default:
		return fmt.Errorf("at %s: @len requires string, array, or object, got %T", path, actual)
	}

	if actualLen != expected {
		return fmt.Errorf("at %s: expected length %d, got %d", path, expected, actualLen)
	}
	return nil
}

func matchEmpty(actual interface{}, path string) error {
	switch v := actual.(type) {
	case string:
		if v != "" {
			return fmt.Errorf("at %s: expected empty string, got %q", path, v)
		}
	case []interface{}:
		if len(v) != 0 {
			return fmt.Errorf("at %s: expected empty array, got %d elements", path, len(v))
		}
	case map[string]interface{}:
		if len(v) != 0 {
			return fmt.Errorf("at %s: expected empty object, got %d keys", path, len(v))
		}
	case nil:
		// nil is considered empty
	default:
		return fmt.Errorf("at %s: @empty requires string, array, object, or null, got %T", path, actual)
	}
	return nil
}

func matchNotEmpty(actual interface{}, path string) error {
	switch v := actual.(type) {
	case string:
		if v == "" {
			return fmt.Errorf("at %s: expected non-empty string", path)
		}
	case []interface{}:
		if len(v) == 0 {
			return fmt.Errorf("at %s: expected non-empty array", path)
		}
	case map[string]interface{}:
		if len(v) == 0 {
			return fmt.Errorf("at %s: expected non-empty object", path)
		}
	case nil:
		return fmt.Errorf("at %s: expected non-empty value, got null", path)
	default:
		return fmt.Errorf("at %s: @notempty requires string, array, object, or null, got %T", path, actual)
	}
	return nil
}

func (r *HTTPClient) getJSONPath(path string) (interface{}, error) {
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

func (r *HTTPClient) responseTimeShouldBeLessThan(duration string) error {
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

func (r *HTTPClient) saveJSONPathToVariable(path, varName string) error {
	if r.lastResponse == nil {
		return fmt.Errorf("no response received")
	}

	value, err := r.getJSONPath(path)
	if err != nil {
		return fmt.Errorf("failed to get JSON path %q: %w", path, err)
	}

	// Convert to string
	strValue := fmt.Sprintf("%v", value)
	SetVariable(varName, strValue)
	return nil
}

func (r *HTTPClient) saveHeaderToVariable(header, varName string) error {
	if r.lastResponse == nil {
		return fmt.Errorf("no response received")
	}

	value := r.lastResponse.Header.Get(header)
	if value == "" {
		return fmt.Errorf("header %q not found or empty", header)
	}

	SetVariable(varName, value)
	return nil
}

func (r *HTTPClient) Cleanup(ctx context.Context) error {
	return nil
}

var _ Handler = (*HTTPClient)(nil)
