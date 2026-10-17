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
		"users": {Type: "grpc-server", Options: map[string]any{"port": 9995}},
		"api":   {Type: "http-client", BaseURL: "http://localhost:8080"},
	}}}
	v.validateResources()
	got := map[string]ValidationResult{}
	for _, r := range v.results {
		got[r.Item] = r
	}
	if r := got["users"]; r.Status != "error" || !strings.Contains(r.Message, "descriptor_set") {
		t.Errorf("users = %+v, want an error naming descriptor_set", r)
	}
	if r := got["api"]; r.Status != "ok" {
		t.Errorf("api = %+v, want ok", r)
	}
}
