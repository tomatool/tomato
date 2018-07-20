package server

import (
	"net/http"
)

const Name = "http/server"

type Client struct {
	port string
	srv  *http.Server

	responseCode int
	responseBody []byte
}

func T(i interface{}) *Client {
	return i.(*Client)
}

func New(params map[string]string) *Client {
	port, ok := params["port"]
	if !ok {
		panic("http/server: port is required")
	}

	return &Client{port: port}
}

func (c *Client) serve() {
	mux := http.NewServeMux()
	mux.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(c.responseCode)
		w.Write(c.responseBody)
	}))

	c.srv = &http.Server{
		Addr:    c.port,
		Handler: mux,
	}

	if err := c.srv.ListenAndServe(); err != nil {
		panic(err)
	}
}

func (c *Client) SetResponse(code int, body []byte) {
	if c.srv == nil {
		go c.serve()
	}
	c.responseCode = code
	c.responseBody = body
}

func (c *Client) Close() {
	c.srv.Close()
}
