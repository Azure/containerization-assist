package common

import (
	"time"
)

type Status string

const (
	StatusPending   Status = "pending"
	StatusRunning   Status = "running"
	StatusCompleted Status = "completed"
	StatusFailed    Status = "failed"
	StatusCancelled Status = "cancelled"
)

type Priority string

const (
	PriorityLow    Priority = "low"
	PriorityMedium Priority = "medium"
	PriorityHigh   Priority = "high"
	PriorityUrgent Priority = "urgent"
)

type OperationContext struct {
	ID           string                 `json:"id"`
	Type         string                 `json:"type"`
	Status       Status                 `json:"status"`
	Priority     Priority               `json:"priority"`
	StartTime    time.Time              `json:"start_time"`
	EndTime      *time.Time             `json:"end_time,omitempty"`
	Duration     time.Duration          `json:"duration"`
	Metadata     map[string]interface{} `json:"metadata"`
	Progress     float64                `json:"progress"`
	ErrorCount   int                    `json:"error_count"`
	WarningCount int                    `json:"warning_count"`
}

type ExecutionResult struct {
	Success   bool                   `json:"success"`
	Data      interface{}            `json:"data,omitempty"`
	Error     string                 `json:"error,omitempty"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
	Timestamp time.Time              `json:"timestamp"`
	Duration  time.Duration          `json:"duration"`
}

type HealthStatus struct {
	Healthy    bool                   `json:"healthy"`
	LastCheck  time.Time              `json:"last_check"`
	Components map[string]bool        `json:"components"`
	Errors     []string               `json:"errors,omitempty"`
	Warnings   []string               `json:"warnings,omitempty"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
}

func NewOperationContext(id, opType string, priority Priority) *OperationContext {
	return &OperationContext{
		ID:           id,
		Type:         opType,
		Status:       StatusPending,
		Priority:     priority,
		StartTime:    time.Now(),
		Metadata:     make(map[string]interface{}),
		Progress:     0.0,
		ErrorCount:   0,
		WarningCount: 0,
	}
}

func (oc *OperationContext) Start() {
	oc.Status = StatusRunning
	oc.StartTime = time.Now()
}

func (oc *OperationContext) Complete() {
	oc.Status = StatusCompleted
	now := time.Now()
	oc.EndTime = &now
	oc.Duration = now.Sub(oc.StartTime)
	oc.Progress = 100.0
}

func (oc *OperationContext) Fail() {
	oc.Status = StatusFailed
	now := time.Now()
	oc.EndTime = &now
	oc.Duration = now.Sub(oc.StartTime)
}

func (oc *OperationContext) Cancel() {
	oc.Status = StatusCancelled
	now := time.Now()
	oc.EndTime = &now
	oc.Duration = now.Sub(oc.StartTime)
}

func (oc *OperationContext) UpdateProgress(progress float64) {
	if progress < 0 {
		progress = 0
	}
	if progress > 100 {
		progress = 100
	}
	oc.Progress = progress
}
