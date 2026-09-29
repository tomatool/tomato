package command

import (
	"testing"

	"github.com/tomatool/tomato/internal/config"
)

func TestContainerSourceNamesWhatRuns(t *testing.T) {
	cases := map[string]config.Container{
		"preset: kafka (auth: aws_msk_iam)": {Preset: "kafka", Auth: "aws_msk_iam", Build: &config.BuildConfig{Context: "/cache/kafka"}},
		"preset: kafka":                     {Preset: "kafka", Image: "ghcr.io/tomatool/tomato-kafka:2.2.0"},
		"image: postgres:16":                {Image: "postgres:16"},
		"build: ./docker":                   {Build: &config.BuildConfig{Context: "./docker"}},
	}
	for want, c := range cases {
		if got := containerSource(c); got != want {
			t.Errorf("containerSource(%+v) = %q, want %q", c, got, want)
		}
	}
}
