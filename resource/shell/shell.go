package shell

import (
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"sync"
	"syscall"

	"github.com/tomatool/tomato/config"
)

type Shell struct {
	prefix []string

	stdout   string
	stderr   string
	exitCode int
}

func New(cfg *config.Resource) (*Shell, error) {
	sh := &Shell{}
	if prefix, ok := cfg.Options["prefix"]; ok {
		sh.prefix = strings.Split(prefix, " ")
	}
	return sh, nil
}

func (s *Shell) Open() error {
	return nil
}

func (s *Shell) Ready() error {
	return nil
}
func (s *Shell) Reset() error {
	s.stderr = ""
	s.stdout = ""
	return nil
}

func (s *Shell) Close() error {
	return nil
}

func (s *Shell) Exec(command string, arguments ...string) error {
	arguments = append([]string{command}, arguments...)
	if len(s.prefix) > 0 {
		arguments = append(s.prefix, arguments...)
	}

	cmd := exec.Command(arguments[0], arguments[1:]...)

	cmd.Stdout = newWriter(&s.stdout)
	cmd.Stderr = newWriter(&s.stderr)

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("shell: %v\nstdout: %s\nstderr: %s", err, s.stdout, s.stderr)
	}

	if err := cmd.Wait(); err != nil {
		if exiterr, ok := err.(*exec.ExitError); ok {
			if status, ok := exiterr.Sys().(syscall.WaitStatus); ok {
				s.exitCode = status.ExitStatus()
			}
		} else {
			return fmt.Errorf("shell: %v\nstdout: %s\nstderr: %s", err, s.stdout, s.stderr)
		}
	}
	return nil
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
func (s *Shell) ExitCode() (int, error) {
	defer func() { s.exitCode = 0 }()

	if s.exitCode == 0 {
		return s.exitCode, nil
	}

	return s.exitCode, nil
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
