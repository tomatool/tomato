package apprunner

import (
	"testing"

	"github.com/tomatool/tomato/internal/config"
)

// valueOf returns the value a process would see for key: the last one.
func valueOf(env []string, key string) (string, bool) {
	value, found := "", false
	for _, kv := range env {
		if len(kv) > len(key) && kv[:len(key)+1] == key+"=" {
			value, found = kv[len(key)+1:], true
		}
	}
	return value, found
}

func TestCommandEnv_ProvidedEnvUnsetAndPrecedence(t *testing.T) {
	t.Setenv("AWS_ACCESS_KEY_ID", "developer-key")
	t.Setenv("AWS_REGION", "us-west-2")
	t.Setenv("TOMATO_TEST_KEEP", "kept")

	r := NewRunner(config.AppConfig{Command: "./app"}, nil)
	r.SetProvidedEnv(
		map[string]string{"AWS_REGION": "eu-central-1", "AWS_ROLE_ARN": "arn:aws:iam::000000000000:role/svc"},
		[]string{"AWS_ACCESS_KEY_ID"},
	)
	env := r.commandEnv(map[string]string{"AWS_ROLE_ARN": "arn:aws:iam::000000000000:role/override"})

	if _, found := valueOf(env, "AWS_ACCESS_KEY_ID"); found {
		t.Error("an unset variable was inherited")
	}
	if v, _ := valueOf(env, "TOMATO_TEST_KEEP"); v != "kept" {
		t.Errorf("an ordinary variable was not inherited: %q", v)
	}
	if v, _ := valueOf(env, "AWS_REGION"); v != "eu-central-1" {
		t.Errorf("AWS_REGION = %q; provided env must win over inherited", v)
	}
	if v, _ := valueOf(env, "AWS_ROLE_ARN"); v != "arn:aws:iam::000000000000:role/override" {
		t.Errorf("AWS_ROLE_ARN = %q; the app's own env must win over provided", v)
	}
}

func TestCommandEnv_WithoutProvidedEnvInheritsEverything(t *testing.T) {
	t.Setenv("AWS_ACCESS_KEY_ID", "developer-key")
	r := NewRunner(config.AppConfig{Command: "./app"}, nil)
	if v, _ := valueOf(r.commandEnv(nil), "AWS_ACCESS_KEY_ID"); v != "developer-key" {
		t.Errorf("AWS_ACCESS_KEY_ID = %q; nothing asked to remove it", v)
	}
}
