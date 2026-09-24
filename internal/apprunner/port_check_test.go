package apprunner

import (
	"context"
	"net"
	"strings"
	"testing"

	"github.com/tomatool/tomato/internal/config"
)

func TestCheckPortFree(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to create listener: %v", err)
	}
	busyPort := listener.Addr().(*net.TCPAddr).Port

	if err := checkPortFree("127.0.0.1", busyPort); err == nil {
		t.Error("expected an error for a port that is already listening")
	} else if !strings.Contains(err.Error(), "already in use") {
		t.Errorf("error should say the port is in use, got %q", err)
	}

	listener.Close()
	if err := checkPortFree("127.0.0.1", busyPort); err != nil {
		t.Errorf("expected a closed port to be free, got %v", err)
	}

	if err := checkPortFree("127.0.0.1", 0); err != nil {
		t.Errorf("port 0 (not configured) must pass, got %v", err)
	}
}

func TestStartCommandRefusesBusyPort(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to create listener: %v", err)
	}
	defer listener.Close()
	busyPort := listener.Addr().(*net.TCPAddr).Port

	runner := NewRunner(config.AppConfig{Command: "sleep 30", Port: busyPort}, nil)
	err = runner.startCommand(context.Background())
	if err == nil {
		runner.Stop()
		t.Fatal("expected startCommand to fail when the app port is taken")
	}
	if runner.cmd != nil {
		t.Error("the app process must not be started when the port is taken")
	}
}
