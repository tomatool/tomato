package client

import (
	"errors"
	"io/ioutil"
	"net/http"
	"net/url"
	"time"
)

const Name = "http/client"

type Client interface {
	Do(req *http.Request) error
	ResponseCode() int
	ResponseBody() []byte
}

func Cast(r interface{}) Client {
	return r.(Client)
}

type response struct {
	Code int
	Body []byte
}

type client struct {
	httpClient   *http.Client
	baseURL      string
	lastResponse *response
}

func Open(params map[string]string) (*client, error) {
	client := &client{new(http.Client), "", nil}
	for key, val := range params {
		switch key {
		case "base_url":
			client.baseURL = val
		case "timeout":
			timeout, err := time.ParseDuration(val)
			if err != nil {
				return nil, errors.New("timeout: get http client, invalid params value : " + err.Error())
			}
			client.httpClient.Timeout = timeout
		default:
			return nil, errors.New(key + ": invalid params")
		}
	}
	return client, nil
}

func (c *client) Ready() error {
	return nil
}

func (c *client) Close() error {
	return nil
}

func (c *client) Do(req *http.Request) error {
	if c.baseURL != "" {
		baseURL, err := url.Parse(c.baseURL)
		if err != nil {
			return err
		}
		req.URL.Scheme = baseURL.Scheme
		req.URL.Host = baseURL.Host
	}

	if req.Method != http.MethodGet {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	c.lastResponse = &response{resp.StatusCode, body}
	return nil
}

func (c *client) ResponseCode() int {
	return c.lastResponse.Code
}

func (c *client) ResponseBody() []byte {
	return c.lastResponse.Body
}
