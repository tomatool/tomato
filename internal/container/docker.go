package container

import (
	"fmt"
	"os/exec"
	"runtime"
)

// Whether this machine can run containers at all, and what to tell someone
// whose Docker is not running.

// ErrDockerNotRunning is returned when Docker daemon is not available
var ErrDockerNotRunning = fmt.Errorf("docker is not running")

// CheckDockerAvailable verifies that Docker daemon is running and accessible
func CheckDockerAvailable() error {
	cmd := exec.Command("docker", "info")
	if err := cmd.Run(); err != nil {
		return &DockerNotRunningError{}
	}
	return nil
}

// DockerNotRunningError provides helpful instructions for starting Docker
type DockerNotRunningError struct{}

func (e *DockerNotRunningError) Error() string {
	var instructions string

	switch runtime.GOOS {
	case "darwin":
		instructions = `Docker is not running. To fix this:

  1. Open Docker Desktop application
  2. Wait for Docker to start (whale icon in menu bar stops animating)
  3. Run tomato again

  Or start Docker from terminal:
    open -a Docker`

	case "linux":
		instructions = `Docker is not running. To fix this:

  1. Start Docker daemon:
       sudo systemctl start docker

  2. Make sure your user is in the docker group:
       sudo usermod -aG docker $USER
       (log out and back in after this)

  3. Run tomato again`

	case "windows":
		instructions = `Docker is not running. To fix this:

  1. Open Docker Desktop application
  2. Wait for Docker to start
  3. Run tomato again`

	default:
		instructions = `Docker is not running. Please start Docker and try again.`
	}

	return instructions
}
