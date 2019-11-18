package config

import (
	"bytes"
	"io/ioutil"
	"log"
	"os"
	"strings"
	"text/template"

	"github.com/pkg/errors"
	yaml "gopkg.in/yaml.v2"
)

type Config struct {
	Logger          *log.Logger
	Composer        string      `yaml:"composer"`
	ResourcesConfig []*Resource `yaml:"resources"`

	FeaturesPaths    []string `yaml:"features_path"`
	StopOnFailure    bool     `yaml:"stop_on_failure"`
	Randomize        bool     `yaml:"randomize"`
	ReadinessTimeout string   `yaml:"readiness_timeout"`
}
type Resource struct {
	Name   string            `yaml:"name"`
	Type   string            `yaml:"type"`
	Image  string            `yaml:"image"`
	Params map[string]string `yaml:"params"`
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
