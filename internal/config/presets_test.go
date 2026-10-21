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

func TestKafkaPreset_NetworkResourceProxiesTheAppsListenersOnly(t *testing.T) {
	stubPorts(t)
	cfg, err := Load(createTempConfig(t, `
version: 2
containers:
  kafka:
    preset: kafka
    auth: aws_msk_iam
resources:
  aws:
    type: aws
  events:
    type: kafka
    container: kafka
  kafka-network:
    type: network
    container: kafka
`))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	k := cfg.Containers["kafka"]
	// plain 40001, iam 40002; then the proxies 40003 (plain) and 40004 (iam),
	// and tomato's own listener 40005.
	wantAdvertised := "EXTERNAL://localhost:40003,INTERNAL://kafka:29092,IAM://localhost:40004,IAM_INTERNAL://kafka:29098,TOMATO://localhost:40005"
	if got := k.Env["KAFKA_ADVERTISED_LISTENERS"]; got != wantAdvertised {
		t.Errorf("advertised listeners\n got %q\nwant %q", got, wantAdvertised)
	}
	if !strings.Contains(k.Env["KAFKA_LISTENERS"], "TOMATO://:9094") || !strings.Contains(k.Env["KAFKA_LISTENER_SECURITY_PROTOCOL_MAP"], "TOMATO:PLAINTEXT") {
		t.Errorf("no listener of tomato's own: %q", k.Env["KAFKA_LISTENERS"])
	}
	if strings.Join(k.Ports, ",") != "40001:9092,40002:9098,40005:9094" {
		t.Errorf("ports %v", k.Ports)
	}
	listen, _ := cfg.Resources["kafka-network"].Options["listen"].(map[string]any)
	if listen["9092/tcp"] != 40003 || listen["9098/tcp"] != 40004 || len(listen) != 2 {
		t.Errorf("the network listens on %v", listen)
	}
	if got := cfg.Resources["events"].Brokers; len(got) != 1 || got[0] != "localhost:40005" {
		t.Errorf("tomato's kafka resource dials %v, want its own listener", got)
	}
}

func TestKafkaPreset_OneNetworkPerContainer(t *testing.T) {
	stubPorts(t)
	_, err := Load(createTempConfig(t, `
version: 2
containers:
  kafka:
    preset: kafka
resources:
  a:
    type: network
    container: kafka
  b:
    type: network
    container: kafka
`))
	if err == nil || !strings.Contains(err.Error(), "one per container") {
		t.Errorf("want one network per container, got %v", err)
	}
}

