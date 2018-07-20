package config

import (
	"io/ioutil"

	yaml "gopkg.in/yaml.v2"
)

type Config struct {
	Resources []*Resource `yaml:"resources"`
}

type Resource struct {
	Name   string            `yaml:"name"`
	Type   string            `yaml:"type"`
	Params map[string]string `yaml:"params"`
}

func (r *Resource) Key() string {
	return r.Type + ":" + r.Name
}

func Retrieve(configFile string) (*Config, error) {
	b, err := ioutil.ReadFile(configFile)
	if err != nil {
		return nil, err
	}

	var cfg Config
	if err := yaml.Unmarshal(b, &cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}
