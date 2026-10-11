package httpserver

import (
	"context"
	"strings"
	"testing"

	"github.com/tomatool/tomato/internal/config"
	"github.com/tomatool/tomato/internal/resource"
)

func TestHTTPServer_StoreURL(t *testing.T) {
	srv, _ := New("mock", config.Resource{Options: map[string]any{}}, nil)
	if err := srv.Init(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer srv.Cleanup(context.Background())
	if err := srv.storeURL("MOCK_URL_TEST"); err != nil {
		t.Fatal(err)
	}
	if got := resource.ReplaceVariables("{{MOCK_URL_TEST}}"); got != srv.GetURL() || !strings.HasPrefix(got, "http://localhost:") {
		t.Errorf("stored url = %q, want %q", got, srv.GetURL())
	}
}
