package handler

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/cucumber/godog"

	"github.com/tomatool/tomato/internal/config"
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

	client, err := NewHTTPClient("api", config.Resource{
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

	client, err := NewHTTPClient("api", config.Resource{
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

	client, err := NewHTTPClient("api", config.Resource{
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
	client, err := NewHTTPClient("api", config.Resource{BaseURL: url}, nil)
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
	SetVariable("session_value", "abc.123")
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
	if got := ReplaceVariables("{{saved}}"); got != "new" {
		t.Errorf("saved variable = %q, want %q", got, "new")
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
