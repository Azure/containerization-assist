// Package commands provides CQRS command definitions for Containerization Assist MCP.
package commands

import (
	"context"
	"fmt"
	"time"

	"github.com/Azure/containerization-assist/pkg/common/errors"
	"github.com/Azure/containerization-assist/pkg/mcp/domain/workflow"
)

// Command represents a command that changes system state
type Command interface {
	// CommandID returns the unique identifier for this command
	CommandID() string

	// CommandType returns the type name of this command
	CommandType() string

	// Validate checks if the command is valid
	Validate() error
}

// CommandHandler handles command execution
type CommandHandler interface {
	Handle(ctx context.Context, cmd Command) error
}

// BaseCommand provides common fields and validation for all commands
type BaseCommand struct {
	ID        string    `json:"id"`
	SessionID string    `json:"session_id"`
	Timestamp time.Time `json:"timestamp"`
	UserID    string    `json:"user_id,omitempty"`
}

// ValidateBase performs common validation for all commands
func (b BaseCommand) ValidateBase() error {
	var errs []error

	if b.ID == "" {
		errs = append(errs, errors.New(
			errors.CodeMissingParameter,
			"command.id",
			"command ID is required",
			nil,
		))
	}

	if b.SessionID == "" {
		errs = append(errs, errors.New(
			errors.CodeMissingParameter,
			"command.session_id",
			"session ID is required",
			nil,
		))
	}

	if len(errs) > 0 {
		return errors.New(
			errors.CodeValidationFailed,
			"command",
			fmt.Sprintf("command validation failed: %d errors", len(errs)),
			fmt.Errorf("validation errors: %v", errs),
		)
	}

	return nil
}

// ContainerizeCommand represents a request to containerize and deploy a repository
type ContainerizeCommand struct {
	BaseCommand
	Args workflow.ContainerizeAndDeployArgs `json:"args"`
}

func (c ContainerizeCommand) CommandID() string   { return c.ID }
func (c ContainerizeCommand) CommandType() string { return "containerize" }

func (c ContainerizeCommand) Validate() error {
	// First validate base command fields
	if err := c.ValidateBase(); err != nil {
		return err
	}

	// Then validate command-specific fields
	if c.Args.RepoURL == "" {
		return errors.New(
			errors.CodeMissingParameter,
			"command.args.repo_url",
			"repository URL is required",
			nil,
		)
	}

	return nil
}

// CancelWorkflowCommand represents a request to cancel a running workflow
type CancelWorkflowCommand struct {
	BaseCommand
	WorkflowID string `json:"workflow_id"`
	Reason     string `json:"reason,omitempty"`
}

func (c CancelWorkflowCommand) CommandID() string   { return c.ID }
func (c CancelWorkflowCommand) CommandType() string { return "cancel_workflow" }

func (c CancelWorkflowCommand) Validate() error {
	// First validate base command fields
	if err := c.ValidateBase(); err != nil {
		return err
	}

	// Then validate command-specific fields
	if c.WorkflowID == "" {
		return errors.New(
			errors.CodeMissingParameter,
			"command.workflow_id",
			"workflow ID is required",
			nil,
		)
	}

	return nil
}

// UpdateWorkflowConfigCommand represents a request to update workflow configuration
type UpdateWorkflowConfigCommand struct {
	BaseCommand
	Config workflow.ServerConfig `json:"config"`
}

func (c UpdateWorkflowConfigCommand) CommandID() string   { return c.ID }
func (c UpdateWorkflowConfigCommand) CommandType() string { return "update_config" }

func (c UpdateWorkflowConfigCommand) Validate() error {
	// First validate base command fields
	if err := c.ValidateBase(); err != nil {
		return err
	}

	// Config validation: The workflow.Config struct has built-in validation
	// through its constructor and setter methods. Additional validation
	// would be added here if custom validation rules are needed.
	return nil
}
