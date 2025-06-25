package workflow

import (
	"context"
)

// StageExecutor interface for executing workflow stages
type StageExecutor interface {
	ExecuteStage(ctx context.Context, stage *WorkflowStage, session *WorkflowSession) (*StageResult, error)
	ValidateStage(stage *WorkflowStage) error
}

// WorkflowSessionManager interface for managing workflow sessions
type WorkflowSessionManager interface {
	CreateSession(workflowSpec *WorkflowSpec) (*WorkflowSession, error)
	GetSession(sessionID string) (*WorkflowSession, error)
	UpdateSession(session *WorkflowSession) error
	DeleteSession(sessionID string) error
	ListSessions(filter SessionFilter) ([]*WorkflowSession, error)
}

// DependencyResolver interface for resolving stage dependencies
type DependencyResolver interface {
	ResolveDependencies(stages []WorkflowStage) ([][]WorkflowStage, error)
	ValidateDependencies(stages []WorkflowStage) error
	GetExecutionOrder(stages []WorkflowStage) ([]string, error)
}

// ErrorRouter interface for routing and handling workflow errors
type ErrorRouter interface {
	RouteError(ctx context.Context, err *WorkflowError, session *WorkflowSession) (*ErrorAction, error)
	CanRecover(err *WorkflowError) bool
	GetRecoveryOptions(err *WorkflowError) []RecoveryOption
}

// CheckpointManager interface for managing workflow checkpoints
type CheckpointManager interface {
	CreateCheckpoint(session *WorkflowSession, stageName string, message string, workflowSpec *WorkflowSpec) (*WorkflowCheckpoint, error)
	RestoreFromCheckpoint(sessionID string, checkpointID string) (*WorkflowSession, error)
	ListCheckpoints(sessionID string) ([]*WorkflowCheckpoint, error)
	DeleteCheckpoint(checkpointID string) error
}
