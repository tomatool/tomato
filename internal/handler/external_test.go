package handler

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/IBM/sarama"
	"github.com/tomatool/tomato/internal/config"
)

// writeTestCert writes a self-signed certificate and key, returning the paths.
func writeTestCert(t *testing.T) (certFile, keyFile string) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "tomato-test"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		IsCA:         true,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	certFile = filepath.Join(dir, "cert.pem")
	keyFile = filepath.Join(dir, "key.pem")
	os.WriteFile(certFile, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0o600)
	os.WriteFile(keyFile, pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER}), 0o600)
	return certFile, keyFile
}

func TestTLSFromOptions(t *testing.T) {
	certFile, keyFile := writeTestCert(t)

	if cfg, err := tlsFromOptions(map[string]any{}); cfg != nil || err != nil {
		t.Errorf("no tls option: got %v, %v", cfg, err)
	}
	if cfg, err := tlsFromOptions(map[string]any{"tls": false}); cfg != nil || err != nil {
		t.Errorf("tls: false: got %v, %v", cfg, err)
	}
	if cfg, err := tlsFromOptions(map[string]any{"tls": true}); cfg == nil || err != nil {
		t.Errorf("tls: true should enable TLS with system roots, got %v, %v", cfg, err)
	}

	cfg, err := tlsFromOptions(map[string]any{"tls": map[string]any{
		"ca_file":     certFile,
		"cert_file":   certFile,
		"key_file":    keyFile,
		"server_name": "broker.internal",
	}})
	if err != nil {
		t.Fatalf("full tls map: %v", err)
	}
	if cfg.RootCAs == nil || len(cfg.Certificates) != 1 || cfg.ServerName != "broker.internal" {
		t.Errorf("tls config not populated: %+v", cfg)
	}

	if _, err := tlsFromOptions(map[string]any{"tls": map[string]any{"cert_file": certFile}}); err == nil {
		t.Error("cert_file without key_file should fail")
	}
	if _, err := tlsFromOptions(map[string]any{"tls": map[string]any{"ca_file": keyFile}}); err == nil {
		t.Error("a ca_file without certificates should fail")
	}
}

