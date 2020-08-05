package config

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestRetrieve(t *testing.T) {
	cfg, err := Retrieve("testdata/conf.good.yml")
	assert.NoError(t, err)

	assert.Equal(t, len(cfg.Resources), 8)
	assert.Equal(t, cfg.StopOnFailure, false)
	assert.Equal(t, len(cfg.FeaturesPaths), 2)
}

func TestEnv(t *testing.T) {
	os.Clearenv()
	timeNow := time.Now().Format(time.RFC3339)
	os.Setenv("samplevar", timeNow)
	os.Setenv("STOP_ON_FAILURE", "true")

	cfg, err := Retrieve("testdata/env.good.yml")
	assert.NoError(t, err)

	assert.Equal(t, cfg.StopOnFailure, true)
	assert.Equal(t, cfg.Resources[0].Options["datasource"], timeNow)
}
