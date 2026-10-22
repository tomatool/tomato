package config

import (
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"strings"
	"testing"
)

func TestRedisPreset_Plain(t *testing.T) {
	cfg, err := Load(createTempConfig(t, `
version: 2
containers:
  redis:
    preset: redis
`))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	r := cfg.Containers["redis"]
	if r.Image != RedisImage {
		t.Errorf("image %q, want %s", r.Image, RedisImage)
	}
	if len(r.Ports) != 1 || r.Ports[0] != "6379/tcp" {
		t.Errorf("ports %v, want [6379/tcp]", r.Ports)
	}
	if r.Entrypoint != nil || r.CAFile != "" || len(r.Files) != 0 {
		t.Errorf("plain redis gets an entrypoint %q, CA %q or files %d", r.Entrypoint, r.CAFile, len(r.Files))
	}
	if r.WaitFor.Type != "log" || r.WaitFor.Target != "Ready to accept connections" {
		t.Errorf("wait_for %+v", r.WaitFor)
	}
}

func TestRedisPreset_ClusterTLS(t *testing.T) {
	stubPorts(t)
	cfg, err := Load(createTempConfig(t, `
version: 2
containers:
  redis:
    preset: redis
    cluster: true
    tls: true
resources:
  sessions-db:
    type: redis
    container: redis
`))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	r := cfg.Containers["redis"]

	// The node announces the host port, so that port is fixed before start.
	if len(r.Ports) != 1 || r.Ports[0] != "40001:6379" {
		t.Errorf("ports %v, want [40001:6379]", r.Ports)
	}
	script := r.Entrypoint[2]
	for _, want := range []string{
		"--cluster-enabled yes", "--cluster-announce-ip 127.0.0.1",
		"--cluster-announce-port 40001", "--cluster-announce-tls-port 40001",
		"--port 0 --tls-port 6379", "cluster addslotsrange 0 16383",
		"redis-cli --tls --cacert " + redisCAPath, redisReady,
	} {
		if !strings.Contains(script, want) {
			t.Errorf("entrypoint lacks %q:\n%s", want, script)
		}
	}
	if r.WaitFor.Target != redisReady {
		t.Errorf("wait_for %+v", r.WaitFor)
	}

	// The certificate is for the names a client on the host dials, and
	// chains to the CA the app and the resource are given.
	var certPEM []byte
	for _, f := range r.Files {
		if f.Path == redisCertPath {
			certPEM = f.Content
		}
	}
	block, _ := pem.Decode(certPEM)
	if block == nil {
		t.Fatalf("no certificate in %s", redisCertPath)
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		t.Fatal(err)
	}
	roots := x509.NewCertPool()
	roots.AppendCertsFromPEM(r.CA)
	for _, name := range []string{"localhost", "127.0.0.1"} {
		if _, err := cert.Verify(x509.VerifyOptions{DNSName: name, Roots: roots}); err != nil {
			t.Errorf("certificate for %s: %v", name, err)
		}
	}

	res := cfg.Resources["sessions-db"]
	if res.Options["cluster"] != true || res.Options["tls_ca_file"] != r.CAFile || r.CAFile == "" {
		t.Errorf("resource options %v, CA file %q", res.Options, r.CAFile)
	}
}

func TestRedisPreset_TLSWithoutClusterLetsDockerPickThePort(t *testing.T) {
	cfg, err := Load(createTempConfig(t, `
version: 2
containers:
  redis:
    preset: redis
    tls: true
resources:
  sessions-db:
    type: redis
    container: redis
`))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	r := cfg.Containers["redis"]
	if len(r.Ports) != 1 || r.Ports[0] != "6379/tcp" {
		t.Errorf("ports %v, want [6379/tcp]", r.Ports)
	}
	if strings.Contains(r.Entrypoint[2], "cluster") {
		t.Errorf("standalone TLS redis runs cluster commands:\n%s", r.Entrypoint[2])
	}
	if _, set := cfg.Resources["sessions-db"].Options["cluster"]; set {
		t.Errorf("standalone TLS redis makes the resource a cluster client")
	}
}

