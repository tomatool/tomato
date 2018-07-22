package server

import (
	"net/http"
)

const Name = "http/server"

type client struct {
	port string
	srv  *http.Server

	responseCode int
	responseBody []byte
}

func T(i interface{}) *client {
	return i.(*client)
}

func New(params map[string]string) *client {
	port, ok := params["port"]
	if !ok {
		panic("http/server: port is required")
	}

	return &client{port: port}
}

func (c *client) serve() {
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

func (c *client) SetResponse(code int, body []byte) {
	if c.srv == nil {
		go c.serve()
	}
	c.responseCode = code
	c.responseBody = body
}

func (c *client) Close() {
	c.srv.Close()
}
