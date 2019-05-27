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
	Randomize     bool        `yaml:"randomize"`
	StopOnFailure bool        `yaml:"stop_on_failure"`
	Features      []string    `yaml:"features"`
	Resources     []*Resource `yaml:"resources"`
}

type Resource struct {
	Name   string            `yaml:"name"`
	Type   string            `yaml:"type"`
	Params map[string]string `yaml:"params"`
}

func Unmarshal(configFile string, target interface{}) error {
	b, err := ioutil.ReadFile(configFile)
	if err != nil {
		return err
	}

	envs := make(map[string]string)
	for _, e := range os.Environ() {
		pair := strings.Split(e, "=")
		envs[pair[0]] = strings.Join(pair[1:], "=")
	}

	t := template.Must(template.New("config").Parse(string(b)))

	var buf bytes.Buffer
	if err := t.ExecuteTemplate(&buf, "config", envs); err != nil {
		return errors.Wrapf(err, "render template : %s", string(b))
	}

	if err := yaml.Unmarshal(buf.Bytes(), &target); err != nil {
		return errors.Wrapf(err, "unmarshal yaml : %s", buf.String())
	}

	return nil
}
