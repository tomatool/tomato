package httpclient

import (
	"bytes"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/tomatool/tomato/config"
	"github.com/tomatool/tomato/stub"
)

type response struct {
	Code   int
	Header http.Header
	Body   []byte
}

type Client struct {
	httpClient   *http.Client
	baseURL      string
	lastResponse *response

	requestHeaders http.Header
	stubs          map[string][]byte
}

var defaultHeaders = map[string][]string{"Content-Type": {"application/json"}}

func New(cfg *config.Resource) (*Client, error) {
	params := cfg.Params

	httpClient := &http.Client{
		// In order for http client not to follow response redirect
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	client := &Client{httpClient, "", nil, defaultHeaders, nil}
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

	path, ok := cfg.Params["stubs_path"]
	if ok {
		var err error
		client.stubs, err = stub.Retrieve(path)
		if err != nil {
			return nil, err
		}
	}
	return client, nil
}

// Open satisfies resource interface
func (c *Client) Open() error {
	return nil
}

func (c *Client) Ready() error {
	resp, err := c.httpClient.Get(c.baseURL)
	if err != nil {
		return err
	}
	if resp.StatusCode == http.StatusServiceUnavailable {
		return errors.New("http/client: server unavailable > " + c.baseURL)
	}
	return nil
}

func (c *Client) Reset() error {
	c.lastResponse = nil
	c.requestHeaders = defaultHeaders
	return nil
}

// Close satisfies resource interface
func (c *Client) Close() error {
	return nil
}

func (c *Client) Response() (int, http.Header, []byte, error) {
	if c.lastResponse == nil {
		return 0, nil, nil, errors.New("no request has been sent, please send request before checking response")
	}
	return c.lastResponse.Code, c.lastResponse.Header, c.lastResponse.Body, nil
}

func (c *Client) SetRequestHeader(key, value string) error {
	c.requestHeaders.Set(key, value)
	return nil
}

func (c *Client) RequestFromFile(method, path, fileName string) error {
	body, ok := c.stubs[fileName]
	if !ok {
		files := make([]string, len(c.stubs))
		for file := range c.stubs {
			files = append(files, file)
		}
		return errors.Errorf("no stubs loaded with file name: %s available: %s", fileName, strings.Join(files, ", "))
	}
	return c.Request(method, path, body)
}

func (c *Client) Request(method, path string, body []byte) error {
	req, err := http.NewRequest(method, path, bytes.NewBuffer(body))
	if err != nil {
		return err
	}

	req.Header = c.requestHeaders

	if c.baseURL != "" {
		baseURL, err := url.Parse(c.baseURL)
		if err != nil {
			return err
		}
		req.URL.Scheme = baseURL.Scheme
		req.URL.Host = baseURL.Host
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	c.lastResponse = &response{resp.StatusCode, resp.Header, body}
	return nil
}
