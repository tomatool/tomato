package container

import (
	"context"
	"fmt"
	"strings"

	mobyclient "github.com/moby/moby/client"
	"github.com/rs/zerolog/log"
	"github.com/testcontainers/testcontainers-go"
)

// The per-run network. Containers reach each other by container name on it,
// and a reused run adopts the one already there.

// CreateNetwork creates the shared Docker network for all containers
func (m *Manager) CreateNetwork(ctx context.Context) error {
	if m.network != nil {
		return nil // Already created
	}

	log.Debug().Str("network", m.networkName).Msg("creating docker network")

	// network.New() hard-codes a uuid for the name, so the network goes
	// through the request type directly: an unreadable network is no use when
	// you are looking at `docker network ls` wondering what a run left behind.
	labels := testcontainers.GenericLabels()
	for k, v := range m.labels("network") {
		labels[k] = v
	}

	// With reuse the previous run's network is still there; adopt it instead
	// of failing on the duplicate name.
	if m.settings.Reuse {
		existing, err := m.findNetwork(ctx, m.networkName)
		if err != nil {
			return fmt.Errorf("looking for an existing network: %w", err)
		}
		if existing {
			log.Debug().Str("network", m.networkName).Msg("reusing the existing docker network")
			m.networkReady = true
			return nil
		}
	}

	//nolint:staticcheck // the named-network request is the only way to set a name
	net, err := testcontainers.GenericNetwork(ctx, testcontainers.GenericNetworkRequest{
		NetworkRequest: testcontainers.NetworkRequest{
			Name:   m.networkName,
			Driver: "bridge",
			Labels: labels,
		},
	})
	if err != nil {
		return fmt.Errorf("creating network: %w", err)
	}

	dn, ok := net.(*testcontainers.DockerNetwork)
	if !ok {
		return fmt.Errorf("creating network: unexpected network type %T", net)
	}
	m.network = dn
	m.networkName = dn.Name
	m.networkReady = true

	log.Debug().Str("network", m.networkName).Msg("docker network created")
	return nil
}

// findNetwork reports whether a network of that exact name already exists.
func (m *Manager) findNetwork(ctx context.Context, name string) (bool, error) {
	cli, err := testcontainers.NewDockerClientWithOpts(ctx)
	if err != nil {
		return false, err
	}
	defer cli.Close()

	// Listing all and matching exactly avoids the filter API, and a name
	// filter is a substring match anyway — "tomato-ab-net" would match
	// "tomato-ab-net-2".
	nets, err := cli.NetworkList(ctx, mobyclient.NetworkListOptions{})
	if err != nil {
		return false, err
	}
	for _, n := range nets.Items {
		if n.Name == name {
			return true, nil
		}
	}
	return false, nil
}

// GetNetworkName returns the shared network name
func (m *Manager) GetNetworkName() string {
	return m.networkName
}

// GetInternalAddress returns the internal Docker DNS address for a container
// This is used by containers to communicate with each other within the network
func (m *Manager) GetInternalAddress(name, port string) string {
	// Strip /tcp or /udp suffix if present
	if idx := strings.Index(port, "/"); idx > 0 {
		port = port[:idx]
	}
	return fmt.Sprintf("%s:%s", name, port)
}
