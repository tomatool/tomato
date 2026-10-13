package httpserver

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"testing/iotest"

	"github.com/cucumber/godog"

	"github.com/tomatool/tomato/internal/config"
	"github.com/tomatool/tomato/internal/resource"
)

func startServer(t *testing.T) *HTTPServer {
	t.Helper()
	srv, _ := New("pm", config.Resource{Options: map[string]any{}}, nil)
	if err := srv.Init(context.Background()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { srv.Cleanup(context.Background()) })
	return srv
}

func post(t *testing.T, srv *HTTPServer, path, body string, headers map[string]string) {
	t.Helper()
	req, _ := http.NewRequest("POST", srv.GetURL()+path, strings.NewReader(body))
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
}

func TestHTTPServer_SaveLastRequestJSON(t *testing.T) {
	srv := startServer(t)
	post(t, srv, "/orders", `{"clientProcessId":"old","n":1}`, nil)
	post(t, srv, "/orders", `{"clientProcessId":"cp-2","size":12.5,"ts":1791900000000,"ok":true,"leg":{"id":"l-1"},"tags":["a"]}`, nil)
	post(t, srv, "/other", `{"clientProcessId":"elsewhere"}`, nil)

	cases := map[string]string{
		"clientProcessId": "cp-2", // the last matching request, not the first or another path
		"size":            "12.5",
		"ts":              "1791900000000", // not 1.7919e+12
		"ok":              "true",
		"leg":             `{"id":"l-1"}`,
		"leg.id":          "l-1",
		"tags[0]":         "a",
	}
	for field, want := range cases {
		if err := srv.saveLastRequestJSON("POST", "/orders", field, "v"); err != nil {
			t.Fatalf("%s: %v", field, err)
		}
		if got := resource.ReplaceVariables("{{v}}"); got != want {
			t.Errorf("%s: saved %q, want %q", field, got, want)
		}
	}
}

func TestHTTPServer_SaveLastRequestJSONErrors(t *testing.T) {
	srv := startServer(t)
	if err := srv.saveLastRequestJSON("POST", "/orders", "id", "v"); err == nil || !strings.Contains(err.Error(), "no POST /orders request received") {
		t.Errorf("want 'no request received', got %v", err)
	}
	post(t, srv, "/orders", `{"id":null}`, nil)
	if err := srv.saveLastRequestJSON("POST", "/orders", "id", "v"); err == nil || !strings.Contains(err.Error(), "is null") {
		t.Errorf("want a null error, got %v", err)
	}
	if err := srv.saveLastRequestJSON("POST", "/orders", "missing", "v"); err == nil {
		t.Error("want a missing field error")
	}
	post(t, srv, "/text", `not json`, nil)
	if err := srv.saveLastRequestJSON("POST", "/text", "id", "v"); err == nil || !strings.Contains(err.Error(), "not json") {
		t.Errorf("want the body in the error, got %v", err)
	}
}

func TestHTTPServer_PathUsesVariables(t *testing.T) {
	srv := startServer(t)
	post(t, srv, "/orders/o-9/cancel", `{"reason":"user"}`, nil)
	resource.SetVariable("order_id", "o-9")
	if err := srv.saveLastRequestJSON("POST", "/orders/{{order_id}}/cancel", "reason", "v"); err != nil {
		t.Fatal(err)
	}
}

func TestHTTPServer_SaveLastRequestHeader(t *testing.T) {
	srv := startServer(t)
	post(t, srv, "/orders", `{}`, map[string]string{"Idempotency-Key": "k-1"})
	if err := srv.saveLastRequestHeader("POST", "/orders", "Idempotency-Key", "key"); err != nil {
		t.Fatal(err)
	}
	if got := resource.ReplaceVariables("{{key}}"); got != "k-1" {
		t.Errorf("saved %q", got)
	}
	if err := srv.saveLastRequestHeader("POST", "/orders", "X-Missing", "key"); err == nil {
		t.Error("want a missing header error")
	}
	if err := srv.saveLastRequestHeader("GET", "/orders", "Idempotency-Key", "key"); err == nil {
		t.Error("want a no request error")
	}
}

func TestHTTPServer_LastRequestJSONContains(t *testing.T) {
	srv := startServer(t)
	post(t, srv, "/orders", `{"orderSize":"10","exchangeId":"LSX","isDMA":false}`, nil)
	doc := func(s string) *godog.DocString { return &godog.DocString{Content: s} }

	resource.SetVariable("size", "10")
	if err := srv.lastRequestJSONContains("POST", "/orders", doc(`{"orderSize":"{{size}}","isDMA":false}`)); err != nil {
		t.Fatal(err)
	}
	if err := srv.lastRequestJSONContains("POST", "/orders", doc(`{"orderSize":"11"}`)); err == nil || !strings.Contains(err.Error(), "orderSize") {
		t.Errorf("want a mismatch, got %v", err)
	}
	if err := srv.lastRequestJSONContains("POST", "/orders", doc(`{bad`)); err == nil || !strings.Contains(err.Error(), "invalid expected JSON") {
		t.Errorf("want an invalid expected JSON error, got %v", err)
	}
	if err := srv.lastRequestJSONContains("DELETE", "/orders", doc(`{}`)); err == nil {
		t.Error("want a no request error")
	}
	post(t, srv, "/text", `plain`, nil)
	if err := srv.lastRequestJSONContains("POST", "/text", doc(`{}`)); err == nil || !strings.Contains(err.Error(), "not JSON") {
		t.Errorf("want a not JSON error, got %v", err)
	}
}

// A body that arrives in pieces is recorded whole.
func TestHTTPServer_RecordsTheWholeBody(t *testing.T) {
	srv := startServer(t)
	body := `{"clientProcessId":"` + strings.Repeat("x", 100000) + `"}`
	req, _ := http.NewRequest("POST", srv.GetURL()+"/orders", iotest.OneByteReader(strings.NewReader(body)))
	req.ContentLength = int64(len(body))
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	call, err := srv.lastCall("POST", "/orders")
	if err != nil {
		t.Fatal(err)
	}
	if call.Body != body {
		t.Fatalf("recorded %d bytes of %d", len(call.Body), len(body))
	}
}
