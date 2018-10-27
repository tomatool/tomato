package handler

import (
	"io/ioutil"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/godog/gherkin"
	"github.com/stretchr/testify/assert"
	"github.com/tomatool/tomato/config"
	"github.com/tomatool/tomato/resource"
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

var letterRunes = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")

func r(n int) string {
	b := make([]rune, n)
	for i := range b {
		b[i] = letterRunes[rand.Intn(len(letterRunes))]
	}
	return string(b)
}

func TestSendRequestWithBody(t *testing.T) {
	var (
		reqMethod string
		reqURL    *url.URL
		reqBody   []byte
		reqCount  int64
	)
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		reqURL = r.URL
		reqMethod = r.Method
		reqBody, _ = ioutil.ReadAll(r.Body)
		reqCount++
	})
	srv := httptest.NewServer(mux)

	h := &Handler{resource.NewManager([]*config.Resource{{Name: "httpcli", Type: "http/client", Params: map[string]string{"base_url": srv.URL}}})}

	testCases := []struct {
		name string

		resource string
		target   string
		body     string

		err string
	}{
		{
			"undefined resource",
			"randomresource",
			"GET /",
			"",
			"resource not found",
		},
		{
			"test if path is getting called correctly",
			"httpcli",
			"GET /" + r(15) + "?" + r(10) + "=" + r(5),
			"",
			"",
		},
		{
			"test if body is passed",
			"httpcli",
			"GET /" + r(15) + "?" + r(10) + "=" + r(5),
			`{"body":true}`,
			"",
		},
	}

	for _, test := range testCases {
		reqURL = nil
		reqMethod = ""
		reqBody = nil

		err := h.sendRequestWithBody(test.resource, test.target, &gherkin.DocString{Content: test.body})
		if test.err != "" {
			assert.NotNil(t, err)
			assert.Contains(t, err.Error(), test.err)
			continue
		}

		u := strings.Split(test.target, " ")
		assert.Equal(t, u[0], reqMethod)
		assert.Equal(t, u[1], reqURL.RequestURI())
		if test.body != "" {
			assert.EqualValues(t, test.body, string(reqBody))
		}

		assert.Nil(t, err)
	}

	if reqCount != 2 {
		t.Errorf("expecting request count to be 2, got %d", reqCount)
	}
}

func TestCheckResponseCode(t *testing.T) {
	var (
		respCode int    = http.StatusOK
		respBody []byte = []byte(`{}`)
	)
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(respCode)
		w.Write(respBody)
	})
	srv := httptest.NewServer(mux)

	h := &Handler{resource.NewManager([]*config.Resource{{Name: "httpcli", Type: "http/client", Params: map[string]string{"base_url": srv.URL}}})}

	err := h.checkResponseCode("uyeah", 200)
	assert.Error(t, err)

	err = h.checkResponseCode("httpcli", 200)
	assert.Error(t, err)

	testCases := []struct {
		name string

		resource string

		err      string
		respCode int
		respBody string
	}{
		{
			"undefined resource",
			"uyeah",
			"resource not found",
			200,
			``,
		},
		{
			"check response code",
			"httpcli",
			"",
			200,
			`{"name":"joni"}`,
		},
		{
			"check response code",
			"httpcli",
			"expecting response code to be 201",
			201,
			`{"name":"joni"}`,
		},
	}

	for _, test := range testCases {
		h.sendRequest(test.resource, "GET /")

		err := h.checkResponseCode(test.resource, test.respCode)
		if test.err != "" {
			assert.NotNil(t, err)
			assert.Contains(t, err.Error(), test.err)
			continue
		}
		assert.Nil(t, err)
	}
}
func TestCheckResponseBody(t *testing.T) {
	var (
		respCode int    = http.StatusOK
		respBody []byte = []byte(``)
	)
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(respCode)
		w.Write(respBody)
	})
	srv := httptest.NewServer(mux)

	h := &Handler{resource.NewManager([]*config.Resource{{Name: "httpcli", Type: "http/client", Params: map[string]string{"base_url": srv.URL}}})}

	err := h.checkResponseBody("httpcli", nil)
	assert.Error(t, err)

	testCases := []struct {
		name string

		resource string

		serverResp string
		respBody   string

		err string
	}{
		{
			"undefined resource",
			"uyeah",
			"{}",
			``,
			"resource not found",
		},
		{
			"check response body",
			"httpcli",
			`not json`,
			`{}`,
			"invalid character",
		},
		{
			"check response body",
			"httpcli",
			`{}`,
			`ulala`,
			"invalid character",
		},
		{
			"check response body",
			"httpcli",
			`{}`,
			`{"name":"joni"}`,
			"unexpected response body",
		},
		{
			"check response body",
			"httpcli",
			`{"name":"joni"}`,
			`{"name":"joni"}`,
			"",
		},
		{
			"check response body",
			"httpcli",
			`{"name":"joni"}`,
			`{"name":"*"}`,
			"",
		},
	}

	for _, test := range testCases {
		respBody = []byte(test.serverResp)
		h.sendRequest(test.resource, "GET /")

		err := h.checkResponseBody(test.resource, &gherkin.DocString{Content: test.respBody})
		if test.err != "" {
			assert.NotNil(t, err)
			if err != nil {
				assert.Contains(t, err.Error(), test.err)
			}
			continue
		}
		assert.Nil(t, err)
	}
}
func TestCheckResponseHeader(t *testing.T) {
	var (
		headers = make(map[string]string)
	)
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		for key, val := range headers {
			w.Header().Set(key, val)
		}
		w.WriteHeader(http.StatusOK)
	})
	srv := httptest.NewServer(mux)

	h := &Handler{resource.NewManager([]*config.Resource{{Name: "httpcli", Type: "http/client", Params: map[string]string{"base_url": srv.URL}}})}

	err := h.checkResponseHeader("httpcli", "", "")
	assert.Error(t, err)

	testCases := []struct {
		name string

		resource string

		responseHeaders map[string]string
		gotHeaders      map[string]string

		err string
	}{
		{
			"undefined resource",
			"uyeah",
			map[string]string{},
			map[string]string{"Content-Type": "application/json"},
			"resource not found",
		},
		{
			"check response body",
			"httpcli",
			map[string]string{},
			map[string]string{"Content-Type": "application/json"},
			"unexpected response header",
		},
		{
			"check response body",
			"httpcli",
			map[string]string{"Content-Type": "application/json"},
			map[string]string{"Content-Type": "application/json"},
			"",
		},
	}

	for _, test := range testCases {
		headers = test.responseHeaders
		h.sendRequest(test.resource, "GET /")

		for key, value := range test.gotHeaders {
			err := h.checkResponseHeader(test.resource, key, value)
			if test.err != "" {
				assert.NotNil(t, err)
				if err != nil {
					assert.Contains(t, err.Error(), test.err)
				}
				continue
			}
			assert.Nil(t, err)
		}
	}
}
