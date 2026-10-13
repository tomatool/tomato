package container

import (
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/moby/moby/client"
	"github.com/testcontainers/testcontainers-go"
)

// pinnableHostPort returns a free host port to bind a dynamic container port
// to, when Docker runs on this machine. A remote Docker host has its own free
// ports, which this machine cannot see, so there the port is left to Docker.
func pinnableHostPort() (string, bool) {
	if host := os.Getenv("DOCKER_HOST"); host != "" && !strings.HasPrefix(host, "unix://") && !strings.HasPrefix(host, "npipe://") {
		return "", false
	}
	l, err := net.Listen("tcp", ":0")
	if err != nil {
		return "", false
	}
	defer l.Close()
	return strconv.Itoa(l.Addr().(*net.TCPAddr).Port), true
}

// StopContainer stops a running container, as when the service it runs goes
// down. Its state and its host ports are kept for StartContainer.
func (m *Manager) StopContainer(ctx context.Context, name string) error {
	ctr, err := m.Get(name)
	if err != nil {
		return err
	}
	timeout := 10 * time.Second
	if err := ctr.Stop(ctx, &timeout); err != nil {
		return fmt.Errorf("stopping container %s: %w", name, err)
	}
	return nil
}

// StartContainer starts a stopped container again and waits until its wait
// strategy passes, as on the first start.
//
// A log wait needs care: a container's log keeps what it printed before it
// was stopped, so the line a log wait looks for is already there and the
// wait would pass at once, before the restarted service is up. For a log
// wait, StartContainer waits for one more occurrence than before the start.
func (m *Manager) StartContainer(ctx context.Context, name string) error {
	ctr, err := m.Get(name)
	if err != nil {
		return err
	}
	ws := m.configs[name].WaitFor
	logWait := ws.Type == "log" && ws.Target != ""
	before := 0
	if logWait {
		if before, err = logOccurrences(ctx, ctr, ws.Target); err != nil {
			return fmt.Errorf("reading the logs of container %s: %w", name, err)
		}
	}
	if err := ctr.Start(ctx); err != nil {
		return fmt.Errorf("starting container %s: %w", name, err)
	}
	if !logWait {
		return nil
	}
	timeout := ws.Timeout
	if timeout == 0 {
		timeout = 60 * time.Second
	}
	deadline := time.Now().Add(timeout)
	for {
		n, err := logOccurrences(ctx, ctr, ws.Target)
		if err == nil && n > before {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("container %s: %q did not appear in its log again within %s of starting", name, ws.Target, timeout)
		}
		time.Sleep(200 * time.Millisecond)
	}
}

// logOccurrences counts how often target appears in a container's log so far.
func logOccurrences(ctx context.Context, ctr testcontainers.Container, target string) (int, error) {
	r, err := ctr.Logs(ctx)
	if err != nil {
		return 0, err
	}
	defer r.Close()
	return countOccurrences(r, target)
}

func countOccurrences(r io.Reader, target string) (int, error) {
	b, err := io.ReadAll(r)
	if err != nil {
		return 0, err
	}
	return strings.Count(string(b), target), nil
}

// PauseContainer freezes a container's processes: connections stay open and
// nothing answers, as with a hung service or a network partition.
func (m *Manager) PauseContainer(ctx context.Context, name string) error {
	return m.withDocker(ctx, name, func(cli *testcontainers.DockerClient, id string) error {
		_, err := cli.ContainerPause(ctx, id, client.ContainerPauseOptions{})
		return err
	})
}

// UnpauseContainer resumes a paused container.
func (m *Manager) UnpauseContainer(ctx context.Context, name string) error {
	return m.withDocker(ctx, name, func(cli *testcontainers.DockerClient, id string) error {
		_, err := cli.ContainerUnpause(ctx, id, client.ContainerUnpauseOptions{})
		return err
	})
}

func (m *Manager) withDocker(ctx context.Context, name string, fn func(*testcontainers.DockerClient, string) error) error {
	ctr, err := m.Get(name)
	if err != nil {
		return err
	}
	cli, err := testcontainers.NewDockerClientWithOpts(ctx)
	if err != nil {
		return fmt.Errorf("connecting to Docker: %w", err)
	}
	defer cli.Close()
	if err := fn(cli, ctr.GetContainerID()); err != nil {
		return fmt.Errorf("container %s: %w", name, err)
	}
	return nil
}
