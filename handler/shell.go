package handler

import (
	"fmt"
	"strings"
)

func (h *Handler) execCommand(resourceName, command string) error {
	r, err := h.resource.GetShell(resourceName)
	if err != nil {
		return err
	}

	cmds := strings.Split(command, " ")
	return r.Exec(cmds[0], cmds[1:]...)
}

func (h *Handler) checkStdoutContains(resourceName, message string) error {
	r, err := h.resource.GetShell(resourceName)
	if err != nil {
		return err
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
	r, err := h.resource.GetShell(resourceName)
	if err != nil {
		return err
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
	r, err := h.resource.GetShell(resourceName)
	if err != nil {
		return err
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
	r, err := h.resource.GetShell(resourceName)
	if err != nil {
		return err
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
