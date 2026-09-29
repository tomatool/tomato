package command

import (
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
