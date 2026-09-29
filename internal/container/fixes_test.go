package container

import (
	"bytes"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/tomatool/tomato/internal/config"
)

// A template the manager cannot resolve used to be found again on every pass
// of the loop, which never ended.
func TestResolveEnvTemplates_UnresolvableTemplateDoesNotHang(t *testing.T) {
	m, err := NewManager(map[string]config.Container{})
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan map[string]string, 1)
	go func() {
		done <- m.resolveEnvTemplates(map[string]string{
			"A": "{{.x}} and {{.kafka.host}}:{{.kafka.port.29092}}",
			"B": "{{ literal }}",
		})
	}()
	select {
	case got := <-done:
		if got["A"] != "{{.x}} and kafka:29092" {
			t.Errorf("A = %q", got["A"])
		}
		if got["B"] != "{{ literal }}" {
			t.Errorf("B = %q", got["B"])
		}
	case <-time.After(2 * time.Second):
		t.Fatal("resolveEnvTemplates did not return")
	}
}

func TestFileLogConsumer_AppendsEveryLogLine(t *testing.T) {
	var buf bytes.Buffer
	c := &fileLogConsumer{w: &buf}
	c.Accept(testcontainers.Log{LogType: testcontainers.StdoutLog, Content: []byte("started\n")})
	c.Accept(testcontainers.Log{LogType: testcontainers.StderrLog, Content: []byte("later\n")})
	if buf.String() != "started\nlater\n" {
		t.Errorf("log file got %q", buf.String())
	}
}
