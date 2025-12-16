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
			// Environment setup
			{
				Pattern:     `^I set "{resource}" environment variable "([^"]*)" to "([^"]*)"$`,
				Description: "Sets an environment variable for commands",
				Example:     `I set "{resource}" environment variable "API_KEY" to "secret"`,
				Handler:     r.setEnvVar,
			},
			{
				Pattern:     `^I set "{resource}" working directory to "([^"]*)"$`,
				Description: "Sets the working directory for commands",
				Example:     `I set "{resource}" working directory to "/tmp/test"`,
				Handler:     r.setWorkDir,
			},

			// Command execution
			{
				Pattern:     `^I run command on "{resource}":$`,
				Description: "Runs a shell command",
				Example:     "I run command on \"{resource}\":\n  \"\"\"\n  echo \"Hello World\"\n  \"\"\"",
				Handler:     r.runCommand,
			},
			{
				Pattern:     `^I run "([^"]*)" on "{resource}"$`,
				Description: "Runs a short inline command",
				Example:     `I run "ls -la" on "{resource}"`,
				Handler:     r.runCommandInline,
			},
			{
				Pattern:     `^I run script "([^"]*)" on "{resource}"$`,
				Description: "Runs a script file",
				Example:     `I run script "scripts/setup.sh" on "{resource}"`,
				Handler:     r.runScript,
			},
			{
				Pattern:     `^I run command on "{resource}" with timeout "([^"]*)":$`,
				Description: "Runs a command with custom timeout",
				Example:     "I run command on \"{resource}\" with timeout \"60s\":\n  \"\"\"\n  ./long-running-task\n  \"\"\"",
				Handler:     r.runCommandWithTimeout,
			},

			// Exit code assertions
			{
				Pattern:     `^"{resource}" exit code should be "(\d+)"$`,
				Description: "Asserts the command exit code",
				Example:     `"{resource}" exit code should be "0"`,
				Handler:     r.exitCodeShouldBe,
			},
			{
				Pattern:     `^"{resource}" should succeed$`,
				Description: "Asserts the command exited with code 0",
				Example:     `"{resource}" should succeed`,
				Handler:     r.shouldSucceed,
			},
			{
				Pattern:     `^"{resource}" should fail$`,
				Description: "Asserts the command exited with non-zero code",
				Example:     `"{resource}" should fail`,
				Handler:     r.shouldFail,
			},

			// Output assertions
			{
				Pattern:     `^"{resource}" stdout should contain "([^"]*)"$`,
				Description: "Asserts stdout contains substring",
				Example:     `"{resource}" stdout should contain "success"`,
				Handler:     r.stdoutShouldContain,
			},
			{
				Pattern:     `^"{resource}" stdout should not contain "([^"]*)"$`,
				Description: "Asserts stdout does not contain substring",
				Example:     `"{resource}" stdout should not contain "error"`,
				Handler:     r.stdoutShouldNotContain,
			},
			{
				Pattern:     `^"{resource}" stdout should be:$`,
				Description: "Asserts stdout matches exactly",
				Example:     "\"{resource}\" stdout should be:\n  \"\"\"\n  Hello World\n  \"\"\"",
				Handler:     r.stdoutShouldBe,
			},
			{
				Pattern:     `^"{resource}" stdout should be empty$`,
				Description: "Asserts stdout is empty",
				Example:     `"{resource}" stdout should be empty`,
				Handler:     r.stdoutShouldBeEmpty,
			},
			{
				Pattern:     `^"{resource}" stderr should contain "([^"]*)"$`,
				Description: "Asserts stderr contains substring",
				Example:     `"{resource}" stderr should contain "warning"`,
				Handler:     r.stderrShouldContain,
			},
			{
				Pattern:     `^"{resource}" stderr should be empty$`,
				Description: "Asserts stderr is empty",
				Example:     `"{resource}" stderr should be empty`,
				Handler:     r.stderrShouldBeEmpty,
			},

			// File assertions
			{
				Pattern:     `^"{resource}" file "([^"]*)" should exist$`,
				Description: "Asserts a file exists",
				Example:     `"{resource}" file "output.txt" should exist`,
				Handler:     r.fileShouldExist,
			},
			{
				Pattern:     `^"{resource}" file "([^"]*)" should not exist$`,
				Description: "Asserts a file does not exist",
				Example:     `"{resource}" file "temp.txt" should not exist`,
				Handler:     r.fileShouldNotExist,
			},
			{
				Pattern:     `^"{resource}" file "([^"]*)" should contain "([^"]*)"$`,
				Description: "Asserts a file contains substring",
				Example:     `"{resource}" file "config.json" should contain "database"`,
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
