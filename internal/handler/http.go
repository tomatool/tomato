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
	ctx.Step(fmt.Sprintf(`^I set "%s" header "([^"]*)" to "([^"]*)"$`, r.name), r.setHeader)
	ctx.Step(fmt.Sprintf(`^I set "%s" headers:$`, r.name), r.setHeaders)
	ctx.Step(fmt.Sprintf(`^I set "%s" query param "([^"]*)" to "([^"]*)"$`, r.name), r.setQueryParam)
	ctx.Step(fmt.Sprintf(`^I set "%s" request body:$`, r.name), r.setRequestBody)
	ctx.Step(fmt.Sprintf(`^I set "%s" JSON body:$`, r.name), r.setJSONBody)
	ctx.Step(fmt.Sprintf(`^I set "%s" form body:$`, r.name), r.setFormBody)

	ctx.Step(fmt.Sprintf(`^I send "([^"]*)" request to "%s" "([^"]*)"$`, r.name), r.sendRequest)
	ctx.Step(fmt.Sprintf(`^I send "([^"]*)" request to "%s" "([^"]*)" with body:$`, r.name), r.sendRequestWithBody)
	ctx.Step(fmt.Sprintf(`^I send "([^"]*)" request to "%s" "([^"]*)" with JSON:$`, r.name), r.sendRequestWithJSON)

	ctx.Step(fmt.Sprintf(`^"%s" response status should be "(\d+)"$`, r.name), r.responseStatusShouldBe)
	ctx.Step(fmt.Sprintf(`^"%s" response status should be (success|redirect|client error|server error)$`, r.name), r.responseStatusClassShouldBe)
	ctx.Step(fmt.Sprintf(`^"%s" response header "([^"]*)" should be "([^"]*)"$`, r.name), r.responseHeaderShouldBe)
	ctx.Step(fmt.Sprintf(`^"%s" response header "([^"]*)" should contain "([^"]*)"$`, r.name), r.responseHeaderShouldContain)
	ctx.Step(fmt.Sprintf(`^"%s" response header "([^"]*)" should exist$`, r.name), r.responseHeaderShouldExist)
	ctx.Step(fmt.Sprintf(`^"%s" response body should be:$`, r.name), r.responseBodyShouldBe)
	ctx.Step(fmt.Sprintf(`^"%s" response body should contain "([^"]*)"$`, r.name), r.responseBodyShouldContain)
	ctx.Step(fmt.Sprintf(`^"%s" response body should not contain "([^"]*)"$`, r.name), r.responseBodyShouldNotContain)
	ctx.Step(fmt.Sprintf(`^"%s" response body should be empty$`, r.name), r.responseBodyShouldBeEmpty)

	ctx.Step(fmt.Sprintf(`^"%s" response JSON "([^"]*)" should be "([^"]*)"$`, r.name), r.responseJSONPathShouldBe)
	ctx.Step(fmt.Sprintf(`^"%s" response JSON "([^"]*)" should exist$`, r.name), r.responseJSONPathShouldExist)
	ctx.Step(fmt.Sprintf(`^"%s" response JSON "([^"]*)" should not exist$`, r.name), r.responseJSONPathShouldNotExist)
	ctx.Step(fmt.Sprintf(`^"%s" response JSON should match:$`, r.name), r.responseJSONShouldMatch)

	ctx.Step(fmt.Sprintf(`^"%s" response time should be less than "([^"]*)"$`, r.name), r.responseTimeShouldBeLessThan)
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
