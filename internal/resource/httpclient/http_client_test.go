package httpclient

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/cucumber/godog"

	"github.com/tomatool/tomato/internal/config"
	"github.com/tomatool/tomato/internal/resource"
)

func TestHTTPClient_HeadersPersistBetweenRequests(t *testing.T) {
	requestCount := 0
	var receivedAuth []string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		receivedAuth = append(receivedAuth, r.Header.Get("Authorization"))
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client, err := New("api", config.Resource{
		BaseURL: server.URL,
	}, nil)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	if err := client.Init(context.Background()); err != nil {
		t.Fatalf("failed to init client: %v", err)
	}

	// Set header once
	client.setHeader("Authorization", "Bearer test-token")

	// First request
	if err := client.sendRequest("GET", "/first"); err != nil {
		t.Fatalf("first request failed: %v", err)
	}

	// Second request - header should still be present
	if err := client.sendRequest("GET", "/second"); err != nil {
		t.Fatalf("second request failed: %v", err)
	}

	// Third request - header should still be present
	if err := client.sendRequest("DELETE", "/third"); err != nil {
		t.Fatalf("third request failed: %v", err)
	}

	if requestCount != 3 {
		t.Errorf("expected 3 requests, got %d", requestCount)
	}

	for i, auth := range receivedAuth {
		if auth != "Bearer test-token" {
			t.Errorf("request %d: expected 'Bearer test-token', got %q", i+1, auth)
		}
	}
}

func TestHTTPClient_BodyClearedBetweenRequests(t *testing.T) {
	var receivedBodies []string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		receivedBodies = append(receivedBodies, string(body))
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client, err := New("api", config.Resource{
		BaseURL: server.URL,
	}, nil)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	if err := client.Init(context.Background()); err != nil {
		t.Fatalf("failed to init client: %v", err)
	}

	// Set body and send first request
	client.requestBody = []byte(`{"name": "test"}`)
	if err := client.sendRequest("POST", "/first"); err != nil {
		t.Fatalf("first request failed: %v", err)
	}

	// Second request without setting body - should have empty body
	if err := client.sendRequest("POST", "/second"); err != nil {
		t.Fatalf("second request failed: %v", err)
	}

	if len(receivedBodies) != 2 {
		t.Fatalf("expected 2 requests, got %d", len(receivedBodies))
	}

	if receivedBodies[0] != `{"name": "test"}` {
		t.Errorf("first request body: expected '{\"name\": \"test\"}', got %q", receivedBodies[0])
	}

	if receivedBodies[1] != "" {
		t.Errorf("second request body: expected empty, got %q", receivedBodies[1])
	}
}

func TestHTTPClient_ResetClearsHeaders(t *testing.T) {
	requestCount := 0
	var receivedAuth []string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		receivedAuth = append(receivedAuth, r.Header.Get("Authorization"))
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client, err := New("api", config.Resource{
		BaseURL: server.URL,
	}, nil)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	if err := client.Init(context.Background()); err != nil {
		t.Fatalf("failed to init client: %v", err)
	}

	// Set header and make request
	client.setHeader("Authorization", "Bearer test-token")
	if err := client.sendRequest("GET", "/first"); err != nil {
		t.Fatalf("first request failed: %v", err)
	}

	// Reset (simulates new scenario)
	if err := client.Reset(context.Background()); err != nil {
		t.Fatalf("reset failed: %v", err)
	}

	// Request after reset - header should be gone
	if err := client.sendRequest("GET", "/second"); err != nil {
		t.Fatalf("second request failed: %v", err)
	}

	if requestCount != 2 {
		t.Errorf("expected 2 requests, got %d", requestCount)
	}

	if receivedAuth[0] != "Bearer test-token" {
		t.Errorf("first request: expected 'Bearer test-token', got %q", receivedAuth[0])
	}

	if receivedAuth[1] != "" {
		t.Errorf("second request (after reset): expected empty, got %q", receivedAuth[1])
	}
}

func newTestHTTPClient(t *testing.T, url string) *HTTPClient {
	t.Helper()
	client, err := New("api", config.Resource{BaseURL: url}, nil)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}
	if err := client.Init(context.Background()); err != nil {
		t.Fatalf("failed to init client: %v", err)
	}
	return client
}

func TestHTTPClient_HostHeaderSetsRequestHost(t *testing.T) {
	var gotHost string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotHost = r.Host
	}))
	defer server.Close()

	client := newTestHTTPClient(t, server.URL)
	client.setHeader("Host", "go.example.com")
	if err := client.sendRequest("GET", "/"); err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if gotHost != "go.example.com" {
		t.Errorf("server saw Host %q, want %q", gotHost, "go.example.com")
	}
}

