package http

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
	options      *Options
	httpClient   *http.Client
	lastResponse *Response
}

type Options struct {
	BaseURL string
	Timeout time.Duration
}

func New(o *Options) *Client {
	client := new(http.Client)
	if o != nil {
		client.Timeout = o.Timeout
	}
	return &Client{o, client, nil}
}

func (c *Client) Do(req *http.Request) error {
	if c.options.BaseURL != "" {
		baseURL, err := url.Parse(c.options.BaseURL)
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
