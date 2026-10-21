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
				"UPSTREAM":     "{{.payments-stub.url}}",
				"USERS_ADDR":   "{{.users-stub.address}}",
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
			"app-db":          {Type: "postgres", Container: "postgres"},
			"app-cache":       {Type: "redis", Container: "redis"},
			"sessions-cache":  {Type: "redis", Container: "redis"},
			"warehouse-db":    {Type: "postgres", AllowDestructiveReset: true},
			"app-client":      {Type: "http-client", BaseURL: "http://localhost:8080"},
			"payments-stub":   {Type: "http-server", Options: map[string]any{"port": 9999}},
			"payments-client": {Type: "http-client", BaseURL: "http://localhost:9999"},
			"prices-stub":     {Type: "websocket-server", Options: map[string]any{"port": 9997}},
			"prices-client":   {Type: "websocket-client", URL: "ws://localhost:9997/"},
			"users-stub":      {Type: "grpc-server", Options: map[string]any{"port": 9995, "descriptor_set": "users.binpb"}},
			"users-client":    {Type: "grpc", Address: "localhost:9995"},
			"shell":           {Type: "shell"},
			"alice-browser":   {Type: "browser", Container: "chrome", Options: map[string]any{"base_url": "http://localhost:8080"}},
		},
	}
	got := topology(cfg)

	if got.App == nil {
		t.Fatal("expected an app node")
	}
	if got.App.Mode != "command" || got.App.Port != 8080 || got.App.Name != "app" {
		t.Errorf("app = %+v", *got.App)
	}
	if want := []string{"payments-stub", "postgres", "redis", "users-stub"}; !reflect.DeepEqual(got.App.Uses, want) {
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
		"app-db":          {Name: "app-db", Type: "postgres", Role: "service", Container: "postgres"},
		"app-cache":       {Name: "app-cache", Type: "redis", Role: "service", Container: "redis"},
		"sessions-cache":  {Name: "sessions-cache", Type: "redis", Role: "service", Container: "redis"},
		"warehouse-db":    {Name: "warehouse-db", Type: "postgres", Role: "service"},
		"app-client":      {Name: "app-client", Type: "http-client", Role: "client", Target: "app", Address: "localhost:8080"},
		"payments-stub":   {Name: "payments-stub", Type: "http-server", Role: "mock", Port: 9999},
		"payments-client": {Name: "payments-client", Type: "http-client", Role: "client", Target: "payments-stub", Address: "localhost:9999"},
		"prices-stub":     {Name: "prices-stub", Type: "websocket-server", Role: "mock", Port: 9997},
		"prices-client":   {Name: "prices-client", Type: "websocket-client", Role: "client", Target: "prices-stub", Address: "localhost:9997"},
		// A gRPC dependency tomato stubs: the app calls it, like an http-server.
		"users-stub":   {Name: "users-stub", Type: "grpc-server", Role: "mock", Port: 9995},
		"users-client": {Name: "users-client", Type: "grpc", Role: "client", Target: "users-stub", Address: "localhost:9995"},
		"shell":        {Name: "shell", Type: "shell", Role: "host"},
		// A person using the app's UI: a client of the app, whose base_url
		// sits among its options, run in the chromium container.
		"alice-browser": {Name: "alice-browser", Type: "browser", Role: "client", Container: "chrome", Target: "app", Address: "localhost:8080"},
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
			"app-client": {Type: "http-client", BaseURL: "http://localhost:3000"},
			"shell":      {Type: "shell"},
		},
	}
	got := topology(cfg)
	if got.App != nil {
		t.Errorf("expected no app node, got %+v", *got.App)
	}
	for _, r := range got.Resources {
		if r.Name == "app-client" && (r.Target != "" || r.Address != "localhost:3000") {
			t.Errorf("app-client = %+v, want no target and the address it calls", r)
		}
	}
	if got.Containers == nil || got.Resources == nil {
		t.Error("containers and resources must be arrays, not null, for the UI")
	}
}
