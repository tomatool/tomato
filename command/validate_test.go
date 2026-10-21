package command

import (
	"strings"
	"testing"

	"github.com/tomatool/tomato/internal/config"
)

func TestContainerSourceNamesWhatRuns(t *testing.T) {
	cases := map[string]config.Container{
		"preset: kafka (auth: aws_msk_iam)": {Preset: "kafka", Auth: "aws_msk_iam", Image: "apache/kafka:3.9.1"},
		"preset: kafka":                     {Preset: "kafka", Image: "apache/kafka:3.9.1"},
		"image: postgres:16":                {Image: "postgres:16"},
		"build: ./docker":                   {Build: &config.BuildConfig{Context: "./docker"}},
	}
	for want, c := range cases {
		if got := containerSource(c); got != want {
			t.Errorf("containerSource(%+v) = %q, want %q", c, got, want)
		}
	}
}

// A resource that can check its options does so in validate: a grpc-server
// without its descriptor set is an error there, not when the run starts.
func TestValidateReportsResourceOptions(t *testing.T) {
	v := &Validator{config: &config.Config{Resources: map[string]config.Resource{
		"users-stub": {Type: "grpc-server", Options: map[string]any{"port": 9995}},
		"app-client": {Type: "http-client", BaseURL: "http://localhost:8080"},
	}}}
	v.validateResources()
	got := map[string]ValidationResult{}
	for _, r := range v.results {
		got[r.Item] = r
	}
	if r := got["users-stub"]; r.Status != "error" || !strings.Contains(r.Message, "descriptor_set is required") {
		t.Errorf("users = %+v, want an error naming descriptor_set", r)
	}
	if r := got["app-client"]; r.Status != "ok" {
		t.Errorf("api = %+v, want ok", r)
	}
}

func TestValidateResourcesChecksOptions(t *testing.T) {
	v := &Validator{config: &config.Config{
		Containers: map[string]config.Container{"postgres": {Image: "postgres:16"}},
		Resources: map[string]config.Resource{
			"app-db": {Type: "postgres", Container: "postgres", Options: map[string]any{"schemas": "plato"}},
		},
	}}
	v.validateResources()
	for _, r := range v.results {
		if r.Item == "app-db" && r.Status == "error" && strings.Contains(r.Message, "options.schemas") {
			return
		}
	}
	t.Fatalf("validate did not flag schemas given as a string: %+v", v.results)
}
