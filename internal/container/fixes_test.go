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

// A built image used to get a random name on every run, so every run left one
// more image behind.
func TestBuildImageName_IsStablePerProjectAndValid(t *testing.T) {
	repo, tag := buildImageName("Schema_Registry", "/work/a/docker", "Dockerfile")
	if repo != "tomato-schema-registry" {
		t.Errorf("repo %q, want tomato-schema-registry", repo)
	}
	if again, againTag := buildImageName("Schema_Registry", "/work/a/docker", "Dockerfile"); again != repo || againTag != tag {
		t.Errorf("the name changed between runs: %s:%s then %s:%s", repo, tag, again, againTag)
	}
	if _, other := buildImageName("Schema_Registry", "/work/b/docker", "Dockerfile"); other == tag {
		t.Error("two projects' images share a tag")
	}
	if repo, _ := buildImageName("--", "/x", "Dockerfile"); repo != "tomato-build" {
		t.Errorf("a name with nothing usable gives %q", repo)
	}
}

func TestContainerFiles_CopiesContentToItsPath(t *testing.T) {
	files := containerFiles([]config.ContainerFile{{Path: "/opt/kafka/libs/x.jar", Content: []byte("jar"), Mode: 0o644}})
	if len(files) != 1 || files[0].ContainerFilePath != "/opt/kafka/libs/x.jar" || files[0].FileMode != 0o644 {
		t.Fatalf("files = %+v", files)
	}
	var got bytes.Buffer
	if _, err := got.ReadFrom(files[0].Reader); err != nil || got.String() != "jar" {
		t.Errorf("content %q, err %v", got.String(), err)
	}
	if containerFiles(nil) != nil {
		t.Error("no files should copy nothing")
	}
}
