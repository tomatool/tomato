package handler

import (
	"context"
	"strings"
	"testing"

	"github.com/tomatool/tomato/internal/config"
)

func TestInitAppEnvProviders_InitializesOnlyProvidersAndMergesTheirEnv(t *testing.T) {
	registry, err := NewRegistry(map[string]config.Resource{
		"aws":  {Type: "aws", Options: map[string]any{"region": "eu-central-1"}},
		"mock": {Type: "http-server"},
	}, nil)
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}
	t.Cleanup(func() { _ = registry.Cleanup(context.Background()) })

	env, err := registry.InitAppEnvProviders(context.Background())
	if err != nil {
		t.Fatalf("InitAppEnvProviders: %v", err)
	}
	if env.Set["AWS_REGION"] != "eu-central-1" || !strings.HasPrefix(env.Set["AWS_ENDPOINT_URL_STS"], "http://127.0.0.1:") {
		t.Errorf("provided env %v", env.Set)
	}
	if len(env.Unset) == 0 {
		t.Error("no inherited variables to remove")
	}

	mock, _ := registry.Get("mock")
	if mock.(*HTTPServer).listener != nil {
		t.Error("a resource that provides no app environment was initialized before the app")
	}

	// The runner initializes every resource again once the app is up.
	if err := registry.WaitReady(context.Background()); err != nil {
		t.Fatalf("WaitReady after InitAppEnvProviders: %v", err)
	}
}
