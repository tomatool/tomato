package wiremock

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"

	"github.com/pkg/errors"
	"github.com/tomatool/tomato/config"
)

// Name sets the reference to be used in the tomato.yml file
const Name = "http/server"

// Wiremock contains the configuration for the wiremock stubbing resource
type Wiremock struct {
	host string
	port int
}

// New connects and creates the wiremock resource
// create the stub via the API you can post the request/response JSON to http://<host>:<port>/__admin/mappings
func New(cfg *config.Resource) (*Wiremock, error) {
	p, ok := cfg.Params["port"]
	if !ok {
		return nil, errors.New("http/server: port is required")
	}

	host, ok := cfg.Params["host"]
	if !ok {
		return nil, errors.New("http/server: host is required")
	}

	port, err := strconv.Atoi(p)
	if err != nil {
		return nil, errors.Wrap(err, "http/server: unrecognized port format")
	}

	return &Wiremock{port: port, host: host}, nil
}

// Ready informs tomato of when wiremock is ready to handle connections
func (w *Wiremock) Ready() error {
	resp, err := http.Get(w.statusURL())
	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusOK {
		return errors.New("wiremock not ready")
	}

	return nil
}

// Reset removes all mappings and responses to put wiremock back into its
// base state
func (w *Wiremock) Reset() error {
	// set the mapping
	resp, err := http.Post(w.resetURL(), "application/json", nil)
	if err != nil {
		return err
	}

	respBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusOK {
		return errors.Wrap(errors.New("failed to reset mappings"), string(respBody))
	}

	return nil
}

// SetResponse satisfies the http/server interface for setting requests and their responses
func (w *Wiremock) SetResponse(requestPath string, responseCode int, responseBody []byte) error {
	m := mapping{}
	// todo make this customizable, and update http server resource to do same
	m.Request.Method = "GET"
	m.Request.URLPath = requestPath
	m.Response.Status = responseCode
	m.Response.Base64Body = responseBody
	m.Response.Headers.ContentType = "application/json"
	return w.createMapping(&m)
}

func (w *Wiremock) mappingURL() string {
	// http://<host>:<port>/__admin/mappings
	return fmt.Sprintf("http://%s:%v/__admin/mappings", w.host, w.port)
}

func (w *Wiremock) resetURL() string {
	// http://<host>:<port>/__admin/reset
	return fmt.Sprintf("http://%s:%v/__admin/reset", w.host, w.port)
}

func (w *Wiremock) statusURL() string {
	// http://<host>:<port>/__admin/reset
	return fmt.Sprintf("http://%s:%v/__admin/docs", w.host, w.port)
}

func (w *Wiremock) createMapping(m *mapping) error {
	body, err := json.Marshal(m)
	if err != nil {
		return err
	}
	// set the mapping
	resp, err := http.Post(w.mappingURL(), "application/json", bytes.NewReader(body))
	if err != nil {
		return err
	}

	respBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusCreated {
		return errors.Wrap(errors.New("failed to create mapping"), string(respBody))
	}

	return nil
}
