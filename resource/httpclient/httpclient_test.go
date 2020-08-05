package httpclient_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/pkg/errors"
	"github.com/tomatool/tomato/config"
	"github.com/tomatool/tomato/resource/httpclient"
)

func TestDefaultHeaders(t *testing.T) {
	reqHeader := make(chan http.Header, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqHeader <- r.Header
	}))

	cli, err := httpclient.New(&config.Resource{
		Options: map[string]string{
			"headers": "awesome=header ; rest =true; gokil =men",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 5; i++ {
		if err := cli.Request("GET", srv.URL, nil); err != nil {
			t.Fatal(err)
		}
		select {
		case h := <-reqHeader:
			if err := keyExist(h, "awesome", "rest", "gokil"); err != nil {
				t.Fatal(err)
			}
		case <-time.After(time.Second):
			t.Fatal("timeout")
		}
		cli.Reset()
	}
}

func keyExist(h http.Header, keys ...string) error {
	for _, key := range keys {
		if v := h.Get(key); v == "" {
			return errors.New("missing key: " + key)
		}
	}
	return nil
}
