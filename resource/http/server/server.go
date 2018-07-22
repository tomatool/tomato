package server

import (
	"net/http"
)

const Name = "http/server"

type Server interface {
	SetResponse(code int, body []byte)
}

func Cast(r interface{}) Server {
	return r.(Server)
}

type server struct {
	port string
	srv  *http.Server

	responseCode int
	responseBody []byte
}

func New(params map[string]string) Server {
	port, ok := params["port"]
	if !ok {
		panic("http/server: port is required")
	}

	return &server{port: port}
}

func (c *server) serve() {
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

func (c *server) SetResponse(code int, body []byte) {
	if c.srv == nil {
		go c.serve()
	}
	c.responseCode = code
	c.responseBody = body
}