func TestRedisPreset_Refusals(t *testing.T) {
	for name, tc := range map[string]struct{ yml, want string }{
		"cluster without the preset": {`
containers:
  redis:
    image: redis:7.4
    cluster: true
`, "need preset: redis"},
		"tls on another preset": {`
containers:
  kafka:
    preset: kafka
    tls: true
`, "options of preset: redis"},
		"app in a container": {`
containers:
  redis:
    preset: redis
    tls: true
app:
  image: my-service
  port: 8080
`, "app.command"},
		"reused containers": {`
settings:
  containers:
    reuse: true
containers:
  redis:
    preset: redis
    tls: true
`, "reuse"},
		"network in front of a cluster": {`
containers:
  redis:
    preset: redis
    cluster: true
resources:
  redis-network:
    type: network
    container: redis
`, "can't stand in front of a redis cluster"},
	} {
		t.Run(name, func(t *testing.T) {
			stubPorts(t)
			_, err := Load(createTempConfig(t, "version: 2\n"+strings.TrimPrefix(tc.yml, "\n")))
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Errorf("want an error with %q, got %v", tc.want, err)
			}
		})
	}
}

// In cluster mode the node announces the host port 6379 is published on, so
// the preset owns that mapping, whatever the entry lists.
func TestRedisPreset_ClusterOwnsThePortItAnnounces(t *testing.T) {
	for _, tc := range []struct {
		ports, want, announced string
	}{
		{`["6379/tcp"]`, "[40001:6379]", "40001"},
		{`["6379"]`, "[40001:6379]", "40001"},
		{`["16379:6379"]`, "[16379:6379]", "16379"},
		{`["16379:6379/tcp", "8001/tcp"]`, "[16379:6379 8001/tcp]", "16379"},
		{`["8001/tcp"]`, "[40001:6379 8001/tcp]", "40001"},
	} {
		t.Run(tc.ports, func(t *testing.T) {
			stubPorts(t)
			cfg, err := Load(createTempConfig(t, `
version: 2
containers:
  redis:
    preset: redis
    cluster: true
    ports: `+tc.ports+`
`))
			if err != nil {
				t.Fatalf("Load: %v", err)
			}
			r := cfg.Containers["redis"]
			if got := fmt.Sprint(r.Ports); got != tc.want {
				t.Errorf("ports %s, want %s", got, tc.want)
			}
			if !strings.Contains(r.Entrypoint[2], "--cluster-announce-port "+tc.announced+" ") {
				t.Errorf("doesn't announce %s:\n%s", tc.announced, r.Entrypoint[2])
			}
		})
	}
}

// Without cluster mode nothing is announced, so the entry's port stays.
func TestRedisPreset_TLSKeepsTheEntrysPort(t *testing.T) {
	cfg, err := Load(createTempConfig(t, `
version: 2
containers:
  redis:
    preset: redis
    tls: true
    ports: ["16379:6379"]
`))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got := fmt.Sprint(cfg.Containers["redis"].Ports); got != "[16379:6379]" {
		t.Errorf("ports %s, want [16379:6379]", got)
	}
}

// The wait loops give up as soon as redis-server exits, so a bad argument
// fails the start at once instead of when the wait times out.
func TestRedisPreset_StartupStopsWhenTheServerExits(t *testing.T) {
	stubPorts(t)
	cfg, err := Load(createTempConfig(t, `
version: 2
containers:
  redis:
    preset: redis
    cluster: true
    tls: true
`))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	script := cfg.Containers["redis"].Entrypoint[2]
	for _, loop := range []string{"ping", "cluster info"} {
		for _, line := range strings.Split(script, "\n") {
			if strings.HasPrefix(line, "until ") && strings.Contains(line, loop) && !strings.Contains(line, "do alive;") {
				t.Errorf("the %s loop doesn't check the server is alive: %s", loop, line)
			}
		}
	}
}
