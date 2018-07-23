package handler

import (
	"reflect"
	"testing"

	"github.com/DATA-DOG/godog/gherkin"
)

var (
	resourceHTTPServer = &resourceHTTPServerMock{}
)

type resourceHTTPServerMock struct {
	uri  string
	code int
	body []byte
}

func (c *resourceHTTPServerMock) SetResponsePath(uri string, code int, body []byte) {
	c.uri = uri
	c.code = code
	c.body = body
}

func TestSetWithPathResponseCodeToAndResponseBody(t *testing.T) {
	path, code, body := "/mypath", 201, &gherkin.DocString{Content: "hello"}

	h.setWithPathResponseCodeToAndResponseBody("httpsrv-resource", path, code, body)
	if !reflect.DeepEqual(resourceHTTPServer.body, []byte(body.Content)) {
		t.Errorf("%s != %s", string(resourceHTTPServer.body), body.Content)
	}
}