func TestNetworkResourceNeedsAContainer(t *testing.T) {
	_, err := Load(createTempConfig(t, `
version: 2
resources:
  db-network:
    type: network
`))
	if err == nil || !strings.Contains(err.Error(), "needs container") {
		t.Errorf("got %v", err)
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

func TestChromiumPreset_TunnelsTheHostPortsTheSuiteUses(t *testing.T) {
	cfg, err := Load(createTempConfig(t, `
version: 2
containers:
  chrome:
    preset: chromium
app:
  command: ./server
  port: 8080
resources:
  alice-browser:
    type: browser
    container: chrome
    options:
      base_url: http://127.0.0.1:3000
  app-client:
    type: http
    base_url: http://localhost:8080
  app-ws-client:
    type: websocket
    url: ws://localhost:7000/ws
  payments-stub:
    type: http-server
    options:
      port: 9999
  prices-stub:
    type: websocket-server
    options:
      port: 9997
  example-client:
    type: http
    base_url: https://api.example.com
  bob-browser:
    type: browser
    container: chrome
    base_url: https://localhost
`))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	c := cfg.Containers["chrome"]

	if c.Image != ChromiumImage {
		t.Errorf("image %q, want %s", c.Image, ChromiumImage)
	}
	if fmt.Sprint(c.HostAccessPorts) != "[443 3000 7000 8080 9997 9999]" {
		t.Errorf("host access ports %v", c.HostAccessPorts)
	}
	if len(c.Ports) != 1 || c.Ports[0] != "9222/tcp" {
		t.Errorf("ports %v, want [9222/tcp]", c.Ports)
	}
	if c.WaitFor.Type != "http" || c.WaitFor.Path != "/json/version" || c.WaitFor.Target != "9222/tcp" {
		t.Errorf("wait_for %+v", c.WaitFor)
	}
	if len(c.Entrypoint) != 4 || c.Entrypoint[0] != "bash" || c.Entrypoint[1] != "-c" {
		t.Fatalf("entrypoint %q", c.Entrypoint)
	}
	script := c.Entrypoint[2]
	for _, want := range []string{
		"socat TCP4-LISTEN:9222,fork,reuseaddr TCP4:127.0.0.1:9223 &",
		"exec /headless-shell/headless-shell ",
		"'--remote-debugging-port=9223'",
		"'--host-resolver-rules=MAP localhost host.testcontainers.internal'",
		`"$@"`,
	} {
		if !strings.Contains(script, want) {
			t.Errorf("entrypoint script lacks %q:\n%s", want, script)
		}
	}
}

func TestChromiumPreset_WithNothingOnTheHost(t *testing.T) {
	cfg, err := Load(createTempConfig(t, `
version: 2
containers:
  chrome:
    preset: chromium
    image: chromedp/headless-shell:latest
    command: ["--force-prefers-reduced-motion"]
    ports: ["19222:9222"]
    wait_for:
      type: port
      target: "9222"
`))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	c := cfg.Containers["chrome"]
	if c.Image != "chromedp/headless-shell:latest" {
		t.Errorf("image %q: the entry's own image wins", c.Image)
	}
	if len(c.HostAccessPorts) != 0 {
		t.Errorf("host access ports %v with nothing on the host", c.HostAccessPorts)
	}
	if strings.Contains(c.Entrypoint[2], "host-resolver-rules") {
		t.Errorf("localhost is mapped with no tunnel to map it to: %s", c.Entrypoint[2])
	}
	if fmt.Sprint(c.Ports) != "[19222:9222]" {
		t.Errorf("ports %v: the entry already publishes 9222", c.Ports)
	}
	if c.WaitFor.Type != "port" {
		t.Errorf("wait_for %+v: the entry's own wins", c.WaitFor)
	}
	if fmt.Sprint(c.Command) != "[--force-prefers-reduced-motion]" {
		t.Errorf("command %v: it becomes Chromium's flags", c.Command)
	}
}

func TestLoopbackPort(t *testing.T) {
	for in, want := range map[string]int{
		"":                       0,
		"http://localhost:8080":  8080,
		"http://127.0.0.1:3000/": 3000,
		"http://0.0.0.0:3001":    3001,
		"http://[::1]:3002":      3002,
		"ws://localhost/ws":      80,
		"wss://localhost":        443,
		"https://localhost":      443,
		"ftp://localhost":        0,
		"http://app:8080":        0,
		"http://10.0.0.1:8080":   0,
		"/relative":              0,
		"http://localhost:x":     0,
	} {
		if got := loopbackPort(in); got != want {
			t.Errorf("loopbackPort(%q) = %d, want %d", in, got, want)
		}
	}
}

func TestVeraPDFPreset(t *testing.T) {
	cfg, err := Load(createTempConfig(t, `
version: 2
containers:
  verapdf:
    preset: verapdf
`))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	v := cfg.Containers["verapdf"]
	if v.Image != VeraPDFImage || v.Build != nil {
		t.Errorf("image %q, build %+v; want %s", v.Image, v.Build, VeraPDFImage)
	}
	if len(v.Ports) != 1 || v.Ports[0] != VeraPDFPort+"/tcp" {
		t.Errorf("ports %v, want [%s/tcp]", v.Ports, VeraPDFPort)
	}
	if v.WaitFor.Type != "http" || v.WaitFor.Path != "/api/info" || v.WaitFor.Target != VeraPDFPort+"/tcp" || v.WaitFor.Timeout <= 0 {
		t.Errorf("wait_for %+v", v.WaitFor)
	}
}

func TestVeraPDFPreset_EntryOverridesWin(t *testing.T) {
	cfg, err := Load(createTempConfig(t, `
version: 2
containers:
  verapdf:
    preset: verapdf
    image: ghcr.io/example/verapdf-rest:test
    ports:
      - "18080:8080"
    wait_for:
      type: port
      target: "8080"
      timeout: 10s
`))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	v := cfg.Containers["verapdf"]
	if v.Image != "ghcr.io/example/verapdf-rest:test" {
		t.Errorf("image %q", v.Image)
	}
	if len(v.Ports) != 1 || v.Ports[0] != "18080:8080" {
		t.Errorf("a published port already covering 8080 is kept as is: %v", v.Ports)
	}
	if v.WaitFor.Type != "port" {
		t.Errorf("wait_for %+v", v.WaitFor)
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
		"verapdf has no auth": {`
version: 2
containers:
  x:
    preset: verapdf
    auth: aws_msk_iam
`, "the verapdf preset has no auth"},
		"chromium has no auth": {`
version: 2
containers:
  x:
    preset: chromium
    auth: aws_msk_iam
`, "the chromium preset has no auth"},
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
