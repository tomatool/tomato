package config

import (
	"bytes"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tomatool/tomato/internal/awsid"
	"github.com/tomatool/tomato/internal/presets"
)

// stubPorts makes the host ports the presets pick predictable: 40001, 40002, ...
func stubPorts(t *testing.T) {
	t.Helper()
	origPort := freePort
	next := 40000
	freePort = func() (string, error) {
		next++
		return fmt.Sprintf("%d", next), nil
	}
	t.Cleanup(func() { freePort = origPort })
}

func TestKafkaPreset_Plaintext(t *testing.T) {
	stubPorts(t)
	cfg, err := Load(createTempConfig(t, `
version: 2
containers:
  kafka:
    preset: kafka
`))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	k := cfg.Containers["kafka"]

	if k.Image != KafkaImage || k.Build != nil {
		t.Errorf("image %q, build %+v; want %s", k.Image, k.Build, KafkaImage)
	}
	if len(k.Files) != 0 {
		t.Errorf("plaintext kafka gets files: %+v", k.Files)
	}
	if len(k.Ports) != 1 || k.Ports[0] != "40001:9092" {
		t.Errorf("ports %v, want [40001:9092]", k.Ports)
	}
	if got := k.Env["KAFKA_ADVERTISED_LISTENERS"]; got != "EXTERNAL://localhost:40001,INTERNAL://kafka:29092" {
		t.Errorf("advertised listeners %q", got)
	}
	if strings.Contains(k.Env["KAFKA_LISTENERS"], "IAM") {
		t.Errorf("plaintext kafka has an IAM listener: %q", k.Env["KAFKA_LISTENERS"])
	}
	if k.WaitFor.Type != "log" || k.WaitFor.Target != "Kafka Server started" {
		t.Errorf("wait_for %+v", k.WaitFor)
	}
}

func TestKafkaPreset_AWSMSKIAMAllowsTheRoleSessionsOfAWSResources(t *testing.T) {
	stubPorts(t)
	cfg, err := Load(createTempConfig(t, `
version: 2
containers:
  broker:
    preset: kafka
    auth: aws_msk_iam
resources:
  aws:
    type: aws
    options:
      role_arn: arn:aws:iam::000000000000:role/my-service
`))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	b := cfg.Containers["broker"]

	// The stock image, with the plugin tomato carries copied onto its classpath.
	if b.Image != KafkaImage || b.Build != nil {
		t.Errorf("image %q, build %+v; want %s", b.Image, b.Build, KafkaImage)
	}
	if len(b.Files) != 1 || b.Files[0].Path != "/opt/kafka/libs/tomato-msk-iam.jar" ||
		!bytes.Equal(b.Files[0].Content, presets.MskIamJar) || b.Files[0].Mode != 0o644 {
		t.Errorf("files %+v, want the MSK IAM plugin in /opt/kafka/libs", b.Files)
	}
	if len(b.Ports) != 2 || b.Ports[1] != "40002:9098" {
		t.Errorf("ports %v, want the IAM listener on 40002:9098", b.Ports)
	}
	wantAdvertised := "EXTERNAL://localhost:40001,INTERNAL://broker:29092,IAM://localhost:40002,IAM_INTERNAL://broker:29098"
	if got := b.Env["KAFKA_ADVERTISED_LISTENERS"]; got != wantAdvertised {
		t.Errorf("advertised listeners %q, want %q", got, wantAdvertised)
	}
	// The image maps _ to . and __ to _, so IAM_INTERNAL's settings are only
	// listener.name.iam_internal.* with its underscore doubled.
	for _, listener := range []string{"IAM", "IAM__INTERNAL"} {
		if got := b.Env["KAFKA_LISTENER_NAME_"+listener+"_SASL_ENABLED_MECHANISMS"]; got != "AWS_MSK_IAM" {
			t.Errorf("%s listener mechanisms %q", listener, got)
		}
		if got := b.Env["KAFKA_LISTENER_NAME_"+listener+"_AWS__MSK__IAM_SASL_JAAS_CONFIG"]; !strings.Contains(got, "MskIamLoginModule") {
			t.Errorf("%s listener JAAS config %q", listener, got)
		}
	}
	if _, ok := b.Env["KAFKA_LISTENER_NAME_IAM_INTERNAL_SASL_ENABLED_MECHANISMS"]; ok {
		t.Error("KAFKA_LISTENER_NAME_IAM_INTERNAL_* configures a listener named iam.internal, which does not exist")
	}
	want := awsid.Session("arn:aws:iam::000000000000:role/my-service").AccessKeyID
	if got := b.Env["TOMATO_MSK_IAM_ALLOWED_ACCESS_KEY_IDS"]; got != want {
		t.Errorf("allowed access keys %q, want the role session %q", got, want)
	}
}

