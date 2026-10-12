package command

import (
	"net"
	"net/url"
	"regexp"
	"sort"
	"strconv"

	"github.com/tomatool/tomato/internal/config"
)

// TopologyJSON is what the UI's topology view draws: tomato, the app under
// test, the containers tomato starts and every resource tomato.yml defines,
// with how they are wired to each other before a single step runs.
type TopologyJSON struct {
	// AppPort is the app's port, kept for the node label; App is nil when
	// tomato.yml has no app.
	AppPort    int                 `json:"appPort,omitempty"`
	App        *TopoAppJSON        `json:"app,omitempty"`
	Containers []TopoContainerJSON `json:"containers"`
	Resources  []TopoResourceJSON  `json:"resources"`
}

// TopoAppJSON is the app under test.
type TopoAppJSON struct {
	Name string `json:"name"`
	// Mode is how tomato runs it: command, image or build.
	Mode string `json:"mode"`
	Port int    `json:"port,omitempty"`
	// Uses names the containers and resources the app's env refers to through
	// {{.name.host}}, {{.name.port.N}} and {{.name.url}} templates: what the
	// app is wired to by configuration.
	Uses []string `json:"uses,omitempty"`
}

// TopoContainerJSON is a container tomato starts.
type TopoContainerJSON struct {
	Name string `json:"name"`
	// Image is the image, or the preset when the entry is a preset.
	Image     string   `json:"image"`
	Ports     []string `json:"ports,omitempty"`
	DependsOn []string `json:"dependsOn,omitempty"`
}

// TopoResourceJSON is a resource tomato.yml defines.
type TopoResourceJSON struct {
	Name string `json:"name"`
	Type string `json:"type"`
	// Role is how the resource sits in the picture: "client" (tomato sends
	// through it), "mock" (tomato serves it and the app calls it), "host" (it
	// runs on the machine) or "service" (a dependency, in a container tomato
	// starts or outside).
	Role string `json:"role"`
	// Container is the container the resource is backed by, when it is one
	// tomato starts.
	Container string `json:"container,omitempty"`
	// Target is set on clients: "app", or the resource serving the port the
	// client calls, like a websocket-server mock. Address is the configured
	// address the client calls, for the label when neither is known.
	Target  string `json:"target,omitempty"`
	Address string `json:"address,omitempty"`
	// Port is the port a mock serves.
	Port int `json:"port,omitempty"`
}

// clientTypes are resource types tomato calls something through, rather than
// a dependency of the app.
var clientTypes = map[string]bool{
	"http": true, "http-client": true,
	"grpc": true, "grpc-client": true,
	"websocket": true, "websocket-client": true,
}

// mockTypes are resource types tomato serves itself: the app calls them,
// tomato only observes the traffic arriving.
var mockTypes = map[string]bool{
	"http-server": true, "websocket-server": true, "aws": true,
}

// templateRef matches the {{.name.…}} references app.env values use.
var templateRef = regexp.MustCompile(`\{\{\s*\.([A-Za-z0-9_-]+)\.`)

func topology(cfg *config.Config) TopologyJSON {
	t := TopologyJSON{AppPort: cfg.App.Port, Containers: []TopoContainerJSON{}, Resources: []TopoResourceJSON{}}

	for name, c := range cfg.Containers {
		image := c.Image
		if image == "" && c.Preset != "" {
			image = "preset: " + c.Preset
		}
		t.Containers = append(t.Containers, TopoContainerJSON{Name: name, Image: image, Ports: c.Ports, DependsOn: c.DependsOn})
	}
	sort.Slice(t.Containers, func(i, j int) bool { return t.Containers[i].Name < t.Containers[j].Name })

	// Ports tomato serves itself: mocks and the aws resource's STS.
	served := map[string]string{}
	for name, res := range cfg.Resources {
		if p, ok := res.Options["port"].(int); ok {
			served[strconv.Itoa(p)] = name
		}
	}

	hasApp := cfg.App.Command != "" || cfg.App.Image != "" || cfg.App.Build != nil
	for name, res := range cfg.Resources {
		r := TopoResourceJSON{Name: name, Type: res.Type, Role: "service"}
		switch {
		case clientTypes[res.Type]:
			r.Role = "client"
			r.Address = clientAddress(res)
			if server, ok := served[clientPort(res)]; ok {
				r.Target = server
			} else if hasApp {
				r.Target = "app"
			}
		case mockTypes[res.Type]:
			r.Role = "mock"
			if p, ok := res.Options["port"].(int); ok {
				r.Port = p
			}
		case res.Type == "shell":
			r.Role = "host"
		}
		if _, ok := cfg.Containers[res.Container]; ok {
			r.Container = res.Container
		}
		t.Resources = append(t.Resources, r)
	}
	sort.Slice(t.Resources, func(i, j int) bool { return t.Resources[i].Name < t.Resources[j].Name })

	if hasApp {
		app := &TopoAppJSON{Name: cfg.App.Name, Mode: "command", Port: cfg.App.Port}
		if app.Name == "" {
			app.Name = "app"
		}
		if cfg.App.Build != nil {
			app.Mode = "build"
		} else if cfg.App.Image != "" {
			app.Mode = "image"
		}
		seen := map[string]bool{}
		for _, v := range cfg.App.Env {
			for _, m := range templateRef.FindAllStringSubmatch(v, -1) {
				ref := m[1]
				_, isContainer := cfg.Containers[ref]
				_, isResource := cfg.Resources[ref]
				if (isContainer || isResource) && !seen[ref] {
					seen[ref] = true
					app.Uses = append(app.Uses, ref)
				}
			}
		}
		sort.Strings(app.Uses)
		t.App = app
	}
	return t
}

// clientAddress is what a client resource is configured to call, or "".
func clientAddress(res config.Resource) string {
	for _, raw := range []string{res.BaseURL, res.URL} {
		if raw == "" {
			continue
		}
		if u, err := url.Parse(raw); err == nil && u.Host != "" {
			return u.Host
		}
		return raw
	}
	return res.Address
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