func TestHTTPClient_CookiesPersistAndUseVariables(t *testing.T) {
	var got []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := r.Cookie("session")
		if err != nil {
			got = append(got, "<none>")
			return
		}
		got = append(got, c.Value)
	}))
	defer server.Close()

	client := newTestHTTPClient(t, server.URL)
	resource.SetVariable("session_value", "abc.123")
	client.setCookie("session", "{{session_value}}")
	client.sendRequest("GET", "/one")
	client.sendRequest("GET", "/two")
	client.Reset(context.Background())
	client.sendRequest("GET", "/three")

	want := []string{"abc.123", "abc.123", "<none>"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("cookies seen by server = %v, want %v", got, want)
	}
}

func TestHTTPClient_ResponseCookies(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Same cookie set twice: the last one should win.
		http.SetCookie(w, &http.Cookie{Name: "session", Value: "old"})
		http.SetCookie(w, &http.Cookie{Name: "state", Value: "", MaxAge: -1})
		http.SetCookie(w, &http.Cookie{Name: "session", Value: "new", HttpOnly: true})
	}))
	defer server.Close()

	client := newTestHTTPClient(t, server.URL)
	if err := client.sendRequest("GET", "/"); err != nil {
		t.Fatalf("request failed: %v", err)
	}

	if err := client.responseCookieShouldExist("session"); err != nil {
		t.Error(err)
	}
	if err := client.responseCookieShouldExist("missing"); err == nil {
		t.Error("expected error for a cookie that was not set")
	}
	if err := client.responseCookieShouldBe("session", "new"); err != nil {
		t.Error(err)
	}
	if err := client.responseCookieShouldBe("state", ""); err != nil {
		t.Error(err)
	}
	if err := client.responseCookieShouldBe("session", "old"); err == nil {
		t.Error("expected mismatch error")
	}
	if err := client.saveCookieToVariable("session", "saved"); err != nil {
		t.Fatal(err)
	}
	if got := resource.ReplaceVariables("{{saved}}"); got != "new" {
		t.Errorf("saved variable = %q, want %q", got, "new")
	}
}

func TestHTTPClient_SaveHeaderQueryParam(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 201 with a Location, so the client doesn't follow it
		w.Header().Set("Location", r.URL.Query().Get("location"))
		w.WriteHeader(http.StatusCreated)
	}))
	defer server.Close()

	client := newTestHTTPClient(t, server.URL)
	send := func(location string) {
		t.Helper()
		if err := client.sendRequest("GET", "/?location="+url.QueryEscape(location)); err != nil {
			t.Fatalf("request failed: %v", err)
		}
	}

	send("https://app.example.com/callback?code=abc%2F123&state=xyz")
	if err := client.saveHeaderQueryParamToVariable("Location", "code", "code"); err != nil {
		t.Fatal(err)
	}
	if got := resource.ReplaceVariables("{{code}}"); got != "abc/123" {
		t.Errorf("saved code = %q, want the decoded %q", got, "abc/123")
	}

	send("/echo?session=sess-42")
	if err := client.saveHeaderQueryParamToVariable("Location", "session", "session"); err != nil {
		t.Fatalf("relative URL: %v", err)
	}
	if got := resource.ReplaceVariables("{{session}}"); got != "sess-42" {
		t.Errorf("saved session = %q, want %q", got, "sess-42")
	}

	send("https://app.example.com/callback?state=xyz&error=denied")
	err := client.saveHeaderQueryParamToVariable("Location", "code", "code")
	if err == nil || !strings.Contains(err.Error(), "has: error, state") {
		t.Errorf("missing param error = %v, want it to list the params the URL has", err)
	}

	send("https://app.example.com/callback?code=&state=xyz")
	if err := client.saveHeaderQueryParamToVariable("Location", "code", "code"); err == nil {
		t.Error("expected error for an empty param")
	}

	send("https://app.example.com/callback?code=a&code=b")
	if err := client.saveHeaderQueryParamToVariable("Location", "code", "code"); err == nil {
		t.Error("expected error for a repeated param")
	}

	if err := client.saveHeaderQueryParamToVariable("X-Missing", "code", "code"); err == nil {
		t.Error("expected error for a missing header")
	}
}

func TestHTTPClient_ResponseBodyContainsDocString(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `requests_total{handler="GET /",code="200"} 3`)
	}))
	defer server.Close()

	client := newTestHTTPClient(t, server.URL)
	client.sendRequest("GET", "/metrics")

	if err := client.responseBodyShouldContainDoc(&godog.DocString{Content: "\n  requests_total{handler=\"GET /\",code=\"200\"} 3\n"}); err != nil {
		t.Error(err)
	}
	if err := client.responseBodyShouldContainDoc(&godog.DocString{Content: `code="500"`}); err == nil {
		t.Error("expected error for missing text")
	}
	if err := client.responseBodyShouldNotContainDoc(&godog.DocString{Content: `code="500"`}); err != nil {
		t.Error(err)
	}
	if err := client.responseBodyShouldNotContainDoc(&godog.DocString{Content: `code="200"`}); err == nil {
		t.Error("expected error for present text")
	}
}

