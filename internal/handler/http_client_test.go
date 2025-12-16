package handler

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

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
