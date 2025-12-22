package handler

import (
	"context"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/tomatool/tomato/internal/config"
)

func TestLoadFixtures(t *testing.T) {
	// Create temp directory for fixtures
	tmpDir := t.TempDir()

	// Create stubs.yml
	stubsYAML := `stubs:
  - id: test-stub
    method: GET
    path: /api/test
    response:
      status: 200
      body: '{"test": "value"}'

  - id: test-with-file
    method: GET
    path: /api/file
    response:
      status: 200
      bodyFile: responses/test.json
`

	if err := os.WriteFile(filepath.Join(tmpDir, "stubs.yml"), []byte(stubsYAML), 0644); err != nil {
		t.Fatal(err)
	}

	// Create responses directory and file
	responsesDir := filepath.Join(tmpDir, "responses")
	if err := os.Mkdir(responsesDir, 0755); err != nil {
		t.Fatal(err)
	}

	responseJSON := `{"from": "file", "value": 123}`
	if err := os.WriteFile(filepath.Join(responsesDir, "test.json"), []byte(responseJSON), 0644); err != nil {
		t.Fatal(err)
	}

	// Create HTTP server handler
	srv, err := NewHTTPServer("test", config.Resource{}, nil)
	if err != nil {
		t.Fatal(err)
	}

	// Initialize server
	ctx := context.Background()
	if err := srv.Init(ctx); err != nil {
		t.Fatal(err)
	}
	defer srv.Cleanup(ctx)

	// Load fixtures
	if err := srv.LoadFixtures(tmpDir); err != nil {
		t.Fatalf("LoadFixtures failed: %v", err)
	}

	// Verify fixtures were loaded
	if len(srv.fixtureStubs) != 2 {
		t.Fatalf("expected 2 fixture stubs, got %d", len(srv.fixtureStubs))
	}

	// Test inline body stub
	resp, err := http.Get(srv.GetURL() + "/api/test")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	if string(body) != `{"test": "value"}` {
		t.Errorf("unexpected body: %s", body)
	}

	// Test bodyFile stub
	resp, err = http.Get(srv.GetURL() + "/api/file")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}

	body, _ = io.ReadAll(resp.Body)
	if string(body) != responseJSON {
		t.Errorf("unexpected body: %s", body)
	}
}

func TestFixtureConditions(t *testing.T) {
	tmpDir := t.TempDir()

	// Create stubs with conditions
	stubsYAML := `stubs:
  - id: with-header
    method: GET
    path: /api/protected
    conditions:
      headers:
        Authorization:
          contains: "Bearer "
    response:
      status: 200
      body: '{"authorized": true}'

  - id: without-header
    method: GET
    path: /api/protected
    response:
      status: 401
      body: '{"authorized": false}'

  - id: with-query
    method: GET
    path: /api/search
    conditions:
      query:
        page: "1"
    response:
      status: 200
      body: '{"page": 1}'
`

	if err := os.WriteFile(filepath.Join(tmpDir, "stubs.yml"), []byte(stubsYAML), 0644); err != nil {
		t.Fatal(err)
	}

	// Create server
	srv, err := NewHTTPServer("test", config.Resource{}, nil)
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	if err := srv.Init(ctx); err != nil {
		t.Fatal(err)
	}
	defer srv.Cleanup(ctx)

	if err := srv.LoadFixtures(tmpDir); err != nil {
		t.Fatalf("LoadFixtures failed: %v", err)
	}

	// Test with header - should match first stub (more specific)
	req, _ := http.NewRequest("GET", srv.GetURL()+"/api/protected", nil)
	req.Header.Set("Authorization", "Bearer token123")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	if string(body) != `{"authorized": true}` {
		t.Errorf("unexpected body: %s", body)
	}

	// Test without header - should match second stub (less specific)
	resp, err = http.Get(srv.GetURL() + "/api/protected")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 401 {
		t.Errorf("expected status 401, got %d", resp.StatusCode)
	}

	body, _ = io.ReadAll(resp.Body)
	if string(body) != `{"authorized": false}` {
		t.Errorf("unexpected body: %s", body)
	}

	// Test with query parameter
	resp, err = http.Get(srv.GetURL() + "/api/search?page=1")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}
}

func TestDynamicStubOverridesFixture(t *testing.T) {
	tmpDir := t.TempDir()

	stubsYAML := `stubs:
  - id: fixture-stub
    method: GET
    path: /api/test
    response:
      status: 200
      body: '{"source": "fixture"}'
`

	if err := os.WriteFile(filepath.Join(tmpDir, "stubs.yml"), []byte(stubsYAML), 0644); err != nil {
		t.Fatal(err)
	}

	srv, err := NewHTTPServer("test", config.Resource{}, nil)
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	if err := srv.Init(ctx); err != nil {
		t.Fatal(err)
	}
	defer srv.Cleanup(ctx)

	if err := srv.LoadFixtures(tmpDir); err != nil {
		t.Fatal(err)
	}

	// First request should return fixture
	resp, _ := http.Get(srv.GetURL() + "/api/test")
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	if string(body) != `{"source": "fixture"}` {
		t.Errorf("expected fixture response, got: %s", body)
	}

	// Add dynamic stub
	srv.stubsMu.Lock()
	srv.stubs = append(srv.stubs, &HTTPStub{
		Method: "GET",
		Path:   "/api/test",
		Status: 200,
		Body:   `{"source": "dynamic"}`,
	})
	srv.stubsMu.Unlock()

	// Second request should return dynamic stub (overrides fixture)
	resp, _ = http.Get(srv.GetURL() + "/api/test")
	body, _ = io.ReadAll(resp.Body)
	resp.Body.Close()

	if string(body) != `{"source": "dynamic"}` {
		t.Errorf("expected dynamic response, got: %s", body)
	}
}

func TestResetPreservesFixtures(t *testing.T) {
	tmpDir := t.TempDir()

	stubsYAML := `stubs:
  - id: fixture-stub
    method: GET
    path: /api/test
    response:
      status: 200
      body: '{"test": true}'
`

	if err := os.WriteFile(filepath.Join(tmpDir, "stubs.yml"), []byte(stubsYAML), 0644); err != nil {
		t.Fatal(err)
	}

	srv, err := NewHTTPServer("test", config.Resource{}, nil)
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	if err := srv.Init(ctx); err != nil {
		t.Fatal(err)
	}
	defer srv.Cleanup(ctx)

	if err := srv.LoadFixtures(tmpDir); err != nil {
		t.Fatal(err)
	}

	// Add dynamic stub
	srv.stubsMu.Lock()
	srv.stubs = append(srv.stubs, &HTTPStub{
		Method: "GET",
		Path:   "/api/dynamic",
		Status: 200,
		Body:   `{"dynamic": true}`,
	})
	srv.stubsMu.Unlock()

	// Reset
	if err := srv.Reset(ctx); err != nil {
		t.Fatal(err)
	}

	// Verify dynamic stub is cleared
	srv.stubsMu.RLock()
	dynamicCount := len(srv.stubs)
	fixtureCount := len(srv.fixtureStubs)
	srv.stubsMu.RUnlock()

	if dynamicCount != 0 {
		t.Errorf("expected 0 dynamic stubs after reset, got %d", dynamicCount)
	}

	if fixtureCount != 1 {
		t.Errorf("expected 1 fixture stub after reset, got %d", fixtureCount)
	}

	// Verify fixture still works
	resp, _ := http.Get(srv.GetURL() + "/api/test")
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	if string(body) != `{"test": true}` {
		t.Errorf("fixture not working after reset: %s", body)
	}
}
