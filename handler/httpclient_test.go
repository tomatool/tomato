package handler

import (
	"net/http"
	"testing"

	"github.com/DATA-DOG/godog/gherkin"
)

var (
	resourceHTTPClient = &resourceHTTPClientMock{}
)

type resourceHTTPClientMock struct {
	requestSent *http.Request
}

func (mgr *resourceHTTPClientMock) Ready() error { return nil }
func (mgr *resourceHTTPClientMock) Close() error { return nil }

func (c *resourceHTTPClientMock) Do(req *http.Request) error {
	c.requestSent = req
	return nil
}
func (c *resourceHTTPClientMock) ResponseCode() int    { return 200 }
func (c *resourceHTTPClientMock) ResponseBody() []byte { return []byte(`{"awesome":"response"}`) }

func TestSendRequestToWithBody(t *testing.T) {
	if err := h.sendRequestToWithBody("httpcli-resource", "POST", &gherkin.DocString{Content: "{}"}); err == nil {
		t.Fatal("expecting err invalid target format, got nil")
	}

	if err := h.sendRequestToWithBody("httpcli-resource", "POST /selection", &gherkin.DocString{Content: "{}"}); err != nil {
		t.Fatal(err)
	}

	req := resourceHTTPClient.requestSent

	if uri := req.URL.RequestURI(); uri != "/selection" {
		t.Errorf("expecting request URI to be /selection, got %s", uri)
	}

	if m := req.Method; m != "POST" {
		t.Errorf("expecting request URI to be POST, got %s", m)
	}

}

func TestResponseCodeShouldBe(t *testing.T) {
	if err := h.responseCodeShouldBe("httpcli-resource", 201); err == nil {
		t.Fatal("expecting err mismatch response code, got nil")
	}
	if err := h.responseCodeShouldBe("httpcli-resource", 200); err != nil {
		t.Fatal(err)
	}
}

func TestResponseBodyShouldBe(t *testing.T) {
	if err := h.responseBodyShouldBe("httpcli-resource", &gherkin.DocString{
		Content: `{"awesome":"response"}`,
	}); err != nil {
		t.Fatal(err)
	}
	if err := h.responseBodyShouldBe("httpcli-resource", &gherkin.DocString{
		Content: `{"awesome":200}`,
	}); err == nil {
		t.Fatal("expecting err to be mismatch response body, got nil")
	}
}
