package progress

import (
	"encoding/json"
	"fmt"
	"time"
)

// Metadata represents structured progress information
type Metadata struct {
	Kind       string                 `json:"kind"`     // e.g. "progress"
	StageID    string                 `json:"stage_id"` // unique identifier for the stage
	Step       string                 `json:"step"`     // human-readable step name
	Current    int                    `json:"current"`
	Total      int                    `json:"total"`
	Percentage int                    `json:"percentage"`
	Status     Status                 `json:"status"`                // e.g. "running", "failed", "skipped"
	StatusCode int                    `json:"status_code,omitempty"` // numeric code for UI styling
	Message    string                 `json:"message,omitempty"`     // current progress message
	Progress   float64                `json:"progress"`              // 0.0 to 1.0
	ETAMS      int64                  `json:"eta_ms,omitempty"`      // estimated time to completion in milliseconds
	StartTime  time.Time              `json:"start_time,omitempty"`
	Duration   time.Duration          `json:"duration,omitempty"`
	Details    map[string]interface{} `json:"details,omitempty"`
	Error      string                 `json:"error,omitempty"`
	RetryCount int                    `json:"retry_count,omitempty"`
}

// Status represents the current status of a step
type Status string

const (
	StatusPending   Status = "pending"
	StatusRunning   Status = "running"
	StatusCompleted Status = "completed"
	StatusFailed    Status = "failed"
	StatusSkipped   Status = "skipped"
	StatusRetrying  Status = "retrying"
)

// Status code constants for UI styling
const (
	StatusCodePending   = 0
	StatusCodeRunning   = 1
	StatusCodeCompleted = 2
	StatusCodeFailed    = 3
	StatusCodeSkipped   = 4
	StatusCodeRetrying  = 5
)

// GetStatusCode returns the numeric code for a status
func GetStatusCode(status Status) int {
	switch status {
	case StatusPending:
		return StatusCodePending
	case StatusRunning:
		return StatusCodeRunning
	case StatusCompleted:
		return StatusCodeCompleted
	case StatusFailed:
		return StatusCodeFailed
	case StatusSkipped:
		return StatusCodeSkipped
	case StatusRetrying:
		return StatusCodeRetrying
	default:
		return 0
	}
}

// StepInfo provides detailed information about a workflow step
type StepInfo struct {
	Name        string        `json:"name"`
	Description string        `json:"description"`
	StartTime   time.Time     `json:"start_time"`
	EndTime     time.Time     `json:"end_time,omitempty"`
	Duration    time.Duration `json:"duration,omitempty"`
	Status      Status        `json:"status"`
	Error       error         `json:"-"`
	ErrorMsg    string        `json:"error,omitempty"`
	Metadata    Metadata      `json:"metadata"`
}

// NewStepInfo creates a new step info
func NewStepInfo(name, description string, current, total int) *StepInfo {
	percentage := 0
	progress := 0.0
	if total > 0 {
		percentage = (current * 100) / total
		progress = float64(current) / float64(total)
	}

	stageID := generateStageID(name, current)
	status := StatusRunning

	return &StepInfo{
		Name:        name,
		Description: description,
		StartTime:   time.Now(),
		Status:      status,
		Metadata: Metadata{
			Kind:       "progress",
			StageID:    stageID,
			Step:       name,
			Current:    current,
			Total:      total,
			Percentage: percentage,
			Progress:   progress,
			Status:     status,
			StatusCode: GetStatusCode(status),
			Message:    description,
			StartTime:  time.Now(),
			Details:    make(map[string]interface{}),
		},
	}
}

// generateStageID creates a unique stage identifier
func generateStageID(name string, step int) string {
	// Simple stage ID generation - can be enhanced as needed
	return fmt.Sprintf("%s_%d_%s", name, step, time.Now().Format("20060102150405"))
}

// Complete marks the step as completed
func (s *StepInfo) Complete() {
	s.EndTime = time.Now()
	s.Duration = s.EndTime.Sub(s.StartTime)
	s.Status = StatusCompleted
	s.Metadata.Status = StatusCompleted
	s.Metadata.StatusCode = GetStatusCode(StatusCompleted)
	s.Metadata.Duration = s.Duration
	s.Metadata.Progress = 1.0
	s.Metadata.Percentage = 100
	s.Metadata.ETAMS = 0 // No ETA when completed
}

