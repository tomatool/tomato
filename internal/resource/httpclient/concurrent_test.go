package httpclient

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cucumber/godog"

	"github.com/tomatool/tomato/internal/config"
	"github.com/tomatool/tomato/internal/resource"
)

// idempotentServer creates an order once per clientOrderId: the first request
// gets created, any other gets duplicate. It records how many requests were in
// flight at once.
type idempotentServer struct {
	*httptest.Server
	mu       sync.Mutex
	seen     map[string]bool
	inFlight int32
	maxIn    int32
	headers  []string
}

func newIdempotentServer(t *testing.T) *idempotentServer {
	s := &idempotentServer{seen: map[string]bool{}}
	s.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&s.inFlight, 1)
		for {
			m := atomic.LoadInt32(&s.maxIn)
			if n <= m || atomic.CompareAndSwapInt32(&s.maxIn, m, n) {
				break
			}
		}
		time.Sleep(100 * time.Millisecond) // long enough for the others to arrive
		atomic.AddInt32(&s.inFlight, -1)

		body, _ := io.ReadAll(r.Body)
		s.mu.Lock()
		s.headers = append(s.headers, r.Header.Get("Authorization")+"|"+r.URL.Query().Get("q")+"|"+r.Header.Get("Content-Type"))
		key := string(body)
		dup := s.seen[key]
		s.seen[key] = true
		s.mu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		if dup {
			fmt.Fprint(w, `{"clientOrderId":"c-60","duplicate":true}`)
		} else {
			fmt.Fprint(w, `{"clientOrderId":"c-60"}`)
		}
	}))
	t.Cleanup(s.Close)
	return s
}

func newConcurrentClient(t *testing.T, url string) *HTTPClient {
	t.Helper()
	c, err := New("api", config.Resource{BaseURL: url}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := c.Init(context.Background()); err != nil {
		t.Fatal(err)
	}
	return c
}

func TestConcurrent_SendsAtTheSameMoment(t *testing.T) {
	srv := newIdempotentServer(t)
	c := newConcurrentClient(t, srv.URL)
	c.setHeader("Authorization", "Bearer t")
	c.setQueryParam("q", "1")

	resource.SetVariable("coid", "c-60")
	if err := c.sendConcurrentJSON(5, "POST", "/orders", &godog.DocString{Content: `{"clientOrderId":"{{coid}}"}`}); err != nil {
		t.Fatal(err)
	}
	if got := atomic.LoadInt32(&srv.maxIn); got != 5 {
		t.Errorf("max requests in flight at once: %d, want 5", got)
	}
	for _, h := range srv.headers {
		if h != "Bearer t|1|application/json" {
			t.Errorf("request sent without the scenario's setup: %q", h)
		}
	}
	if len(c.requestParams) != 0 {
		t.Error("query params are single-use")
	}

	if err := c.concurrentStatusCount(5, 202); err != nil {
		t.Error(err)
	}
	if err := c.concurrentJSONCount(4, "duplicate", "true"); err != nil {
		t.Error(err)
	}
	if err := c.concurrentJSONCount(5, "clientOrderId", "{{coid}}"); err != nil {
		t.Error(err)
	}
	if err := c.concurrentJSONContainsCount(1, &godog.DocString{Content: `{"clientOrderId":"c-60"}`}); err == nil {
		t.Error("contains matches all five, not one")
	}
	if err := c.concurrentJSONContainsCount(4, &godog.DocString{Content: `{"duplicate":true}`}); err != nil {
		t.Error(err)
	}

	// A wrong count fails and lists every response.
	err := c.concurrentStatusCount(1, 202)
	if err == nil || !strings.Contains(err.Error(), "expected 1 of 5") || strings.Count(err.Error(), "202 {") != 5 {
		t.Errorf("want a count error listing the responses, got %v", err)
	}
}

func TestConcurrent_RawBodyAndNoBody(t *testing.T) {
	srv := newIdempotentServer(t)
	c := newConcurrentClient(t, srv.URL)
	c.setRequestBody(&godog.DocString{Content: "same"})
	if err := c.sendConcurrent(2, "POST", "/x"); err != nil {
		t.Fatal(err)
	}
	if err := c.concurrentJSONCount(1, "duplicate", "true"); err != nil {
		t.Error(err)
	}
	if c.requestBody != nil {
		t.Error("the stored body is single-use")
	}
	if err := c.sendConcurrent(3, "GET", "/x"); err != nil {
		t.Fatal(err)
	}
}

func TestConcurrent_Errors(t *testing.T) {
	srv := newIdempotentServer(t)
	c := newConcurrentClient(t, srv.URL)

	if err := c.sendConcurrent(1, "GET", "/"); err == nil || !strings.Contains(err.Error(), "at least 2") {
		t.Errorf("n=1: %v", err)
	}
	if err := c.sendConcurrentJSON(2, "POST", "/", &godog.DocString{Content: "{bad"}); err == nil {
		t.Error("invalid JSON")
	}
	for name, step := range map[string]func() error{
		"status":   func() error { return c.concurrentStatusCount(1, 200) },
		"json":     func() error { return c.concurrentJSONCount(1, "a", "b") },
		"contains": func() error { return c.concurrentJSONContainsCount(1, &godog.DocString{Content: "{}"}) },
	} {
		if err := step(); err == nil || !strings.Contains(err.Error(), "no concurrent requests") {
			t.Errorf("%s before sending: %v", name, err)
		}
	}
	if err := c.sendConcurrent(2, "GET", "/bad\x7fpath"); err == nil {
		t.Error("a request that cannot be built")
	}

	c.sendConcurrent(2, "GET", "/")
	if err := c.concurrentJSONContainsCount(1, &godog.DocString{Content: "{bad"}); err == nil {
		t.Error("invalid expected JSON")
	}

	// Requests that fail to connect fail the step, and say how many.
	dead := newConcurrentClient(t, "http://127.0.0.1:1")
	if err := dead.sendConcurrent(3, "GET", "/"); err == nil || !strings.Contains(err.Error(), "3 of 3 concurrent requests failed") {
		t.Errorf("connection refused: %v", err)
	}

	// Reset forgets the responses.
	c.Reset(context.Background())
	if err := c.concurrentStatusCount(2, 202); err == nil {
		t.Error("responses survived a reset")
	}
}

// A response that is not JSON simply does not match a JSON assertion.
func TestConcurrent_NonJSONResponses(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "plain")
	}))
	defer srv.Close()
	c := newConcurrentClient(t, srv.URL)
	if err := c.sendConcurrent(2, "GET", "/"); err != nil {
		t.Fatal(err)
	}
	if err := c.concurrentJSONCount(0, "a", "b"); err != nil {
		t.Error(err)
	}
	if err := c.concurrentJSONContainsCount(0, &godog.DocString{Content: "{}"}); err != nil {
		t.Error(err)
	}
}
