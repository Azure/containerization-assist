package state

import (
	"context"
	"time"
)

// CheckpointManagerInterface defines the interface for checkpoint management
// This breaks the import cycle with orchestration package
type CheckpointManagerInterface interface {
	SaveCheckpoint(ctx context.Context, sessionID string, data interface{}) error
	LoadCheckpoint(ctx context.Context, sessionID string) (interface{}, error)
	DeleteCheckpoint(ctx context.Context, sessionID string) error
	ListCheckpoints(ctx context.Context) ([]string, error)
}

// WorkflowSessionInterface defines the interface for workflow sessions
// This breaks the import cycle with orchestration package
type WorkflowSessionInterface interface {
	GetSessionID() string
	GetCurrentStage() string
	GetProgress() float64
	GetStartTime() time.Time
	GetEndTime() *time.Time
	IsCompleted() bool
	GetMetadata() map[string]interface{}
}

// Basic workflow session implementation to avoid orchestration dependency
type BasicWorkflowSession struct {
	SessionID    string                 `json:"session_id"`
	CurrentStage string                 `json:"current_stage"`
	Progress     float64                `json:"progress"`
	StartTime    time.Time              `json:"start_time"`
	EndTime      *time.Time             `json:"end_time,omitempty"`
	Completed    bool                   `json:"completed"`
	Metadata     map[string]interface{} `json:"metadata"`
}

func (w *BasicWorkflowSession) GetSessionID() string {
	return w.SessionID
}

func (w *BasicWorkflowSession) GetCurrentStage() string {
	return w.CurrentStage
}

func (w *BasicWorkflowSession) GetProgress() float64 {
	return w.Progress
}

func (w *BasicWorkflowSession) GetStartTime() time.Time {
	return w.StartTime
}

func (w *BasicWorkflowSession) GetEndTime() *time.Time {
	return w.EndTime
}

func (w *BasicWorkflowSession) IsCompleted() bool {
	return w.Completed
}

func (w *BasicWorkflowSession) GetMetadata() map[string]interface{} {
	return w.Metadata
}

// ConversationStateInterface defines the interface for conversation state
// This breaks the import cycle with conversation package
type ConversationStateInterface interface {
	GetConversationID() string
	GetCurrentStage() string
	GetHistory() []ConversationEntryInterface
	GetDecisions() map[string]DecisionInterface
	GetArtifacts() map[string]ArtifactInterface
}

// ConversationEntryInterface defines the interface for conversation entries
type ConversationEntryInterface interface {
	GetTimestamp() time.Time
	GetMessage() string
	GetRole() string
}

// DecisionInterface defines the interface for decisions
type DecisionInterface interface {
	GetID() string
	GetDescription() string
	GetTimestamp() time.Time
}

// ArtifactInterface defines the interface for artifacts
type ArtifactInterface interface {
	GetName() string
	GetType() string
	GetContent() interface{}
}

// Basic implementations to avoid conversation dependency
type BasicConversationState struct {
	ConversationID string                   `json:"conversation_id"`
	CurrentStage   string                   `json:"current_stage"`
	History        []BasicConversationEntry `json:"history"`
	Decisions      map[string]BasicDecision `json:"decisions"`
	Artifacts      map[string]BasicArtifact `json:"artifacts"`
	SessionState   interface{}              `json:"session_state"`
}

type BasicConversationEntry struct {
	Timestamp time.Time `json:"timestamp"`
	Message   string    `json:"message"`
	Role      string    `json:"role"`
}

type BasicDecision struct {
	ID          string    `json:"id"`
	Description string    `json:"description"`
	Timestamp   time.Time `json:"timestamp"`
}

type BasicArtifact struct {
	Name    string      `json:"name"`
	Type    string      `json:"type"`
	Content interface{} `json:"content"`
}

func (c *BasicConversationState) GetConversationID() string {
	return c.ConversationID
}

func (c *BasicConversationState) GetCurrentStage() string {
	return c.CurrentStage
}

func (c *BasicConversationState) GetHistory() []ConversationEntryInterface {
	result := make([]ConversationEntryInterface, len(c.History))
	for i := range c.History {
		result[i] = &c.History[i]
	}
	return result
}

func (c *BasicConversationState) GetDecisions() map[string]DecisionInterface {
	result := make(map[string]DecisionInterface)
	for k, v := range c.Decisions {
		v := v // avoid loop variable capture
		result[k] = &v
	}
	return result
}

func (c *BasicConversationState) GetArtifacts() map[string]ArtifactInterface {
	result := make(map[string]ArtifactInterface)
	for k, v := range c.Artifacts {
		v := v // avoid loop variable capture
		result[k] = &v
	}
	return result
}

func (e *BasicConversationEntry) GetTimestamp() time.Time {
	return e.Timestamp
}

func (e *BasicConversationEntry) GetMessage() string {
	return e.Message
}

func (e *BasicConversationEntry) GetRole() string {
	return e.Role
}

func (d *BasicDecision) GetID() string {
	return d.ID
}

func (d *BasicDecision) GetDescription() string {
	return d.Description
}

func (d *BasicDecision) GetTimestamp() time.Time {
	return d.Timestamp
}

func (a *BasicArtifact) GetName() string {
	return a.Name
}

func (a *BasicArtifact) GetType() string {
	return a.Type
}

func (a *BasicArtifact) GetContent() interface{} {
	return a.Content
}
