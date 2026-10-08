package command

import (
	"net"
	"net/url"
	"sort"
	"strconv"

	"github.com/tomatool/tomato/internal/config"
)

// TopologyJSON is what the UI's flow view draws: the app and every resource
// tomato.yml defines.
type TopologyJSON struct {
	AppPort   int            `json:"appPort,omitempty"`
	Resources []ResourceJSON `json:"resources"`
}

type ResourceJSON struct {
	Name string `json:"name"`
	Type string `json:"type"`
	// Target is set on resources tomato uses as a client: "app", or the
	// resource serving the port it calls, like a websocket-server mock.
	Target string `json:"target,omitempty"`
}

// clientTypes are resource types tomato calls something through, rather than
// a dependency of the app.
var clientTypes = map[string]bool{
	"http": true, "http-client": true,
	"grpc": true, "grpc-client": true,
	"websocket": true, "websocket-client": true,
}

func topology(cfg *config.Config) TopologyJSON {
	t := TopologyJSON{AppPort: cfg.App.Port}

	// Ports tomato serves itself: mocks and the aws resource's STS.
	served := map[string]string{}
	for name, res := range cfg.Resources {
		if p, ok := res.Options["port"].(int); ok {
			served[strconv.Itoa(p)] = name
		}
	}

	for name, res := range cfg.Resources {
		r := ResourceJSON{Name: name, Type: res.Type}
		if clientTypes[res.Type] {
			r.Target = "app"
			if server, ok := served[clientPort(res)]; ok {
				r.Target = server
			}
		}
		t.Resources = append(t.Resources, r)
	}
	sort.Slice(t.Resources, func(i, j int) bool { return t.Resources[i].Name < t.Resources[j].Name })
	return t
}

// clientPort is the port a client resource calls, or "" when it has none.
func clientPort(res config.Resource) string {
	for _, raw := range []string{res.BaseURL, res.URL} {
		if raw == "" {
			continue
		}
		if u, err := url.Parse(raw); err == nil {
			return u.Port()
		}
	}
	if res.Address != "" {
		if _, port, err := net.SplitHostPort(res.Address); err == nil {
			return port
		}
	}
	return ""
}
