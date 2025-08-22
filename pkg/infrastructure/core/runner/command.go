package runner

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
)

// CommandRunner is an interface for running shell commands
type CommandRunner interface {
	RunCommand(cmd string, args ...string) (string, error)
	RunCommandStderr(cmd string, args ...string) (string, error)
	RunWithOutput(cmd string, args ...string) (string, string, error)
}

// DefaultCommandRunner is the default implementation using os/exec
type DefaultCommandRunner struct{}

// RunCommand runs a command and returns stdout
func (r *DefaultCommandRunner) RunCommand(cmd string, args ...string) (string, error) {
	command := exec.Command(cmd, args...)
	output, err := command.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return "", fmt.Errorf("command failed: %v\nStderr: %s", err, exitErr.Stderr)
		}
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}

// RunCommandStderr runs a command and returns stderr
func (r *DefaultCommandRunner) RunCommandStderr(cmd string, args ...string) (string, error) {
	command := exec.Command(cmd, args...)
	var stderr bytes.Buffer
	command.Stderr = &stderr
	err := command.Run()
	return stderr.String(), err
}

// RunWithOutput runs a command and returns both stdout and stderr
func (r *DefaultCommandRunner) RunWithOutput(cmd string, args ...string) (string, string, error) {
	command := exec.Command(cmd, args...)
	var stdout, stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	err := command.Run()
	return stdout.String(), stderr.String(), err
}
