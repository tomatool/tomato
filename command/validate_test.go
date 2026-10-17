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

func TestValidateResourcesChecksOptions(t *testing.T) {
	v := &Validator{config: &config.Config{
		Containers: map[string]config.Container{"postgres": {Image: "postgres:16"}},
		Resources: map[string]config.Resource{
			"db": {Type: "postgres", Container: "postgres", Options: map[string]any{"schemas": "plato"}},
		},
	}}
	v.validateResources()
	for _, r := range v.results {
		if r.Item == "db" && r.Status == "error" && strings.Contains(r.Message, "options.schemas") {
			return
		}
	}
	t.Fatalf("validate did not flag schemas given as a string: %+v", v.results)
}
