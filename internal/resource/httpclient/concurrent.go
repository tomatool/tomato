package httpclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"

	"github.com/cucumber/godog"
	"github.com/tomatool/tomato/internal/jsonmatch"
	"github.com/tomatool/tomato/internal/resource"
)

// Concurrent requests: the same request sent N times at the same moment, for
// what only happens under concurrency: two identical creates racing, an
// idempotency key used twice at once, a counter incremented in parallel.
// Every request is built and connected first, then all are released together.
// The responses are kept as a set, and the steps assert how many of them
// have a status or a JSON value, since which one wins is not fixed.

type concurrentResponse struct {
	status int
	body   []byte
	err    error
}

func (r *HTTPClient) concurrentSteps() []resource.StepDef {
	return []resource.StepDef{
		{
			Group:       "Concurrent Requests",
			Pattern:     `^"{resource}" sends "(\d+)" concurrent "([^"]*)" to "([^"]*)"$`,
			Description: "Send the same request N times at the same moment, with the scenario's headers, cookies and query params",
			Example:     `"app-client" sends "2" concurrent "POST" to "/orders"`,
			Handler:     r.sendConcurrent,
		},
		{
			Group:       "Concurrent Requests",
			Pattern:     `^"{resource}" sends "(\d+)" concurrent "([^"]*)" to "([^"]*)" with json:$`,
			Description: "Send the same JSON request N times at the same moment",
			Example:     "\"app-client\" sends \"2\" concurrent \"POST\" to \"/orders\" with json:\n  \"\"\"\n  {\"clientOrderId\": \"c-60\"}\n  \"\"\"",
			Handler:     r.sendConcurrentJSON,
		},
		{
			Group:       "Concurrent Requests",
			Pattern:     `^"{resource}" "(\d+)" of the concurrent responses have status "(\d+)"$`,
			Description: "Assert how many of the concurrent responses have this status",
			Example:     `"app-client" "2" of the concurrent responses have status "202"`,
			Handler:     r.concurrentStatusCount,
		},
		{
			Group:       "Concurrent Requests",
			Pattern:     `^"{resource}" "(\d+)" of the concurrent responses have json "([^"]*)" is "([^"]*)"$`,
			Description: "Assert how many of the concurrent responses have this value at a JSON path (a missing path counts as not having it)",
			Example:     `"app-client" "1" of the concurrent responses have json "duplicate" is "true"`,
			Handler:     r.concurrentJSONCount,
		},
		{
			Group:       "Concurrent Requests",
			Pattern:     `^"{resource}" "(\d+)" of the concurrent responses contain json:$`,
			Description: "Assert how many of the concurrent responses contain these JSON fields (same matchers as \"response json contains\")",
			Example:     "\"app-client\" \"1\" of the concurrent responses contain json:\n  \"\"\"\n  {\"duplicate\": true}\n  \"\"\"",
			Handler:     r.concurrentJSONContainsCount,
		},
	}
}

func (r *HTTPClient) sendConcurrent(n int, method, path string) error {
	var body []byte
	if r.requestBody != nil {
		body = r.requestBody
	}
	return r.doConcurrent(n, method, path, body)
}

func (r *HTTPClient) sendConcurrentJSON(n int, method, path string, doc *godog.DocString) error {
	var js json.RawMessage
	if err := json.Unmarshal([]byte(doc.Content), &js); err != nil {
		return fmt.Errorf("invalid JSON: %w", err)
	}
	if r.requestHeaders["Content-Type"] == "" {
		r.requestHeaders["Content-Type"] = "application/json"
	}
	return r.doConcurrent(n, method, path, []byte(doc.Content))
}

