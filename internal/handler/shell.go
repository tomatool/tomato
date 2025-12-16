package handler

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/cucumber/godog"
	"github.com/tomatool/tomato/internal/config"
	"github.com/tomatool/tomato/internal/container"
)

type Shell struct {
	name      string
	config    config.Resource
	container *container.Manager

	// State
	lastExitCode int
	lastStdout   string
	lastStderr   string
	env          map[string]string
	workDir      string
	timeout      time.Duration
}

func NewShell(name string, cfg config.Resource, cm *container.Manager) (*Shell, error) {
	timeout := 30 * time.Second
	if t, ok := cfg.Options["timeout"].(string); ok {
		if d, err := time.ParseDuration(t); err == nil {
			timeout = d
		}
	}

	workDir := ""
	if w, ok := cfg.Options["workdir"].(string); ok {
		workDir = w
	}

	return &Shell{
		name:      name,
		config:    cfg,
		container: cm,
		env:       make(map[string]string),
		workDir:   workDir,
		timeout:   timeout,
	}, nil
}

func (r *Shell) Name() string { return r.name }

func (r *Shell) Init(ctx context.Context) error {
	// Load default environment from config
	if envMap, ok := r.config.Options["env"].(map[string]interface{}); ok {
		for k, v := range envMap {
			if s, ok := v.(string); ok {
				r.env[k] = s
			}
		}
	}
	return nil
}

func (r *Shell) Ready(ctx context.Context) error {
	return nil
}

func (r *Shell) Reset(ctx context.Context) error {
	r.lastExitCode = 0
	r.lastStdout = ""
	r.lastStderr = ""
	// Keep env and workDir as configured
	return nil
}

func (r *Shell) RegisterSteps(ctx *godog.ScenarioContext) {
	RegisterStepsToGodog(ctx, r.name, r.Steps())
}

