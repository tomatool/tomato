package shell

import (
	"errors"
	"os/exec"
	"sync"

	"github.com/tomatool/tomato/config"
)

type Shell struct {
	stdout string
	stderr string
}

func New(cfg *config.Resource) (*Shell, error) {
	return &Shell{}, nil
}

func (s *Shell) Ready() error {
	return nil
}
func (s *Shell) Reset() error {
	s.stderr = ""
	s.stdout = ""
	return nil
}
func (s *Shell) Exec(command string, arguments ...string) error {
	cmd := exec.Command(command, arguments...)
	cmd.Stdout = newWriter(&s.stdout)
	cmd.Stderr = newWriter(&s.stderr)
	return cmd.Run()
}
func (s *Shell) Stdout() (string, error) {
	defer func() { s.stdout = "" }()

	if s.stdout == "" {
		return "", errors.New("shell: expecting something from an empty stdout value")
	}

	return s.stdout, nil
}
func (s *Shell) Stderr() (string, error) {
	defer func() { s.stderr = "" }()

	if s.stderr == "" {
		return "", errors.New("shell: expecting something from an empty stderr value")
	}

	return s.stderr, nil
}

type writer struct {
	mtx   sync.Mutex
	value *string
}

func newWriter(target *string) *writer {
	return &writer{value: target}
}

func (c *writer) Write(b []byte) (int, error) {
	c.mtx.Lock()
	defer c.mtx.Unlock()

	if c.value == nil {
		return 0, errors.New("value is not initiated")
	}

	v := *c.value
	v = v + string(b)
	*c.value = v

	return len(b), nil
}
