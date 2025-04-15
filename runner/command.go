package runner

import (
	"errors"
	"io"
	"os/exec"

	log "github.com/sirupsen/logrus"
)

// CommandRunner is an interface for executing commands and getting the output/error
type CommandRunner interface {
	RunCommand(...string) (string, error)
	RunCommandStderr(...string) (string, error)
}

type DefaultCommandRunner struct{}

var _ CommandRunner = &DefaultCommandRunner{}

func (d *DefaultCommandRunner) RunCommand(args ...string) (string, error) {
	log.Debug("Running command: ", args)
	cmd := exec.Command(args[0], args[1:]...)
	out, err := cmd.CombinedOutput()
	log.Debug("Command output: ", string(out))
	return string(out), err
}

// RunCommandStderr runs a command and returns only the stderr output
func (d *DefaultCommandRunner) RunCommandStderr(args ...string) (string, error) {
	log.Debug("Running command (stderr only): ", args)
	cmd := exec.Command(args[0], args[1:]...)

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return "", err
	}

	cmd.Stdout = io.Discard

	if err := cmd.Start(); err != nil {
		return "", err
	}

	stderrBytes, err := io.ReadAll(stderr)
	if err != nil {
		return "", err
	}

	cmdErr := cmd.Wait()

	stderrOutput := string(stderrBytes)
	log.Debug("Command stderr output: ", stderrOutput)

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
