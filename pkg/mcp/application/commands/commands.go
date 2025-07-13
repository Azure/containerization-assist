// Package commands provides CQRS command definitions for Container Kit MCP.
package commands

import (
	"context"
	"fmt"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/domain/workflow"
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

// ContainerizeCommand represents a request to containerize and deploy a repository
type ContainerizeCommand struct {
	ID        string                             `json:"id"`
	SessionID string                             `json:"session_id"`
	Timestamp time.Time                          `json:"timestamp"`
	UserID    string                             `json:"user_id,omitempty"`
	Args      workflow.ContainerizeAndDeployArgs `json:"args"`
}

func (c ContainerizeCommand) CommandID() string   { return c.ID }
func (c ContainerizeCommand) CommandType() string { return "containerize" }

func (c ContainerizeCommand) Validate() error {
	if c.ID == "" {
		return fmt.Errorf("command ID is required")
	}
	if c.SessionID == "" {
		return fmt.Errorf("session ID is required")
	}
	if c.Args.RepoURL == "" {
		return fmt.Errorf("repository URL is required")
	}
	return nil
}

// CancelWorkflowCommand represents a request to cancel a running workflow
type CancelWorkflowCommand struct {
	ID         string    `json:"id"`
	SessionID  string    `json:"session_id"`
	WorkflowID string    `json:"workflow_id"`
	Timestamp  time.Time `json:"timestamp"`
	UserID     string    `json:"user_id,omitempty"`
	Reason     string    `json:"reason,omitempty"`
}

func (c CancelWorkflowCommand) CommandID() string   { return c.ID }
func (c CancelWorkflowCommand) CommandType() string { return "cancel_workflow" }

func (c CancelWorkflowCommand) Validate() error {
	if c.ID == "" {
		return fmt.Errorf("command ID is required")
	}
	if c.SessionID == "" {
		return fmt.Errorf("session ID is required")
	}
	if c.WorkflowID == "" {
		return fmt.Errorf("workflow ID is required")
	}
	return nil
}

// UpdateWorkflowConfigCommand represents a request to update workflow configuration
type UpdateWorkflowConfigCommand struct {
	ID        string                `json:"id"`
	SessionID string                `json:"session_id"`
	Timestamp time.Time             `json:"timestamp"`
	UserID    string                `json:"user_id,omitempty"`
	Config    workflow.ServerConfig `json:"config"`
}

func (c UpdateWorkflowConfigCommand) CommandID() string   { return c.ID }
func (c UpdateWorkflowConfigCommand) CommandType() string { return "update_config" }

func (c UpdateWorkflowConfigCommand) Validate() error {
	if c.ID == "" {
		return fmt.Errorf("command ID is required")
	}
	if c.SessionID == "" {
		return fmt.Errorf("session ID is required")
	}
	// TODO: Add config validation
	return nil
}
