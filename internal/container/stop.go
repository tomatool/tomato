package container

import (
	"context"
	"os"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/testcontainers/testcontainers-go"
)

// Tearing a run down, and what is deliberately left behind when containers
// are reused.

// StopAll stops all containers
func (m *Manager) StopAll(ctx context.Context, removeVolumes bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Reuse only means anything if the containers outlive the run that
	// started them. They keep their names, so the next run finds them.
	if m.settings.Reuse {
		log.Info().
			Int("containers", len(m.containers)).
			Str("scope", m.scope).
			Msg("leaving containers running for reuse; `docker rm -f $(docker ps -aq --filter label=tomato.scope=" + m.scope + ")` removes them")
		m.containers = make(map[string]testcontainers.Container)
		return nil
	}

	// Stop in reverse order
	for i := len(m.order) - 1; i >= 0; i-- {
		name := m.order[i]
		if container, ok := m.containers[name]; ok {
			log.Debug().Str("container", name).Msg("stopping container")
			if err := container.Terminate(ctx); err != nil {
				log.Warn().Err(err).Str("container", name).Msg("failed to stop container")
			}
			delete(m.containers, name)
		}
	}

	return nil
}

// Cleanup stops all containers and cleans up resources
func (m *Manager) Cleanup() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Close all log files
	m.mu.Lock()
	for _, f := range m.logFiles {
		f.Close()
	}
	m.logFiles = make(map[string]*os.File)
	m.mu.Unlock()

	if err := m.StopAll(ctx, true); err != nil {
		log.Warn().Err(err).Msg("cleanup error")
	}

	// Remove the shared network, unless the containers are staying on it.
	if m.settings.Reuse {
		m.network = nil
		return
	}
	if m.network != nil {
		log.Debug().Str("network", m.networkName).Msg("removing docker network")
		if err := m.network.Remove(ctx); err != nil {
			log.Warn().Err(err).Str("network", m.networkName).Msg("failed to remove network")
		}
		m.network = nil
	}
}
