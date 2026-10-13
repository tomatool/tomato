package container

import (
	"context"
	"fmt"
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
func (m *Manager) StartContainer(ctx context.Context, name string) error {
	ctr, err := m.Get(name)
	if err != nil {
		return err
	}
	if err := ctr.Start(ctx); err != nil {
		return fmt.Errorf("starting container %s: %w", name, err)
	}
	return nil
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