func TestKafkaPreset_WithoutAWSResourceAllowsAnyIdentity(t *testing.T) {
	stubPorts(t)
	cfg, err := Load(createTempConfig(t, `
version: 2
containers:
  kafka:
    preset: kafka
    auth: aws_msk_iam
`))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got, ok := cfg.Containers["kafka"].Env["TOMATO_MSK_IAM_ALLOWED_ACCESS_KEY_IDS"]; !ok || got != "" {
		t.Errorf("allowed access keys %q (set: %v), want empty: any identity", got, ok)
	}
}

func TestKafkaPreset_EntryOverridesWin(t *testing.T) {
	stubPorts(t)
	cfg, err := Load(createTempConfig(t, `
version: 2
containers:
  kafka:
    preset: kafka
    image: my/kafka:1
    env:
      KAFKA_AUTO_CREATE_TOPICS_ENABLE: "false"
    wait_for:
      type: port
      target: "9092"
`))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	k := cfg.Containers["kafka"]
	if k.Image != "my/kafka:1" {
		t.Errorf("image %q, want the entry's", k.Image)
	}
	if k.Env["KAFKA_AUTO_CREATE_TOPICS_ENABLE"] != "false" {
		t.Errorf("an env key the entry sets was overridden")
	}
	if k.WaitFor.Type != "port" {
		t.Errorf("wait_for %+v, want the entry's", k.WaitFor)
	}
}

// An aws_msk_iam entry with its own image still gets the plugin copied in.
func TestKafkaPreset_OwnImageStillGetsThePlugin(t *testing.T) {
	stubPorts(t)
	cfg, err := Load(createTempConfig(t, `
version: 2
containers:
  kafka:
    preset: kafka
    auth: aws_msk_iam
    image: apache/kafka:3.8.1
`))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	k := cfg.Containers["kafka"]
	if k.Image != "apache/kafka:3.8.1" || len(k.Files) != 1 || k.Files[0].Path != "/opt/kafka/libs/tomato-msk-iam.jar" {
		t.Errorf("image %q, files %+v; want the entry's image with the plugin", k.Image, k.Files)
	}
}

func TestPresets_Errors(t *testing.T) {
	stubPorts(t)
	for name, tc := range map[string]struct{ content, want string }{
		"unknown preset": {`
version: 2
containers:
  x:
    preset: mongo
`, `unknown preset "mongo"`},
		"unknown kafka auth": {`
version: 2
containers:
  x:
    preset: kafka
    auth: scram
`, `unknown kafka auth "scram"`},
		"auth without preset": {`
version: 2
containers:
  x:
    image: redis
    auth: aws_msk_iam
`, "auth needs a preset"},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := Load(createTempConfig(t, tc.content))
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error %v, want it to contain %q", err, tc.want)
			}
		})
	}
}

func TestResolvePaths(t *testing.T) {
	path := createTempConfig(t, `
version: 2
containers:
  a:
    build:
      context: ./images/a
    volumes:
      - ./fixtures:/data:ro
      - ../shared/x.sql:/docker-entrypoint-initdb.d/x.sql
      - /abs/path:/mnt
      - named-volume:/var/lib/data
`)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	dir := filepath.Dir(path)
	a := cfg.Containers["a"]
	if a.Build.Context != filepath.Join(dir, "images/a") {
		t.Errorf("build context %q", a.Build.Context)
	}
	want := []string{
		filepath.Join(dir, "fixtures") + ":/data:ro",
		filepath.Join(dir, "../shared/x.sql") + ":/docker-entrypoint-initdb.d/x.sql",
		"/abs/path:/mnt",
		"named-volume:/var/lib/data",
	}
	for i := range want {
		if a.Volumes[i] != want[i] {
			t.Errorf("volume %d: %q, want %q", i, a.Volumes[i], want[i])
		}
	}
}
