package tools

import (
	"context"

	"github.com/Azure/container-copilot/pkg/mcp/internal/types"
	"github.com/rs/zerolog"
)

// GetJobStatusArgs defines the arguments for the get_job_status tool
type GetJobStatusArgs struct {
	types.BaseToolArgs
	JobID string `json:"job_id" description:"Job ID to check status"`
}

// GetJobStatusResult defines the response for the get_job_status tool
type GetJobStatusResult struct {
	types.BaseToolResponse
	JobInfo JobInfo `json:"job_info"`
}

// JobInfo represents job information (simplified interface)
type JobInfo struct {
	JobID       string                 `json:"job_id"`
	Type        string                 `json:"type"`
	Status      string                 `json:"status"`
	SessionID   string                 `json:"session_id"`
	CreatedAt   string                 `json:"created_at"`
	StartedAt   *string                `json:"started_at,omitempty"`
	CompletedAt *string                `json:"completed_at,omitempty"`
	Duration    *string                `json:"duration,omitempty"`
	Progress    float64                `json:"progress"`
	Message     string                 `json:"message,omitempty"`
	Error       string                 `json:"error,omitempty"`
	Result      map[string]interface{} `json:"result,omitempty"`
	Logs        []string               `json:"logs,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// GetJobStatusTool implements job status checking functionality
type GetJobStatusTool struct {
	logger     zerolog.Logger
	getJobFunc func(jobID string) (*JobInfo, error)
}

// Execute retrieves the status of a job
func (t *GetJobStatusTool) Execute(ctx context.Context, args GetJobStatusArgs) (*GetJobStatusResult, error) {
	// Create base response
	response := &GetJobStatusResult{
		BaseToolResponse: types.NewBaseResponse("get_job_status", args.SessionID, args.DryRun),
	}

	t.logger.Info().
		Str("session_id", args.SessionID).
		Str("job_id", args.JobID).
		Bool("dry_run", args.DryRun).
		Msg("Getting job status")

	if args.JobID == "" {
		return nil, types.NewRichError("INVALID_ARGUMENTS", "job_id is required", "validation_error")
	}

	// Handle dry-run mode
	if args.DryRun {
		response.JobInfo = JobInfo{
			JobID:     args.JobID,
			Type:      "build",
			Status:    "running",
			SessionID: args.SessionID,
			CreatedAt: "2024-12-17T10:00:00Z",
			Progress:  0.5,
			Message:   "Dry-run: Job would be checked",
			Logs:      []string{"This is a dry-run preview"},
		}
		return response, nil
	}

	// Get job from job manager
	job, err := t.getJobFunc(args.JobID)
	if err != nil {
		return nil, types.NewRichError("INTERNAL_SERVER_ERROR", "failed to get job: "+err.Error(), "execution_error")
	}

	// Job is already in the correct format
	response.JobInfo = *job

	t.logger.Info().
		Str("session_id", args.SessionID).
		Str("job_id", args.JobID).
		Str("status", response.JobInfo.Status).
		Float64("progress", response.JobInfo.Progress).
		Msg("Retrieved job status")

	return response, nil
}

// NewGetJobStatusTool creates a new instance of GetJobStatusTool
func NewGetJobStatusTool(logger zerolog.Logger, getJobFunc func(jobID string) (*JobInfo, error)) *GetJobStatusTool {
	return &GetJobStatusTool{
		logger:     logger,
		getJobFunc: getJobFunc,
	}
}

// CreateMockJobStatusTool creates a simplified version for testing
func CreateMockJobStatusTool(logger zerolog.Logger) *GetJobStatusTool {
	mockGetJob := func(jobID string) (*JobInfo, error) {
		return &JobInfo{
			JobID:     jobID,
			Type:      "build",
			Status:    "completed",
			SessionID: "test-session",
			CreatedAt: "2024-12-17T10:00:00Z",
			Progress:  1.0,
			Message:   "Mock job completed successfully",
			Logs:      []string{"Starting build...", "Build completed successfully"},
		}, nil
	}
	return NewGetJobStatusTool(logger, mockGetJob)
}