func TestResolveRequestURL(t *testing.T) {
	cases := []struct{ base, path, want string }{
		{"http://localhost:8080", "/users", "http://localhost:8080/users"},
		{"http://localhost:8080", "http://localhost:9999/health", "http://localhost:9999/health"},
		{"http://localhost:8080", "https://example.com/x?a=1", "https://example.com/x?a=1"},
		{"", "/users", "/users"},
	}
	for _, c := range cases {
		if got := resolveRequestURL(c.base, c.path); got != c.want {
			t.Errorf("resolveRequestURL(%q, %q) = %q, want %q", c.base, c.path, got, c.want)
		}
	}
}

// Every response assertion compares against the value a saved variable holds,
// not the literal "{{name}}" text, as request steps already do.
func TestHTTPClient_AssertionsUseVariables(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Order", "o-7")
		io.WriteString(w, `{"id":"o-7","receivedAt":"2026-10-13T09:31:03Z"}`)
	}))
	defer server.Close()

	client, err := New("api", config.Resource{BaseURL: server.URL}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := client.Init(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := client.sendRequest("GET", "/"); err != nil {
		t.Fatal(err)
	}

	resource.SetVariable("order_id", "o-7")
	resource.SetVariable("received_at", "2026-10-13T09:31:03Z")
	resource.SetVariable("other", "o-8")
	doc := func(s string) *godog.DocString { return &godog.DocString{Content: s} }

	pass := map[string]func() error{
		"response json is":         func() error { return client.responseJSONPathShouldBe("receivedAt", "{{received_at}}") },
		"response header is":       func() error { return client.responseHeaderShouldBe("X-Order", "{{order_id}}") },
		"response header contains": func() error { return client.responseHeaderShouldContain("X-Order", "{{order_id}}") },
		"response body is": func() error {
			return client.responseBodyShouldBe(doc(`{"id":"{{order_id}}","receivedAt":"{{received_at}}"}`))
		},
		"response body contains":     func() error { return client.responseBodyShouldContain("{{order_id}}") },
		"response body contains doc": func() error { return client.responseBodyShouldContainDoc(doc(`"id":"{{order_id}}"`)) },
		"response body not contain":  func() error { return client.responseBodyShouldNotContain("{{other}}") },
		"response json matches": func() error {
			return client.responseJSONShouldMatch(doc(`{"id":"{{order_id}}","receivedAt":"@string"}`))
		},
		"response json contains": func() error { return client.responseJSONShouldContain(doc(`{"id":"{{order_id}}"}`)) },
	}
	for name, step := range pass {
		if err := step(); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}

	// And a different saved value fails, so the variable is what was compared.
	fail := map[string]func() error{
		"response json is":          func() error { return client.responseJSONPathShouldBe("id", "{{other}}") },
		"response header is":        func() error { return client.responseHeaderShouldBe("X-Order", "{{other}}") },
		"response body contains":    func() error { return client.responseBodyShouldContain("{{other}}") },
		"response body not contain": func() error { return client.responseBodyShouldNotContain("{{order_id}}") },
		"response json contains":    func() error { return client.responseJSONShouldContain(doc(`{"id":"{{other}}"}`)) },
	}
	for name, step := range fail {
		if err := step(); err == nil {
			t.Errorf("%s: should fail against a different saved value", name)
		}
	}

	// An unknown variable stays literal, so suites without variables are unaffected.
	if err := client.responseBodyShouldNotContain("{{never_saved}}"); err != nil {
		t.Errorf("unknown variable: %v", err)
	}
}

func TestHTTPClient_FormBodySubstitutesVariables(t *testing.T) {
	var got url.Values
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.ParseForm()
		got = r.PostForm
	}))
	defer server.Close()

	resource.SetVariable("form_code", "abc&123")
	client := newTestHTTPClient(t, server.URL)
	if err := client.setFormBody(eventsTable(
		[]string{"field", "value"},
		[]string{"code", "{{form_code}}"},
		[]string{"{{form_code}}", "as a field name"},
	)); err != nil {
		t.Fatal(err)
	}
	if err := client.sendRequest("POST", "/token"); err != nil {
		t.Fatalf("request failed: %v", err)
	}

	// Substituted, then encoded: the & in the value doesn't split the field
	if v := got.Get("code"); v != "abc&123" {
		t.Errorf("code = %q, want %q", v, "abc&123")
	}
	if v := got.Get("abc&123"); v != "as a field name" {
		t.Errorf("substituted field name = %q, want %q", v, "as a field name")
	}
}