// Steps returns the structured step definitions for the Shell handler
func (r *Shell) Steps() StepCategory {
	return StepCategory{
		Name:        "Shell",
		Description: "Steps for executing shell commands and scripts",
		Steps: []StepDef{
			// Setup
			{
				Group:       "Setup",
				Pattern:     `^"{resource}" env "([^"]*)" is "([^"]*)"$`,
				Description: "Set environment variable",
				Example:     `"shell" env "API_KEY" is "secret"`,
				Handler:     r.setEnvVar,
			},
			{
				Group:       "Setup",
				Pattern:     `^"{resource}" workdir is "([^"]*)"$`,
				Description: "Set working directory",
				Example:     `"shell" workdir is "/tmp/test"`,
				Handler:     r.setWorkDir,
			},

			// Execution
			{
				Group:       "Execution",
				Pattern:     `^"{resource}" runs:$`,
				Description: "Run command (docstring)",
				Example:     `"shell" runs:`,
				Handler:     r.runCommand,
			},
			{
				Group:       "Execution",
				Pattern:     `^"{resource}" runs "([^"]*)"$`,
				Description: "Run inline command",
				Example:     `"shell" runs "ls -la"`,
				Handler:     r.runCommandInline,
			},
			{
				Group:       "Execution",
				Pattern:     `^"{resource}" runs script "([^"]*)"$`,
				Description: "Run script file",
				Example:     `"shell" runs script "scripts/setup.sh"`,
				Handler:     r.runScript,
			},
			{
				Group:       "Execution",
				Pattern:     `^"{resource}" runs with timeout "([^"]*)":$`,
				Description: "Run with custom timeout",
				Example:     `"shell" runs with timeout "60s":`,
				Handler:     r.runCommandWithTimeout,
			},

			// Exit Code
			{
				Group:       "Exit Code",
				Pattern:     `^"{resource}" exit code is "(\d+)"$`,
				Description: "Assert exit code",
				Example:     `"shell" exit code is "0"`,
				Handler:     r.exitCodeShouldBe,
			},
			{
				Group:       "Exit Code",
				Pattern:     `^"{resource}" succeeds$`,
				Description: "Assert exit code 0",
				Example:     `"shell" succeeds`,
				Handler:     r.shouldSucceed,
			},
			{
				Group:       "Exit Code",
				Pattern:     `^"{resource}" fails$`,
				Description: "Assert non-zero exit code",
				Example:     `"shell" fails`,
				Handler:     r.shouldFail,
			},

			// Output
			{
				Group:       "Output",
				Pattern:     `^"{resource}" stdout contains "([^"]*)"$`,
				Description: "Assert stdout contains substring",
				Example:     `"shell" stdout contains "success"`,
				Handler:     r.stdoutShouldContain,
			},
			{
				Group:       "Output",
				Pattern:     `^"{resource}" stdout does not contain "([^"]*)"$`,
				Description: "Assert stdout doesn't contain",
				Example:     `"shell" stdout does not contain "error"`,
				Handler:     r.stdoutShouldNotContain,
			},
			{
				Group:       "Output",
				Pattern:     `^"{resource}" stdout is:$`,
				Description: "Assert exact stdout",
				Example:     `"shell" stdout is:`,
				Handler:     r.stdoutShouldBe,
			},
			{
				Group:       "Output",
				Pattern:     `^"{resource}" stdout is empty$`,
				Description: "Assert stdout empty",
				Example:     `"shell" stdout is empty`,
				Handler:     r.stdoutShouldBeEmpty,
			},
			{
				Group:       "Output",
				Pattern:     `^"{resource}" stderr contains "([^"]*)"$`,
				Description: "Assert stderr contains substring",
				Example:     `"shell" stderr contains "warning"`,
				Handler:     r.stderrShouldContain,
			},
			{
				Group:       "Output",
				Pattern:     `^"{resource}" stderr is empty$`,
				Description: "Assert stderr empty",
				Example:     `"shell" stderr is empty`,
				Handler:     r.stderrShouldBeEmpty,
			},

			// Files
			{
				Group:       "Files",
				Pattern:     `^"{resource}" file "([^"]*)" exists$`,
				Description: "Assert file exists",
				Example:     `"shell" file "output.txt" exists`,
				Handler:     r.fileShouldExist,
			},
			{
				Group:       "Files",
				Pattern:     `^"{resource}" file "([^"]*)" does not exist$`,
				Description: "Assert file doesn't exist",
				Example:     `"shell" file "temp.txt" does not exist`,
				Handler:     r.fileShouldNotExist,
			},
			{
				Group:       "Files",
				Pattern:     `^"{resource}" file "([^"]*)" contains "([^"]*)"$`,
				Description: "Assert file contains substring",
				Example:     `"shell" file "config.json" contains "database"`,
				Handler:     r.fileShouldContain,
			},
		},
	}
}

// Environment setup

func (r *Shell) setEnvVar(key, value string) error {
	r.env[key] = value
	return nil
}

func (r *Shell) setWorkDir(dir string) error {
	r.workDir = dir
	return nil
}

// Command execution

func (r *Shell) runCommand(doc *godog.DocString) error {
	return r.executeCommand(doc.Content, r.timeout)
}

func (r *Shell) runCommandInline(command string) error {
	return r.executeCommand(command, r.timeout)
}

func (r *Shell) runScript(path string) error {
	content, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("reading script: %w", err)
	}
	return r.executeCommand(string(content), r.timeout)
}

func (r *Shell) runCommandWithTimeout(timeout string, doc *godog.DocString) error {
	d, err := time.ParseDuration(timeout)
	if err != nil {
		return fmt.Errorf("invalid timeout: %w", err)
	}
	return r.executeCommand(doc.Content, d)
}

func (r *Shell) executeCommand(command string, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "sh", "-c", command)

	// Set working directory
	if r.workDir != "" {
		cmd.Dir = r.workDir
	}

	// Set environment
	cmd.Env = os.Environ()
	for k, v := range r.env {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	r.lastStdout = stdout.String()
	r.lastStderr = stderr.String()

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			r.lastExitCode = exitErr.ExitCode()
		} else if ctx.Err() == context.DeadlineExceeded {
			r.lastExitCode = -1
			return fmt.Errorf("command timed out after %s", timeout)
		} else {
			r.lastExitCode = -1
		}
	} else {
		r.lastExitCode = 0
	}

	return nil
}