func TestIsLocalHostAndHostOf(t *testing.T) {
	local := []string{"localhost", "127.0.0.1", "::1", "[::1]", "host.docker.internal", ""}
	for _, h := range local {
		if !isLocalHost(h) {
			t.Errorf("isLocalHost(%q) = false, want true", h)
		}
	}
	remote := []string{"db.staging.internal", "10.1.2.3", "kafka-1.example.com"}
	for _, h := range remote {
		if isLocalHost(h) {
			t.Errorf("isLocalHost(%q) = true, want false", h)
		}
	}

	cases := map[string]string{
		"localhost:9092":                         "localhost",
		"postgres://u:p@db.internal:5432/app":    "db.internal",
		"amqps://user:pass@mq.example.com/vhost": "mq.example.com",
		"https://s3.eu-central-1.amazonaws.com":  "s3.eu-central-1.amazonaws.com",
		"redis.internal":                         "redis.internal",
	}
	for in, want := range cases {
		if got := hostOf(in); got != want {
			t.Errorf("hostOf(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestRemoteResetGuard(t *testing.T) {
	yes := true
	tests := []struct {
		name  string
		cfg   config.Resource
		hosts []string
		skip  bool
	}{
		{name: "managed container", cfg: config.Resource{Container: "db"}, hosts: []string{"db.internal"}, skip: false},
		{name: "local explicit host", hosts: []string{"localhost:5432"}, skip: false},
		{name: "remote host", hosts: []string{"db.staging.internal:5432"}, skip: true},
		{name: "any remote broker", hosts: []string{"localhost:9092", "kafka-2.internal:9092"}, skip: true},
		{name: "remote with reset: true", cfg: config.Resource{Reset: &yes}, hosts: []string{"db.staging.internal"}, skip: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := remoteResetGuard("res", tt.cfg, tt.hosts...); got != tt.skip {
				t.Errorf("remoteResetGuard = %v, want %v", got, tt.skip)
			}
		})
	}
}

func TestPostgresDSN(t *testing.T) {
	ctx := context.Background()

	r := &Postgres{name: "db", config: config.Resource{URL: "postgres://app:secret@db.internal:5432/app?sslmode=verify-full"}}
	dsn, host, err := r.dsn(ctx)
	if err != nil || host != "db.internal" || !strings.Contains(dsn, "verify-full") {
		t.Errorf("url: dsn=%q host=%q err=%v", dsn, host, err)
	}

	r = &Postgres{name: "db", config: config.Resource{Options: map[string]any{"dsn": "host=10.0.0.5 dbname=app"}}}
	if _, host, _ := r.dsn(ctx); host != "10.0.0.5" {
		t.Errorf("keyword dsn host = %q, want 10.0.0.5", host)
	}

	r = &Postgres{name: "db", config: config.Resource{Database: "app", Options: map[string]any{
		"host": "db.internal", "port": 6432, "user": "app", "password": "p w'd",
	}}}
	dsn, host, err = r.dsn(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if host != "db.internal" || !strings.Contains(dsn, "port=6432") || !strings.Contains(dsn, "sslmode=require") {
		t.Errorf("remote host should default to sslmode=require: %q", dsn)
	}
	if !strings.Contains(dsn, `password='p w\'d'`) {
		t.Errorf("password should be quoted: %q", dsn)
	}

	r = &Postgres{name: "db", config: config.Resource{Options: map[string]any{"host": "localhost"}}}
	if dsn, _, _ := r.dsn(ctx); !strings.Contains(dsn, "sslmode=disable") {
		t.Errorf("localhost should default to sslmode=disable: %q", dsn)
	}

	r = &Postgres{name: "db", config: config.Resource{Options: map[string]any{"host": "db.internal", "sslmode": "verify-ca", "sslrootcert": "/ca.pem"}}}
	if dsn, _, _ := r.dsn(ctx); !strings.Contains(dsn, "sslmode=verify-ca") || !strings.Contains(dsn, "sslrootcert=/ca.pem") {
		t.Errorf("sslmode/sslrootcert should pass through: %q", dsn)
	}

	r = &Postgres{name: "db"}
	if _, _, err := r.dsn(ctx); err == nil || !strings.Contains(err.Error(), "options.host") {
		t.Errorf("no connection settings should explain the options, got %v", err)
	}
}

func TestRedisClientOptions(t *testing.T) {
	ctx := context.Background()

	r := &Redis{name: "cache", config: config.Resource{URL: "rediss://user:secret@redis.internal:6380/2"}}
	opts, err := r.clientOptions(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if opts.Addr != "redis.internal:6380" || opts.DB != 2 || opts.TLSConfig == nil || opts.Password != "secret" {
		t.Errorf("rediss url not applied: %+v", opts)
	}

	r = &Redis{name: "cache", config: config.Resource{Options: map[string]any{"host": "localhost", "port": 6390, "password": "pw", "tls": true}}}
	opts, err = r.clientOptions(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if opts.Addr != "localhost:6390" || opts.Password != "pw" || opts.TLSConfig == nil {
		t.Errorf("host options not applied: %+v", opts)
	}

	r = &Redis{name: "cache"}
	if _, err := r.clientOptions(ctx); err == nil {
		t.Error("no connection settings should fail")
	}
}

func TestApplyKafkaSecurity(t *testing.T) {
	cfg := sarama.NewConfig()
	if err := applyKafkaSecurity(cfg, map[string]any{}); err != nil || cfg.Net.SASL.Enable || cfg.Net.TLS.Enable {
		t.Errorf("no options should leave security off, err=%v", err)
	}

	cfg = sarama.NewConfig()
	err := applyKafkaSecurity(cfg, map[string]any{
		"tls":  true,
		"sasl": map[string]any{"mechanism": "scram-sha-512", "user": "u", "password": "p"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.Net.TLS.Enable || !cfg.Net.SASL.Enable || cfg.Net.SASL.Mechanism != sarama.SASLTypeSCRAMSHA512 {
		t.Errorf("tls/scram not applied: tls=%v sasl=%v mech=%v", cfg.Net.TLS.Enable, cfg.Net.SASL.Enable, cfg.Net.SASL.Mechanism)
	}
	client := cfg.Net.SASL.SCRAMClientGeneratorFunc()
	if err := client.Begin("u", "p", ""); err != nil {
		t.Errorf("scram client should begin: %v", err)
	}
	if first, err := client.Step(""); err != nil || !strings.HasPrefix(first, "n,,n=u,r=") {
		t.Errorf("scram client-first message = %q, %v", first, err)
	}

	cfg = sarama.NewConfig()
	if err := applyKafkaSecurity(cfg, map[string]any{"sasl": map[string]any{"user": "u", "password": "p"}}); err != nil || cfg.Net.SASL.Mechanism != sarama.SASLTypePlaintext {
		t.Errorf("mechanism should default to PLAIN, got %v, %v", cfg.Net.SASL.Mechanism, err)
	}
	if err := applyKafkaSecurity(sarama.NewConfig(), map[string]any{"sasl": map[string]any{"mechanism": "GSSAPI", "user": "u"}}); err == nil {
		t.Error("unsupported mechanism should fail")
	}
	if err := applyKafkaSecurity(sarama.NewConfig(), map[string]any{"sasl": map[string]any{"password": "p"}}); err == nil {
		t.Error("missing user should fail")
	}
}
