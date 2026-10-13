package container

import (
	"strconv"
	"strings"
	"testing"
	"testing/iotest"
)

// With a local Docker tomato binds dynamic ports itself, so a container keeps
// its address across a stop and start; a remote Docker host's free ports are
// not ours to pick.
func TestPinnableHostPort(t *testing.T) {
	for _, host := range []string{"", "unix:///var/run/docker.sock", "npipe:////./pipe/docker_engine"} {
		t.Setenv("DOCKER_HOST", host)
		port, ok := pinnableHostPort()
		if !ok {
			t.Errorf("DOCKER_HOST=%q: want a pinned port", host)
			continue
		}
		if n, err := strconv.Atoi(port); err != nil || n <= 0 {
			t.Errorf("DOCKER_HOST=%q: port %q", host, port)
		}
	}
	t.Setenv("DOCKER_HOST", "tcp://docker.example:2376")
	if _, ok := pinnableHostPort(); ok {
		t.Error("a remote Docker host must keep Docker's own port allocation")
	}
}

// A restarted container's log still holds the line its first start printed;
// the restart waits for the count to go up, not for the line to exist.
func TestCountOccurrences(t *testing.T) {
	log := "starting\nReady to accept connections tcp\nshutting down\nstarting\nReady to accept connections tcp\n"
	n, err := countOccurrences(strings.NewReader(log), "Ready to accept connections")
	if err != nil || n != 2 {
		t.Fatalf("got %d, %v; want 2", n, err)
	}
	if n, _ := countOccurrences(strings.NewReader(""), "x"); n != 0 {
		t.Errorf("empty log: %d", n)
	}
	if _, err := countOccurrences(iotest.ErrReader(iotest.ErrTimeout), "x"); err == nil {
		t.Error("a read error is reported")
	}
}
