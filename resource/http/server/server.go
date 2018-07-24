package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
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
	if c.srv == nil {
		return fmt.Errorf("http/server: port %s is not running", c.port)
	}
	return nil
}

func (c *server) Close() error {
	return c.srv.Close()
}

const defaultResponseKey = ""

func (c *server) getResponse(path string) response {
	path = jsonizePath(path)

	resp, ok := c.responses[path]
	if ok {
		return resp
	}

	resp, ok = c.responses[defaultResponseKey]
	if ok {
		return resp
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
		c.srv = nil
		panic(err)
	}
}

func jsonizePath(path string) string {
	splited := strings.Split(path, "?")
	if len(splited) < 2 {
		return path
	}
	left, right := splited[0], strings.Join(splited[1:], "?")

	tmp := make(map[string]string)
	for _, param := range strings.Split(right, "&") {
		var (
			key   string
			value string
		)
		for i, v := range strings.Split(param, "=") {
			if i == 0 {
				key = v
			}
			if i == 1 {
				value = v
			}
			if i > 1 {
				value += ("=" + v)
			}
		}
		tmp[key] = value
	}
	b, err := json.Marshal(tmp)
	if err != nil {
		return path
	}

	return left + "?" + string(b)
}

func (c *server) SetResponsePath(path string, code int, body []byte) {
	path = jsonizePath(path)

	if path == "" {
		path = defaultResponseKey
	}

	c.responses[path] = response{code, body}
}