// Fail marks the step as failed
func (s *StepInfo) Fail(err error) {
	s.EndTime = time.Now()
	s.Duration = s.EndTime.Sub(s.StartTime)
	s.Status = StatusFailed
	s.Error = err
	s.ErrorMsg = err.Error()
	s.Metadata.Status = StatusFailed
	s.Metadata.StatusCode = GetStatusCode(StatusFailed)
	s.Metadata.Error = err.Error()
	s.Metadata.Duration = s.Duration
	s.Metadata.ETAMS = 0 // No ETA when failed
}

// AddDetail adds a detail to the step metadata
func (s *StepInfo) AddDetail(key string, value interface{}) {
	s.Metadata.Details[key] = value
}

// UpdateProgress updates the current progress and calculates ETA
func (s *StepInfo) UpdateProgress(current int, message string) {
	s.Metadata.Current = current
	s.Metadata.Message = message

	if s.Metadata.Total > 0 {
		s.Metadata.Percentage = (current * 100) / s.Metadata.Total
		s.Metadata.Progress = float64(current) / float64(s.Metadata.Total)

		// Calculate ETA based on elapsed time and progress
		elapsed := time.Since(s.StartTime)
		if current > 0 && current < s.Metadata.Total {
			totalEstimated := elapsed * time.Duration(s.Metadata.Total) / time.Duration(current)
			remaining := totalEstimated - elapsed
			s.Metadata.ETAMS = remaining.Milliseconds()
		} else {
			s.Metadata.ETAMS = 0
		}
	}
}

// WorkflowProgress tracks overall workflow progress
type WorkflowProgress struct {
	WorkflowID   string        `json:"workflow_id"`
	WorkflowName string        `json:"workflow_name"`
	TotalSteps   int           `json:"total_steps"`
	CurrentStep  int           `json:"current_step"`
	Percentage   int           `json:"percentage"`
	StartTime    time.Time     `json:"start_time"`
	EndTime      time.Time     `json:"end_time,omitempty"`
	Duration     time.Duration `json:"duration,omitempty"`
	Status       Status        `json:"status"`
	Steps        []*StepInfo   `json:"steps"`
	Error        string        `json:"error,omitempty"`
}

// NewWorkflowProgress creates a new workflow progress tracker
func NewWorkflowProgress(id, name string, totalSteps int) *WorkflowProgress {
	return &WorkflowProgress{
		WorkflowID:   id,
		WorkflowName: name,
		TotalSteps:   totalSteps,
		CurrentStep:  0,
		Percentage:   0,
		StartTime:    time.Now(),
		Status:       StatusRunning,
		Steps:        make([]*StepInfo, 0, totalSteps),
	}
}

// AddStep adds a new step to the workflow
func (w *WorkflowProgress) AddStep(step *StepInfo) {
	w.Steps = append(w.Steps, step)
	w.CurrentStep = len(w.Steps)
	w.Percentage = (w.CurrentStep * 100) / w.TotalSteps
}

// Complete marks the workflow as completed
func (w *WorkflowProgress) Complete() {
	w.EndTime = time.Now()
	w.Duration = w.EndTime.Sub(w.StartTime)
	w.Status = StatusCompleted
	w.Percentage = 100
}

// Fail marks the workflow as failed
func (w *WorkflowProgress) Fail(err string) {
	w.EndTime = time.Now()
	w.Duration = w.EndTime.Sub(w.StartTime)
	w.Status = StatusFailed
	w.Error = err
}

// MarshalJSON implements json.Marshaler for Metadata
func (m Metadata) MarshalJSON() ([]byte, error) {
	// Use an alias to avoid recursion
	type Alias Metadata
	return json.Marshal(&struct {
		Alias
		StartTime string `json:"start_time,omitempty"`
		Duration  string `json:"duration,omitempty"`
	}{
		Alias:     Alias(m),
		StartTime: m.StartTime.Format(time.RFC3339),
		Duration:  m.Duration.String(),
	})
}

// UnmarshalJSON implements json.Unmarshaler for Metadata
func (m *Metadata) UnmarshalJSON(data []byte) error {
	// Use an alias to avoid recursion
	type Alias Metadata
	aux := &struct {
		*Alias
		StartTime string `json:"start_time,omitempty"`
		Duration  string `json:"duration,omitempty"`
	}{
		Alias: (*Alias)(m),
	}

	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	// Parse time fields
	if aux.StartTime != "" {
		t, err := time.Parse(time.RFC3339, aux.StartTime)
		if err == nil {
			m.StartTime = t
		}
	}

	if aux.Duration != "" {
		d, err := time.ParseDuration(aux.Duration)
		if err == nil {
			m.Duration = d
		}
	}

	return nil
}
