package config

import (
	"bytes"
	"io/ioutil"
	"os"
	"strings"
	"text/template"

	"github.com/pkg/errors"
	yaml "gopkg.in/yaml.v2"
)

type Config struct {
	Randomize        bool        `json:"randomize" yaml:"randomize"`
	StopOnFailure    bool        `json:"stop_on_failure" yaml:"stop_on_failure"`
	FeaturesPaths    []string    `json:"feature_paths" yaml:"features_path"`
	ReadinessTimeout string      `json:"readiness_timeout" yaml:"readiness_timeout"`
	Resources        []*Resource `json:"resources" yaml:"resources"`
}

type Resource struct {
	Name    string            `json:"name" yaml:"name"`
	Type    string            `json:"type" yaml:"type"`
	Options map[string]string `json:"options" yaml:"options"`
}

func Retrieve(configFile string) (*Config, error) {
	b, err := ioutil.ReadFile(configFile)
	if err != nil {
		return nil, err
	}

	envs := make(map[string]string)
	for _, e := range os.Environ() {
		pair := strings.Split(e, "=")
		envs[pair[0]] = strings.Join(pair[1:], "=")
	}

	t := template.Must(template.New("config").Parse(string(b)))

	var buf bytes.Buffer
	if err := t.ExecuteTemplate(&buf, "config", envs); err != nil {
		return nil, errors.Wrapf(err, "render template : %s", string(b))
	}

	var cfg Config
	if err := yaml.Unmarshal(buf.Bytes(), &cfg); err != nil {
		return nil, errors.Wrapf(err, "unmarshal yaml : %s", buf.String())
	}

	return &cfg, nil
}
