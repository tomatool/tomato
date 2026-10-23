package container

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"time"

	mobycontainer "github.com/moby/moby/api/types/container"
	"github.com/moby/moby/client"
	"github.com/testcontainers/testcontainers-go"
)

// Stats is one reading of a container's resource use, as Docker reports it
// about once a second.
type Stats struct {
	At time.Time
	// CPU is the container's CPU time since it started, in nanoseconds.
	CPU uint64
	// Memory is what the container uses, without the page cache it could
	// give back, the way `docker stats` counts it.
	Memory uint64
	// NetworkRx and NetworkTx are bytes since it started, over all interfaces.
	NetworkRx, NetworkTx uint64
}

// StreamStats calls fn with every reading Docker reports for a running
// container, until ctx is done or the container stops. A container a
// scenario restarts is a new stream: the caller calls it again.
func (m *Manager) StreamStats(ctx context.Context, name string, fn func(Stats)) error {
	ctr, err := m.Get(name)
	if err != nil {
		return err
	}
	cli, err := testcontainers.NewDockerClientWithOpts(ctx)
	if err != nil {
		return fmt.Errorf("connecting to Docker: %w", err)
	}
	defer cli.Close()
	res, err := cli.ContainerStats(ctx, ctr.GetContainerID(), client.ContainerStatsOptions{Stream: true})
	if err != nil {
		return fmt.Errorf("container %s: stats: %w", name, err)
	}
	defer res.Body.Close()
	dec := json.NewDecoder(res.Body)
	for {
		var s mobycontainer.StatsResponse
		if err := dec.Decode(&s); err != nil {
			if err == io.EOF || ctx.Err() != nil {
				return nil
			}
			return fmt.Errorf("container %s: reading stats: %w", name, err)
		}
		fn(statsOf(s))
	}
}

func statsOf(s mobycontainer.StatsResponse) Stats {
	out := Stats{At: s.Read, CPU: s.CPUStats.CPUUsage.TotalUsage, Memory: s.MemoryStats.Usage}
	// cgroup v2 calls the page cache inactive_file, v1 total_inactive_file.
	for _, key := range []string{"inactive_file", "total_inactive_file"} {
		if cache, ok := s.MemoryStats.Stats[key]; ok && cache < out.Memory {
			out.Memory -= cache
			break
		}
	}
	for _, n := range s.Networks {
		out.NetworkRx += n.RxBytes
		out.NetworkTx += n.TxBytes
	}
	if out.At.IsZero() {
		out.At = time.Now()
	}
	return out
}

// Names returns the configured containers, in start order.
func (m *Manager) Names() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]string, len(m.order))
	copy(out, m.order)
	return out
}
