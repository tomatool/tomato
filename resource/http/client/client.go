package client

import (
	"io/ioutil"
	"net/http"
	"net/url"
	"time"
)

type Response struct {
	Code int
	Body []byte
}

type Client struct {
	httpClient   *http.Client
	baseURL      string
	lastResponse *Response
}

func T(i interface{}) *Client {
	return i.(*Client)
}

func New(params map[string]string) *Client {
	client := &Client{new(http.Client), "", nil}

	for key, val := range params {
		switch key {
		case "base_url":
			client.baseURL = val
		case "timeout":
			timeout, err := time.ParseDuration(val)
			if err != nil {
				panic("timeout: get http client, invalid params value : " + err.Error())
			}
			client.httpClient.Timeout = timeout
		default:
			panic(key + ": invalid params")
		}
	}
	return client
}

func (c *Client) Close() {}

func (c *Client) Do(req *http.Request) error {
	if c.baseURL != "" {
		baseURL, err := url.Parse(c.baseURL)
		if err != nil {
			return err
		}
		req.URL.Scheme = baseURL.Scheme
		req.URL.Host = baseURL.Host
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	c.lastResponse = &Response{resp.StatusCode, body}
	return nil
}

func (c *Client) LastResponse() *Response {
	return c.lastResponse
}
