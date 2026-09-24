package handler

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net"
	"net/url"
	"os"
	"strings"

	"github.com/rs/zerolog/log"
	"github.com/tomatool/tomato/internal/config"
)

// This file holds what resources need to run against services tomato did not
// start: shared deployed environments, cloud brokers, a database on another
// host. Every resource still works with `container:`; these helpers cover the
// explicit-connection path.

// tlsFromOptions builds a TLS config from a resource's `options.tls` map:
//
//	tls:
//	  enabled: true            # default true once the map is present
//	  ca_file: ./ca.pem        # trust this CA instead of the system roots
//	  cert_file: ./client.pem  # client certificate (mutual TLS)
//	  key_file: ./client.key
//	  server_name: broker.internal
//	  insecure_skip_verify: false
//
// It returns nil when TLS is not configured.
func tlsFromOptions(opts map[string]any) (*tls.Config, error) {
	raw, ok := opts["tls"]
	if !ok || raw == nil {
		return nil, nil
	}
	if enabled, ok := raw.(bool); ok {
		if !enabled {
			return nil, nil
		}
		return &tls.Config{MinVersion: tls.VersionTLS12}, nil
	}
	m, ok := raw.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("options.tls must be true/false or a map")
	}
	if enabled, ok := m["enabled"].(bool); ok && !enabled {
		return nil, nil
	}

	cfg := &tls.Config{MinVersion: tls.VersionTLS12}
	if name, ok := m["server_name"].(string); ok {
		cfg.ServerName = name
	}
	if skip, ok := m["insecure_skip_verify"].(bool); ok {
		cfg.InsecureSkipVerify = skip
	}
	if caFile, ok := m["ca_file"].(string); ok && caFile != "" {
		pem, err := os.ReadFile(caFile)
		if err != nil {
			return nil, fmt.Errorf("reading tls.ca_file: %w", err)
		}
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(pem) {
			return nil, fmt.Errorf("tls.ca_file %s contains no PEM certificates", caFile)
		}
		cfg.RootCAs = pool
	}
	certFile, _ := m["cert_file"].(string)
	keyFile, _ := m["key_file"].(string)
	if certFile != "" || keyFile != "" {
		if certFile == "" || keyFile == "" {
			return nil, fmt.Errorf("tls.cert_file and tls.key_file must be set together")
		}
		cert, err := tls.LoadX509KeyPair(certFile, keyFile)
		if err != nil {
			return nil, fmt.Errorf("loading client certificate: %w", err)
		}
		cfg.Certificates = []tls.Certificate{cert}
	}
	return cfg, nil
}

// isLocalHost reports whether host is this machine or a Docker host: the
// places a disposable test dependency lives.
func isLocalHost(host string) bool {
	host = strings.Trim(strings.ToLower(host), "[]")
	switch host {
	case "", "localhost", "host.docker.internal", "docker", "host.testcontainers.internal":
		return true
	}
	if ip := net.ParseIP(host); ip != nil {
		return ip.IsLoopback()
	}
	return false
}

// hostOf extracts the host from "host:port", a URL, or a bare host.
func hostOf(addr string) string {
	if strings.Contains(addr, "://") {
		if u, err := url.Parse(addr); err == nil {
			return u.Hostname()
		}
	}
	if h, _, err := net.SplitHostPort(addr); err == nil {
		return h
	}
	return addr
}

// remoteResetGuard decides whether a resource may reset (truncate, flush,
// purge, recreate) the service it points at. Resetting a disposable local
// dependency is the point of tomato; doing it to a shared remote database is
// a disaster. So for a resource that reaches a remote host, reset is off
// unless the resource says `reset: true`.
//
// It returns true when reset must be skipped, and logs why once.
func remoteResetGuard(name string, cfg config.Resource, hosts ...string) bool {
	if cfg.Container != "" || (cfg.Reset != nil && *cfg.Reset) {
		return false
	}
	for _, h := range hosts {
		if !isLocalHost(hostOf(h)) {
			log.Warn().
				Str("resource", name).
				Str("host", hostOf(h)).
				Msg("resource points at a remote host; reset is disabled (set `reset: true` on the resource to allow it)")
			return true
		}
	}
	return false
}
