package server

import (
	"net/http"
)

const Name = "http/server"

type Server interface {
	SetResponsePath(uri string, code int, body []byte)
}

func Cast(r interface{}) Server {
	return r.(Server)
}

type response struct {
	code int
	body []byte
}

type server struct {
	port string
	srv  *http.Server

	responses map[string]response
}

func New(params map[string]string) *server {
	port, ok := params["port"]
	if !ok {
		panic("http/server: port is required")
	}

	c := &server{
		port:      port,
		responses: make(map[string]response),
	}
	go c.serve()
	return c
}

func (c *server) Ready() error {
	return nil
}

func (c *server) Close() error {
	return c.srv.Close()
}

const defaultResponseKey = ""

func (c *server) getResponse(path string) response {
	resp, ok := c.responses[path]
	if ok {
		return resp
	}

	resp, ok = c.responses[defaultResponseKey]
	if ok {
		return resp
	}

	// if default is not set, and path not found
	// get any responses exist on response list
	for key := range c.responses {
		return c.responses[key]
	}
	return response{502, []byte("response unavailable")}
}

func (c *server) serve() {
	mux := http.NewServeMux()
	mux.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := c.getResponse(r.URL.RequestURI())
		w.WriteHeader(resp.code)
		w.Write(resp.body)
	}))

	if c.port[0] != ':' {
		c.port = ":" + c.port
	}
	c.srv = &http.Server{
		Addr:    c.port,
		Handler: mux,
	}

	if err := c.srv.ListenAndServe(); err != nil {
		panic(err)
	}
}

func (c *server) SetResponsePath(path string, code int, body []byte) {
	if path == "" {
		path = defaultResponseKey
	}

	c.responses[path] = response{code, body}
}
