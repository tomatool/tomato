package command

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/tomatool/tomato/internal/config"
	"github.com/tomatool/tomato/internal/handler"
)

func newTestAPI(t *testing.T) (*httptest.Server, *atomic.Bool) {
	t.Helper()
	reg, err := handler.NewRegistry(map[string]config.Resource{"sh": {Type: "shell"}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := reg.WaitReady(context.Background()); err != nil {
		t.Fatal(err)
	}
	exec, err := handler.NewExecutor(reg)
	if err != nil {
		t.Fatal(err)
	}
	stopped := &atomic.Bool{}
	api := &serveAPI{registry: reg, executor: exec, shutdown: func() { stopped.Store(true) }}
	srv := httptest.NewServer(api.routes())
	t.Cleanup(srv.Close)
	return srv, stopped
}

func post(t *testing.T, url string, body any) (int, map[string]any) {
	t.Helper()
	data, _ := json.Marshal(body)
	resp, err := http.Post(url, "application/json", bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var out map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&out)
	return resp.StatusCode, out
}

func TestServeAPI(t *testing.T) {
	srv, stopped := newTestAPI(t)

	if resp, err := http.Get(srv.URL + "/v1/health"); err != nil || resp.StatusCode != 200 {
		t.Fatalf("health: %v %v", resp, err)
	}

	var steps []handler.BoundStepInfo
	resp, _ := http.Get(srv.URL + "/v1/steps")
	_ = json.NewDecoder(resp.Body).Decode(&steps)
	if len(steps) == 0 || steps[0].Resource != "sh" {
		t.Fatalf("steps: %+v", steps)
	}

	doc := `echo "value=42"`
	if code, out := post(t, srv.URL+"/v1/steps/run", map[string]any{"step": `"sh" runs:`, "docString": doc}); code != 200 {
		t.Fatalf("run: %d %v", code, out)
	}
	if code, out := post(t, srv.URL+"/v1/steps/run", map[string]any{"step": `"sh" stdout contains "value=42"`}); code != 200 {
		t.Fatalf("assert: %d %v", code, out)
	}
	code, out := post(t, srv.URL+"/v1/steps/run", map[string]any{"step": `"sh" stdout contains "nope"`})
	if code != 422 || out["ok"] != false || out["error"] == "" {
		t.Fatalf("a failing step should be 422 with its error: %d %v", code, out)
	}
	if code, _ := post(t, srv.URL+"/v1/steps/run", "not an object"); code != 400 {
		t.Errorf("bad request should be 400, got %d", code)
	}

	handler.SetVariable("order_id", "o-1")
	resp, _ = http.Get(srv.URL + "/v1/variables/order_id")
	var v map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&v)
	if v["value"] != "o-1" {
		t.Errorf("variable: %v", v)
	}
	if code, _ := post(t, srv.URL+"/v1/reset", nil); code != 200 {
		t.Errorf("reset: %d", code)
	}
	if resp, _ := http.Get(srv.URL + "/v1/variables/order_id"); resp.StatusCode != 404 {
		t.Errorf("reset should clear variables, got %d", resp.StatusCode)
	}
	if code, _ := post(t, srv.URL+"/v1/shutdown", nil); code != 200 {
		t.Errorf("shutdown: %d", code)
	}
	deadline := time.Now().Add(2 * time.Second)
	for !stopped.Load() && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if !stopped.Load() {
		t.Error("shutdown should stop the server")
	}
}