func (r *HTTPClient) doConcurrent(n int, method, path string, body []byte) error {
	if n < 2 {
		return fmt.Errorf("concurrent requests need at least 2, got %d", n)
	}
	if body != nil {
		body = []byte(resource.ReplaceVariables(string(body)))
	}

	// Build every request first, so the release is all that is left.
	reqs := make([]*http.Request, n)
	for i := range reqs {
		var reader io.Reader
		if body != nil {
			reader = bytes.NewReader(body)
		}
		req, err := r.newRequest(context.Background(), method, path, reader)
		if err != nil {
			return err
		}
		reqs[i] = req
	}
	r.requestBody = nil
	r.requestParams = make(url.Values)

	// One connection per request, so none waits for another's to be free.
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.MaxConnsPerHost = 0
	transport.MaxIdleConnsPerHost = n
	client := &http.Client{Transport: transport, Timeout: r.client.Timeout, CheckRedirect: r.client.CheckRedirect}
	defer transport.CloseIdleConnections()

	responses := make([]concurrentResponse, n)
	var ready, done sync.WaitGroup
	start := make(chan struct{})
	ready.Add(n)
	done.Add(n)
	for i, req := range reqs {
		go func(i int, req *http.Request) {
			defer done.Done()
			ready.Done()
			<-start
			resp, err := client.Do(req)
			if err != nil {
				responses[i] = concurrentResponse{err: err}
				return
			}
			b, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			responses[i] = concurrentResponse{status: resp.StatusCode, body: b}
		}(i, req)
	}
	ready.Wait()
	close(start)
	done.Wait()

	r.concurrent = responses
	var failed []string
	for _, resp := range responses {
		if resp.err != nil {
			failed = append(failed, resp.err.Error())
		}
	}
	if len(failed) > 0 {
		return fmt.Errorf("%d of %d concurrent requests failed: %s", len(failed), n, strings.Join(failed, "; "))
	}
	return nil
}

func (r *HTTPClient) requireConcurrent() error {
	if len(r.concurrent) == 0 {
		return fmt.Errorf("no concurrent requests sent: use \"%s\" sends \"2\" concurrent ... first", r.name)
	}
	return nil
}

// summary lists the responses for a failure message.
func (r *HTTPClient) concurrentSummary() string {
	lines := make([]string, len(r.concurrent))
	for i, resp := range r.concurrent {
		lines[i] = fmt.Sprintf("  %d: %d %s", i+1, resp.status, strings.TrimSpace(string(resp.body)))
	}
	sort.Strings(lines)
	return strings.Join(lines, "\n")
}

func (r *HTTPClient) countConcurrent(want int, what string, match func(concurrentResponse) bool) error {
	if err := r.requireConcurrent(); err != nil {
		return err
	}
	got := 0
	for _, resp := range r.concurrent {
		if match(resp) {
			got++
		}
	}
	if got != want {
		return fmt.Errorf("expected %d of %d concurrent responses %s, got %d:\n%s", want, len(r.concurrent), what, got, r.concurrentSummary())
	}
	return nil
}

func (r *HTTPClient) concurrentStatusCount(want, status int) error {
	return r.countConcurrent(want, fmt.Sprintf("with status %d", status), func(resp concurrentResponse) bool {
		return resp.status == status
	})
}

func (r *HTTPClient) concurrentJSONCount(want int, path, value string) error {
	value = resource.ReplaceVariables(value)
	return r.countConcurrent(want, fmt.Sprintf("with %s = %q", path, value), func(resp concurrentResponse) bool {
		v, err := jsonmatch.Path(resp.body, path)
		return err == nil && fmt.Sprintf("%v", v) == value
	})
}

func (r *HTTPClient) concurrentJSONContainsCount(want int, doc *godog.DocString) error {
	var expected interface{}
	if err := json.Unmarshal([]byte(resource.ReplaceVariables(doc.Content)), &expected); err != nil {
		return fmt.Errorf("invalid expected JSON: %w", err)
	}
	return r.countConcurrent(want, "containing the JSON", func(resp concurrentResponse) bool {
		var actual interface{}
		if json.Unmarshal(resp.body, &actual) != nil {
			return false
		}
		return jsonmatch.Compare(expected, actual, "", true) == nil
	})
}
