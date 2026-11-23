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

func TestCommandEnv_JavaAgentJoinsJavaToolOptions(t *testing.T) {
	const agent = "-javaagent:/run/coverage/jacocoagent.jar=destfile=/run/coverage/jacoco.exec"
	tests := []struct {
		name      string
		inherited string
		appEnv    map[string]string
		want      string
	}{
		{"alone", "", nil, agent},
		{"after tomato's environment", "-Xmx1g", nil, "-Xmx1g " + agent},
		{"after app.env, which wins over tomato's environment", "-Xmx1g", map[string]string{"JAVA_TOOL_OPTIONS": "-Dfoo=bar"}, "-Dfoo=bar " + agent},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("JAVA_TOOL_OPTIONS", tt.inherited)
			r := NewRunner(config.AppConfig{Command: "java -jar app.jar"}, nil)
			r.SetJavaAgent(agent)
			env := r.commandEnv(tt.appEnv)

			if v, _ := valueOf(env, "JAVA_TOOL_OPTIONS"); v != tt.want {
				t.Errorf("JAVA_TOOL_OPTIONS = %q, want %q", v, tt.want)
			}
			n := 0
			for _, kv := range env {
				if len(kv) > 18 && kv[:18] == "JAVA_TOOL_OPTIONS=" {
					n++
				}
			}
			if n != 1 {
				t.Errorf("JAVA_TOOL_OPTIONS is set %d times, want once", n)
			}
		})
	}
}

func TestCommandEnv_NoJavaAgentLeavesJavaToolOptions(t *testing.T) {
	t.Setenv("JAVA_TOOL_OPTIONS", "-Xmx1g")
	env := NewRunner(config.AppConfig{Command: "java -jar app.jar"}, nil).commandEnv(nil)
	if v, _ := valueOf(env, "JAVA_TOOL_OPTIONS"); v != "-Xmx1g" {
		t.Errorf("JAVA_TOOL_OPTIONS = %q, want it inherited untouched", v)
	}
}
