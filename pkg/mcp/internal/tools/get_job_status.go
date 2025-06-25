package tools

import (
	"context"
	"fmt"

	"github.com/Azure/container-copilot/pkg/mcp/internal/types"
	mcptypes "github.com/Azure/container-copilot/pkg/mcp/types"
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

// Execute implements the unified Tool interface
func (t *GetJobStatusTool) Execute(ctx context.Context, args interface{}) (interface{}, error) {
	// Type assertion to get proper args
	jobArgs, ok := args.(GetJobStatusArgs)
	if !ok {
		return nil, fmt.Errorf("invalid arguments type: expected GetJobStatusArgs, got %T", args)
	}

	return t.ExecuteTyped(ctx, jobArgs)
}

// ExecuteTyped provides typed execution for backward compatibility
func (t *GetJobStatusTool) ExecuteTyped(ctx context.Context, args GetJobStatusArgs) (*GetJobStatusResult, error) {
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

// GetMetadata returns comprehensive metadata about the get job status tool
func (t *GetJobStatusTool) GetMetadata() mcptypes.ToolMetadata {
	return mcptypes.ToolMetadata{
		Name:        "get_job_status",
		Description: "Retrieve detailed status information for a specific job",
		Version:     "1.0.0",
		Category:    "Job Management",
		Dependencies: []string{
			"Job Manager",
			"Job Storage",
		},
		Capabilities: []string{
			"Job status retrieval",
			"Progress tracking",
			"Log access",
			"Result inspection",
			"Error analysis",
			"Metadata access",
		},
		Requirements: []string{
			"Valid job ID",
			"Job manager access",
		},
		Parameters: map[string]string{
			"job_id": "Required: Job ID to check status",
		},
		Examples: []mcptypes.ToolExample{
			{
				Name:        "Check running job status",
				Description: "Get status of a currently running build job",
				Input: map[string]interface{}{
					"job_id": "job-build-123",
				},
				Output: map[string]interface{}{
					"job_info": map[string]interface{}{
						"job_id":     "job-build-123",
						"type":       "build",
						"status":     "running",
						"session_id": "session-456",
						"created_at": "2024-12-17T10:00:00Z",
						"started_at": "2024-12-17T10:01:00Z",
						"progress":   0.75,
						"message":    "Building Docker image",
						"logs":       []string{"Step 1/5: FROM node:16", "Step 2/5: WORKDIR /app"},
					},
				},
			},
			{
				Name:        "Check completed job with result",
				Description: "Get status of a completed deployment job",
				Input: map[string]interface{}{
					"job_id": "job-deploy-789",
				},
				Output: map[string]interface{}{
					"job_info": map[string]interface{}{
						"job_id":       "job-deploy-789",
						"type":         "deploy",
						"status":       "completed",
						"session_id":   "session-456",
						"created_at":   "2024-12-17T09:30:00Z",
						"started_at":   "2024-12-17T09:31:00Z",
						"completed_at": "2024-12-17T09:35:00Z",
						"duration":     "4m",
						"progress":     1.0,
						"message":      "Deployment completed successfully",
						"result": map[string]interface{}{
							"namespace":   "default",
							"deployments": []string{"myapp-deployment"},
							"services":    []string{"myapp-service"},
						},
					},
				},
			},
		},
	}
}

// Validate checks if the provided arguments are valid for the get job status tool
func (t *GetJobStatusTool) Validate(ctx context.Context, args interface{}) error {
	jobArgs, ok := args.(GetJobStatusArgs)
	if !ok {
		return fmt.Errorf("invalid arguments type: expected GetJobStatusArgs, got %T", args)
	}

	// Validate required fields
	if jobArgs.JobID == "" {
		return fmt.Errorf("job_id is required and cannot be empty")
	}

	// Validate job ID format
	if len(jobArgs.JobID) < 3 || len(jobArgs.JobID) > 100 {
		return fmt.Errorf("job_id must be between 3 and 100 characters")
	}

	// Validate job function is available
	if t.getJobFunc == nil {
		return fmt.Errorf("job retrieval function is not configured")
	}

	return nil
}
