package container

import (
	"strconv"
	"testing"
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
