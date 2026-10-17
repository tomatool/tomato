package command

import (
	"reflect"
	"testing"

	"github.com/tomatool/tomato/internal/config"
)

// The topology is the wiring tomato.yml describes, not only the resource
// list: which container backs a resource, what the app's env points at, what
// each client calls, and what tomato serves itself.
func TestTopologyDescribesTheWiring(t *testing.T) {
	cfg := &config.Config{
		App: config.AppConfig{
			Command: "go run .",
			Port:    8080,
			Env: map[string]string{
				"DATABASE_URL": "postgres://u:p@{{.postgres.host}}:{{.postgres.port.5432}}/db",
				"CACHE":        "{{ .redis.host }}:{{ .redis.port.6379 }}",
				"UPSTREAM":     "{{.mock.url}}",
				"USERS_ADDR":   "{{.users.address}}",
				"UNKNOWN":      "{{.nothing.host}}",
			},
		},
		Containers: map[string]config.Container{
			"postgres": {Image: "postgres:15-alpine", Ports: []string{"5432/tcp"}},
			"redis":    {Image: "redis:7-alpine"},
			"kafka":    {Preset: "kafka"},
			"chrome":   {Preset: "chromium"},
		},
		Resources: map[string]config.Resource{
			"db":            {Type: "postgres", Container: "postgres"},
			"cache":         {Type: "redis", Container: "redis"},
			"cache2":        {Type: "redis", Container: "redis"},
			"warehouse":     {Type: "postgres", AllowDestructiveReset: true},
			"api":           {Type: "http-client", BaseURL: "http://localhost:8080"},
			"mock":          {Type: "http-server", Options: map[string]any{"port": 9999}},
			"mock-client":   {Type: "http-client", BaseURL: "http://localhost:9999"},
			"wsmock":        {Type: "websocket-server", Options: map[string]any{"port": 9997}},
			"wsmock-client": {Type: "websocket-client", URL: "ws://localhost:9997/"},
			"users":         {Type: "grpc-server", Options: map[string]any{"port": 9995, "descriptor_set": "users.binpb"}},
			"users-client":  {Type: "grpc", Address: "localhost:9995"},
			"shell":         {Type: "shell"},
			"alice":         {Type: "browser", Container: "chrome", Options: map[string]any{"base_url": "http://localhost:8080"}},
		},
	}
	got := topology(cfg)

	if got.App == nil {
		t.Fatal("expected an app node")
	}
	if got.App.Mode != "command" || got.App.Port != 8080 || got.App.Name != "app" {
		t.Errorf("app = %+v", *got.App)
	}
	if want := []string{"mock", "postgres", "redis", "users"}; !reflect.DeepEqual(got.App.Uses, want) {
		t.Errorf("app.uses = %v, want %v (templates naming containers or resources, nothing else)", got.App.Uses, want)
	}

	var names []string
	images := map[string]string{}
	for _, c := range got.Containers {
		names = append(names, c.Name)
		images[c.Name] = c.Image
	}
	if want := []string{"chrome", "kafka", "postgres", "redis"}; !reflect.DeepEqual(names, want) {
		t.Errorf("containers = %v, want %v", names, want)
	}
	if images["kafka"] != "preset: kafka" || images["postgres"] != "postgres:15-alpine" {
		t.Errorf("container images = %v", images)
	}

	res := map[string]TopoResourceJSON{}
	for _, r := range got.Resources {
		res[r.Name] = r
	}
	want := map[string]TopoResourceJSON{
		"db":            {Name: "db", Type: "postgres", Role: "service", Container: "postgres"},
		"cache":         {Name: "cache", Type: "redis", Role: "service", Container: "redis"},
		"cache2":        {Name: "cache2", Type: "redis", Role: "service", Container: "redis"},
		"warehouse":     {Name: "warehouse", Type: "postgres", Role: "service"},
		"api":           {Name: "api", Type: "http-client", Role: "client", Target: "app", Address: "localhost:8080"},
		"mock":          {Name: "mock", Type: "http-server", Role: "mock", Port: 9999},
		"mock-client":   {Name: "mock-client", Type: "http-client", Role: "client", Target: "mock", Address: "localhost:9999"},
		"wsmock":        {Name: "wsmock", Type: "websocket-server", Role: "mock", Port: 9997},
		"wsmock-client": {Name: "wsmock-client", Type: "websocket-client", Role: "client", Target: "wsmock", Address: "localhost:9997"},
		// A gRPC dependency tomato stubs: the app calls it, like an http-server.
		"users":        {Name: "users", Type: "grpc-server", Role: "mock", Port: 9995},
		"users-client": {Name: "users-client", Type: "grpc", Role: "client", Target: "users", Address: "localhost:9995"},
		"shell":        {Name: "shell", Type: "shell", Role: "host"},
		// A person using the app's UI: a client of the app, whose base_url
		// sits among its options, run in the chromium container.
		"alice": {Name: "alice", Type: "browser", Role: "client", Container: "chrome", Target: "app", Address: "localhost:8080"},
	}
	for name, w := range want {
		if g, ok := res[name]; !ok || !reflect.DeepEqual(g, w) {
			t.Errorf("%s = %+v, want %+v", name, res[name], w)
		}
	}
}

// Without an app there is nothing for a client to default to: the node is
// left out and the client only carries the address it calls.
func TestTopologyWithoutAnApp(t *testing.T) {
	cfg := &config.Config{
		Resources: map[string]config.Resource{
			"api":   {Type: "http-client", BaseURL: "http://localhost:3000"},
			"shell": {Type: "shell"},
		},
	}
	got := topology(cfg)
	if got.App != nil {
		t.Errorf("expected no app node, got %+v", *got.App)
	}
	for _, r := range got.Resources {
		if r.Name == "api" && (r.Target != "" || r.Address != "localhost:3000") {
			t.Errorf("api = %+v, want no target and the address it calls", r)
		}
	}
	if got.Containers == nil || got.Resources == nil {
		t.Error("containers and resources must be arrays, not null, for the UI")
	}
}
