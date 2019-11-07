package shell

import (
	"fmt"
	"strings"

	"github.com/tomatool/tomato/resource"
)

type Resource interface {
	resource.Resource

	Exec(command string, arguments ...string) error
	Stdout() (string, error)
	Stderr() (string, error)
	ExitCode() (int, error)
}

type Handler struct {
	r map[string]Resource
}

func New(r map[string]Resource) *Handler {
	return &Handler{r}
}

func (h *Handler) execCommand(resourceName, command string) error {
	r, ok := h.r[resourceName]
	if !ok {
		return fmt.Errorf("%s not found", resourceName)
	}

	cmds := strings.Split(command, " ")
	return r.Exec(cmds[0], cmds[1:]...)
}

func (h *Handler) checkStdoutContains(resourceName, message string) error {
	r, ok := h.r[resourceName]
	if !ok {
		return fmt.Errorf("%s not found", resourceName)
	}

	stdout, err := r.Stdout()
	if err != nil {
		return err
	}

	if !strings.Contains(stdout, message) {
		return fmt.Errorf("stdout is not contains `%s`\nstdout actual output:%s", message, stdout)
	}

	return nil
}

func (h *Handler) checkStderrContains(resourceName, message string) error {
	r, ok := h.r[resourceName]
	if !ok {
		return fmt.Errorf("%s not found", resourceName)
	}

	stderr, err := r.Stderr()
	if err != nil {
		return err
	}

	if !strings.Contains(stderr, message) {
		return fmt.Errorf("stderr is not contains `%s`\nstdout actual output:%s", message, stderr)
	}

	return nil
}

func (h *Handler) checkStdoutNotContains(resourceName, message string) error {
	r, ok := h.r[resourceName]
	if !ok {
		return fmt.Errorf("%s not found", resourceName)
	}

	stdout, err := r.Stdout()
	if err != nil {
		return err
	}

	if strings.Contains(stdout, message) {
		return fmt.Errorf("stdout is contains `%s`\nstdout actual output:%s", message, stdout)
	}

	return nil
}

func (h *Handler) checkStderrNotContains(resourceName, message string) error {
	r, ok := h.r[resourceName]
	if !ok {
		return fmt.Errorf("%s not found", resourceName)
	}

	stderr, err := r.Stderr()
	if err != nil {
		return err
	}

	if strings.Contains(stderr, message) {
		return fmt.Errorf("stderr is contains `%s`\nstdout actual output:%s", message, stderr)
	}

	return nil
}

func (h *Handler) checkExitCodeEqual(resourceName string, expectedExitCode int) error {
	r, ok := h.r[resourceName]
	if !ok {
		return fmt.Errorf("%s not found", resourceName)
	}

	exitCode, err := r.ExitCode()
	if err != nil {
		return err
	}

	if exitCode != expectedExitCode {
		return fmt.Errorf("expecting exit code to be %d, got %d", expectedExitCode, exitCode)
	}

	return nil
}

func (h *Handler) checkExitCodeNotEqual(resourceName string, unexpectedExitCode int) error {
	r, ok := h.r[resourceName]
	if !ok {
		return fmt.Errorf("%s not found", resourceName)
	}

	exitCode, err := r.ExitCode()
	if err != nil {
		return err
	}

	if exitCode == unexpectedExitCode {
		return fmt.Errorf("expecting exit code not to be %d, got %d", unexpectedExitCode, exitCode)
	}

	return nil
}
