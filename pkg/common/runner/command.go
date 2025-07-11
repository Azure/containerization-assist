package runner

import (
	"errors"
	"fmt"
	"io"
	"os/exec"

	"github.com/Azure/container-kit/pkg/common/logger"
)

// CommandRunner is an interface for executing commands and getting the output/error
type CommandRunner interface {
	RunCommand(...string) (string, error)
	RunCommandStderr(...string) (string, error)
}

type DefaultCommandRunner struct{}

var _ CommandRunner = &DefaultCommandRunner{}

func (d *DefaultCommandRunner) RunCommand(args ...string) (string, error) {
	logger.Debugf("Running command: %s", args)
	cmd := exec.Command(args[0], args[1:]...)
	out, err := cmd.CombinedOutput()
	logger.Debugf("Command output: %s", string(out))
	return string(out), err
}

// RunCommandStderr runs a command and returns only the stderr output
func (d *DefaultCommandRunner) RunCommandStderr(args ...string) (string, error) {
	logger.Debugf("Running command (stderr only): %v", args)
	cmd := exec.Command(args[0], args[1:]...)

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return "", fmt.Errorf("failed to get stderr pipe: %w", err)
	}

	cmd.Stdout = io.Discard

	if err := cmd.Start(); err != nil {
		return "", err
	}

	stderrBytes, err := io.ReadAll(stderr)
	if err != nil {
		return "", fmt.Errorf("failed to start command: %w", err)
	}

	cmdErr := cmd.Wait()

	stderrOutput := string(stderrBytes)
	logger.Debugf("Command stderr output: %s", stderrOutput)

	return stderrOutput, cmdErr
}

type FakeCommandRunner struct {
	Output string
	ErrStr string
}

var _ CommandRunner = &FakeCommandRunner{}

func (f *FakeCommandRunner) RunCommand(args ...string) (string, error) {
	if f.ErrStr != "" {
		return f.Output, errors.New(f.ErrStr)
	}
	return f.Output, nil
}

func (f *FakeCommandRunner) RunCommandStderr(args ...string) (string, error) {
	if f.ErrStr != "" {
		return f.ErrStr, errors.New(f.ErrStr)
	}
	return "", nil
}
