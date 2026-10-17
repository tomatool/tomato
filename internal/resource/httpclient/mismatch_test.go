package httpclient

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/cucumber/godog"

	"github.com/tomatool/tomato/internal/jsonmatch"
)

// The body and JSON assertions fail with the text they always printed, and
// carry both sides for the UI's diff.
func TestHTTPClient_ComparisonsFailWithAMismatch(t *testing.T) {
	const body = `{"id":"1","sku":"TMT-01","status":"pending"}`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, body)
	}))
	defer server.Close()
	client := newTestHTTPClient(t, server.URL)
	if err := client.sendRequest("GET", "/"); err != nil {
		t.Fatal(err)
	}
	doc := func(s string) *godog.DocString { return &godog.DocString{Content: s} }

	cases := []struct {
		name     string
		err      error
		text     string
		mode     jsonmatch.Mode
		expected string
	}{
		{"body is", client.responseBodyShouldBe(doc(`{"sku":"TMT-01"}`)),
			"body mismatch:\nexpected: {\"sku\":\"TMT-01\"}\nactual: " + body, jsonmatch.ModeText, `{"sku":"TMT-01"}`},
		{"json contains", client.responseJSONShouldContain(doc(`{"status": "created"}`)),
			"at status: expected created, got pending", jsonmatch.ModeSubset, `{"status": "created"}`},
		{"json matches", client.responseJSONShouldMatch(doc(`{"status": "pending"}`)),
			"", jsonmatch.ModeJSON, `{"status": "pending"}`},
	}
	for _, c := range cases {
		if c.err == nil {
			t.Fatalf("%s: passed", c.name)
		}
		if c.text != "" && c.err.Error() != c.text {
			t.Errorf("%s: text = %q, want %q", c.name, c.err.Error(), c.text)
		}
		var m *jsonmatch.Mismatch
		if !errors.As(c.err, &m) {
			t.Fatalf("%s: not a Mismatch: %v", c.name, c.err)
		}
		if m.Mode != c.mode || m.Expected != c.expected || m.Actual != body {
			t.Errorf("%s: mismatch = %+v", c.name, m)
		}
	}
}
