package container

import (
	"io"
	"sync"

	"github.com/rs/zerolog/log"
	"github.com/testcontainers/testcontainers-go"
)

// Container logs, streamed to the run's log directory.

// openLogConsumer creates the container's log file and a consumer that appends
// to it for as long as the container runs. It used to be a one-shot read taken
// after the wait strategy passed, which ended at that point: nothing the
// container logged later, or before failing to start, reached the file.
func (m *Manager) openLogConsumer(name string) *fileLogConsumer {
	logFile, err := m.runCtx.CreateLogFile("container-" + name)
	if err != nil {
		log.Warn().Err(err).Str("container", name).Msg("failed to create container log file")
		return nil
	}

	m.mu.Lock()
	m.logFiles[name] = logFile
	m.mu.Unlock()

	return &fileLogConsumer{w: logFile}
}

// fileLogConsumer writes a container's log stream to a file.
type fileLogConsumer struct {
	mu sync.Mutex
	w  io.Writer
}

func (c *fileLogConsumer) Accept(l testcontainers.Log) {
	c.mu.Lock()
	defer c.mu.Unlock()
	_, _ = c.w.Write(l.Content)
}
