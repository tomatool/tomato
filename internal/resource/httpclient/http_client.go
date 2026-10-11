// Package httpclient provides tomato's `http` resource: making HTTP requests and validating
// responses.
package httpclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/cucumber/godog"
	"github.com/tomatool/tomato/internal/config"
	"github.com/tomatool/tomato/internal/container"
	"github.com/tomatool/tomato/internal/jsonmatch"
	"github.com/tomatool/tomato/internal/resource"
)

type HTTPClient struct {
	name      string
	config    config.Resource
	container *container.Manager
	client    *http.Client
	baseURL   string

	requestHeaders map[string]string
	requestCookies map[string]string
	requestBody    []byte
	requestParams  url.Values

	lastResponse *http.Response
	lastBody     []byte
}

func New(name string, cfg config.Resource, cm *container.Manager) (*HTTPClient, error) {
	return &HTTPClient{
		name:           name,
		config:         cfg,
		container:      cm,
		requestHeaders: make(map[string]string),
		requestCookies: make(map[string]string),
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
	r.requestCookies = make(map[string]string)
	r.requestBody = nil
	r.requestParams = make(url.Values)
	r.lastResponse = nil
	r.lastBody = nil
	return nil
}

func (r *HTTPClient) RegisterSteps(ctx *godog.ScenarioContext) {
	resource.RegisterStepsToGodog(ctx, r.name, r.Steps())
}

// Steps returns the structured step definitions for the HTTP handler
func (r *HTTPClient) Steps() resource.StepCategory {
	return resource.StepCategory{
		Name:        "HTTP Client",
		Description: "Steps for making HTTP requests and validating responses",
		Steps: []resource.StepDef{
			// Request Setup
			{
				Group:       "Request Setup",
				Pattern:     `^"{resource}" header "([^"]*)" is "([^"]*)"$`,
				Description: "Set a header (a \"Host\" header sets the request host)",
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
				Pattern:     `^"{resource}" cookie "([^"]*)" is "([^"]*)"$`,
				Description: "Set a request cookie (kept for the rest of the scenario, like headers)",
				Example:     `"api" cookie "session" is "{{session}}"`,
				Handler:     r.setCookie,
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
				Pattern:     `^"{resource}" response body contains:$`,
				Description: "Assert body contains the docstring text (use for text with quotes)",
				Example:     `"api" response body contains:`,
				Handler:     r.responseBodyShouldContainDoc,
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
				Pattern:     `^"{resource}" response body does not contain:$`,
				Description: "Assert body doesn't contain the docstring text",
				Example:     `"api" response body does not contain:`,
				Handler:     r.responseBodyShouldNotContainDoc,
			},
			{
				Group:       "Response Body",
				Pattern:     `^"{resource}" response body is empty$`,
				Description: "Assert empty body",
				Example:     `"api" response body is empty`,
				Handler:     r.responseBodyShouldBeEmpty,
			},

			// Response Cookies
			{
				Group:       "Response Cookies",
				Pattern:     `^"{resource}" response cookie "([^"]*)" exists$`,
				Description: "Assert the response sets a cookie",
				Example:     `"api" response cookie "session" exists`,
				Handler:     r.responseCookieShouldExist,
			},
			{
				Group:       "Response Cookies",
				Pattern:     `^"{resource}" response cookie "([^"]*)" is "([^"]*)"$`,
				Description: "Assert a cookie's value (an empty value means the cookie is cleared)",
				Example:     `"api" response cookie "session" is ""`,
				Handler:     r.responseCookieShouldBe,
			},
			{
				Group:       "Response Cookies",
				Pattern:     `^"{resource}" response cookie "([^"]*)" saved as "\{\{([^}]+)\}\}"$`,
				Description: "Save a cookie's value to a variable",
				Example:     `"api" response cookie "session" saved as "{{session}}"`,
				Handler:     r.saveCookieToVariable,
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

func (r *HTTPClient) setCookie(name, value string) error {
	r.requestCookies[name] = value
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
	path = resource.ReplaceVariables(path)

	reqURL := resolveRequestURL(r.baseURL, path)
	if len(r.requestParams) > 0 {
		sep := "?"
		if strings.Contains(reqURL, "?") {
			sep = "&"
		}
		reqURL += sep + r.requestParams.Encode()
	}

	var bodyReader io.Reader
	if body != nil {
		// Replace variables in body
		body = []byte(resource.ReplaceVariables(string(body)))
		bodyReader = bytes.NewReader(body)
	} else if r.requestBody != nil {
		// Replace variables in stored body
		replacedBody := []byte(resource.ReplaceVariables(string(r.requestBody)))
		bodyReader = bytes.NewReader(replacedBody)
	}

	req, err := http.NewRequest(method, reqURL, bodyReader)
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}

	for k, v := range r.requestHeaders {
		// Replace variables in header values
		v = resource.ReplaceVariables(v)
		// net/http ignores a Host header; the request's Host field is what gets sent.
		if strings.EqualFold(k, "Host") {
			req.Host = v
			continue
		}
		req.Header.Set(k, v)
	}
	for name, value := range r.requestCookies {
		req.AddCookie(&http.Cookie{Name: name, Value: resource.ReplaceVariables(value)})
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

func (r *HTTPClient) responseBodyShouldContainDoc(doc *godog.DocString) error {
	return r.responseBodyShouldContain(resource.ReplaceVariables(strings.TrimSpace(doc.Content)))
}

func (r *HTTPClient) responseBodyShouldNotContainDoc(doc *godog.DocString) error {
	return r.responseBodyShouldNotContain(resource.ReplaceVariables(strings.TrimSpace(doc.Content)))
}

// responseCookie returns the named cookie from the response's Set-Cookie
// headers. When set more than once, the last one wins, as in a browser.
func (r *HTTPClient) responseCookie(name string) (*http.Cookie, error) {
	if r.lastResponse == nil {
		return nil, fmt.Errorf("no response received")
	}
	var found *http.Cookie
	for _, c := range r.lastResponse.Cookies() {
		if c.Name == name {
			found = c
		}
	}
	if found == nil {
		return nil, fmt.Errorf("response does not set cookie %q", name)
	}
	return found, nil
}

func (r *HTTPClient) responseCookieShouldExist(name string) error {
	_, err := r.responseCookie(name)
	return err
}

func (r *HTTPClient) responseCookieShouldBe(name, expected string) error {
	c, err := r.responseCookie(name)
	if err != nil {
		return err
	}
	if expected = resource.ReplaceVariables(expected); c.Value != expected {
		return fmt.Errorf("cookie %q: expected %q, got %q", name, expected, c.Value)
	}
	return nil
}

func (r *HTTPClient) saveCookieToVariable(name, varName string) error {
	c, err := r.responseCookie(name)
	if err != nil {
		return err
	}
	resource.SetVariable(varName, c.Value)
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

	return jsonmatch.Compare(expected, actual, "", false)
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

	return jsonmatch.Compare(expected, actual, "", true)
}

func (r *HTTPClient) compareJSON(expected, actual interface{}, path string) error {
	return jsonmatch.Compare(expected, actual, path, false)
}

func (r *HTTPClient) matchSpecial(matcher string, actual interface{}, path string) error {
	return jsonmatch.Special(matcher, actual, path)
}

// getJSONPath reads a dotted path out of the last response body. The
// traversal itself is shared with the other resources that assert on JSON —
// see jsonPathValue.
func (r *HTTPClient) getJSONPath(path string) (interface{}, error) {
	return jsonmatch.Path(r.lastBody, path)
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
	resource.SetVariable(varName, strValue)
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

	resource.SetVariable(varName, value)
	return nil
}

func (r *HTTPClient) Cleanup(ctx context.Context) error {
	return nil
}

var _ resource.Handler = (*HTTPClient)(nil)

// resolveRequestURL joins a step's path onto base_url. A path that is already
// an absolute URL (http://localhost:9999/health, e.g. an http-server stub)
// is used as is.
func resolveRequestURL(baseURL, path string) string {
	if strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") {
		return path
	}
	return baseURL + path
}