// Exit code assertions

func (r *Shell) exitCodeShouldBe(expected int) error {
	if r.lastExitCode != expected {
		return fmt.Errorf("expected exit code %d, got %d\nstdout: %s\nstderr: %s",
			expected, r.lastExitCode, r.lastStdout, r.lastStderr)
	}
	return nil
}

func (r *Shell) shouldSucceed() error {
	return r.exitCodeShouldBe(0)
}

func (r *Shell) shouldFail() error {
	if r.lastExitCode == 0 {
		return fmt.Errorf("expected command to fail, but it succeeded\nstdout: %s", r.lastStdout)
	}
	return nil
}

// Output assertions

func (r *Shell) stdoutShouldContain(substr string) error {
	if !strings.Contains(r.lastStdout, substr) {
		return fmt.Errorf("stdout does not contain %q\nstdout: %s", substr, r.lastStdout)
	}
	return nil
}

func (r *Shell) stdoutShouldNotContain(substr string) error {
	if strings.Contains(r.lastStdout, substr) {
		return fmt.Errorf("stdout should not contain %q\nstdout: %s", substr, r.lastStdout)
	}
	return nil
}

func (r *Shell) stdoutShouldBe(doc *godog.DocString) error {
	expected := strings.TrimSpace(doc.Content)
	actual := strings.TrimSpace(r.lastStdout)
	if actual != expected {
		return fmt.Errorf("stdout mismatch\nexpected: %s\nactual: %s", expected, actual)
	}
	return nil
}

func (r *Shell) stdoutShouldBeEmpty() error {
	if strings.TrimSpace(r.lastStdout) != "" {
		return fmt.Errorf("expected empty stdout, got: %s", r.lastStdout)
	}
	return nil
}

func (r *Shell) stderrShouldContain(substr string) error {
	if !strings.Contains(r.lastStderr, substr) {
		return fmt.Errorf("stderr does not contain %q\nstderr: %s", substr, r.lastStderr)
	}
	return nil
}

func (r *Shell) stderrShouldBeEmpty() error {
	if strings.TrimSpace(r.lastStderr) != "" {
		return fmt.Errorf("expected empty stderr, got: %s", r.lastStderr)
	}
	return nil
}

// File assertions

func (r *Shell) fileShouldExist(path string) error {
	fullPath := path
	if r.workDir != "" && !strings.HasPrefix(path, "/") {
		fullPath = fmt.Sprintf("%s/%s", r.workDir, path)
	}
	if _, err := os.Stat(fullPath); os.IsNotExist(err) {
		return fmt.Errorf("file %q does not exist", fullPath)
	}
	return nil
}

func (r *Shell) fileShouldNotExist(path string) error {
	fullPath := path
	if r.workDir != "" && !strings.HasPrefix(path, "/") {
		fullPath = fmt.Sprintf("%s/%s", r.workDir, path)
	}
	if _, err := os.Stat(fullPath); err == nil {
		return fmt.Errorf("file %q exists but should not", fullPath)
	}
	return nil
}

func (r *Shell) fileShouldContain(path, substr string) error {
	fullPath := path
	if r.workDir != "" && !strings.HasPrefix(path, "/") {
		fullPath = fmt.Sprintf("%s/%s", r.workDir, path)
	}
	content, err := os.ReadFile(fullPath)
	if err != nil {
		return fmt.Errorf("reading file: %w", err)
	}
	if !strings.Contains(string(content), substr) {
		return fmt.Errorf("file %q does not contain %q", fullPath, substr)
	}
	return nil
}

func (r *Shell) Cleanup(ctx context.Context) error {
	return nil
}

var _ Handler = (*Shell)(nil)
