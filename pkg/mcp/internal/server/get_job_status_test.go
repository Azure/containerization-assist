package server

import (
	"testing"

	"github.com/Azure/container-kit/pkg/mcp/internal/types"
	"github.com/rs/zerolog"
)

// Test GetJobStatusArgs type
func TestGetJobStatusArgs(t *testing.T) {
	args := GetJobStatusArgs{
		BaseToolArgs: types.BaseToolArgs{
			SessionID: "session-123",
			DryRun:    false,
		},
		JobID: "job-456",
	}

	if args.SessionID != "session-123" {
		t.Errorf("Expected SessionID to be 'session-123', got '%s'", args.SessionID)
	}
	if args.DryRun {
		t.Error("Expected DryRun to be false")
	}
	if args.JobID != "job-456" {
		t.Errorf("Expected JobID to be 'job-456', got '%s'", args.JobID)
	}
}

// Test GetJobStatusResult type
func TestGetJobStatusResult(t *testing.T) {
	jobInfo := JobInfo{
		JobID:     "job-789",
		Type:      "build",
		Status:    "running",
		SessionID: "session-abc",
		CreatedAt: "2024-01-01T10:00:00Z",
		Progress:  0.5,
		Message:   "Job in progress",
	}

	result := GetJobStatusResult{
		BaseToolResponse: types.BaseToolResponse{
			SessionID: "session-abc",
			Tool:      "get_job_status",
		},
		JobInfo: jobInfo,
	}

	if result.SessionID != "session-abc" {
		t.Errorf("Expected SessionID to be 'session-abc', got '%s'", result.SessionID)
	}
	if result.Tool != "get_job_status" {
		t.Errorf("Expected Tool to be 'get_job_status', got '%s'", result.Tool)
	}
	if result.JobInfo.JobID != "job-789" {
		t.Errorf("Expected JobInfo.JobID to be 'job-789', got '%s'", result.JobInfo.JobID)
	}
	if result.JobInfo.Status != "running" {
		t.Errorf("Expected JobInfo.Status to be 'running', got '%s'", result.JobInfo.Status)
	}
}

// Test JobInfo type
func TestJobInfo(t *testing.T) {
	startedAt := "2024-01-01T10:05:00Z"
	completedAt := "2024-01-01T10:30:00Z"
	duration := "25m"

	jobInfo := JobInfo{
		JobID:       "job-test-123",
		Type:        "deployment",
		Status:      "completed",
		SessionID:   "session-xyz",
		CreatedAt:   "2024-01-01T10:00:00Z",
		StartedAt:   &startedAt,
		CompletedAt: &completedAt,
		Duration:    &duration,
		Progress:    1.0,
		Message:     "Deployment completed successfully",
		Error:       "",
		Result:      map[string]interface{}{"deployed": true, "replicas": 3},
		Logs:        []string{"Starting deployment", "Scaling up", "Deployment complete"},
		Metadata:    map[string]interface{}{"environment": "production", "region": "us-east-1"},
	}

	if jobInfo.JobID != "job-test-123" {
		t.Errorf("Expected JobID to be 'job-test-123', got '%s'", jobInfo.JobID)
	}
	if jobInfo.Type != "deployment" {
		t.Errorf("Expected Type to be 'deployment', got '%s'", jobInfo.Type)
	}
	if jobInfo.Status != "completed" {
		t.Errorf("Expected Status to be 'completed', got '%s'", jobInfo.Status)
	}
	if jobInfo.SessionID != "session-xyz" {
		t.Errorf("Expected SessionID to be 'session-xyz', got '%s'", jobInfo.SessionID)
	}
	if jobInfo.CreatedAt != "2024-01-01T10:00:00Z" {
		t.Errorf("Expected CreatedAt to be '2024-01-01T10:00:00Z', got '%s'", jobInfo.CreatedAt)
	}
	if jobInfo.StartedAt == nil {
		t.Error("Expected StartedAt to not be nil")
	} else if *jobInfo.StartedAt != startedAt {
		t.Errorf("Expected StartedAt to be '%s', got '%s'", startedAt, *jobInfo.StartedAt)
	}
	if jobInfo.CompletedAt == nil {
		t.Error("Expected CompletedAt to not be nil")
	} else if *jobInfo.CompletedAt != completedAt {
		t.Errorf("Expected CompletedAt to be '%s', got '%s'", completedAt, *jobInfo.CompletedAt)
	}
	if jobInfo.Duration == nil {
		t.Error("Expected Duration to not be nil")
	} else if *jobInfo.Duration != duration {
		t.Errorf("Expected Duration to be '%s', got '%s'", duration, *jobInfo.Duration)
	}
	if jobInfo.Progress != 1.0 {
		t.Errorf("Expected Progress to be 1.0, got %f", jobInfo.Progress)
	}
	if jobInfo.Message != "Deployment completed successfully" {
		t.Errorf("Expected Message to be 'Deployment completed successfully', got '%s'", jobInfo.Message)
	}
	if jobInfo.Error != "" {
		t.Errorf("Expected Error to be empty, got '%s'", jobInfo.Error)
	}
	if jobInfo.Result["deployed"] != true {
		t.Errorf("Expected Result['deployed'] to be true, got '%v'", jobInfo.Result["deployed"])
	}
	if len(jobInfo.Logs) != 3 {
		t.Errorf("Expected 3 logs, got %d", len(jobInfo.Logs))
	}
	if jobInfo.Logs[0] != "Starting deployment" {
		t.Errorf("Expected first log to be 'Starting deployment', got '%s'", jobInfo.Logs[0])
	}
	if jobInfo.Metadata["environment"] != "production" {
		t.Errorf("Expected Metadata['environment'] to be 'production', got '%v'", jobInfo.Metadata["environment"])
	}
}

// Test NewGetJobStatusTool constructor
func TestNewGetJobStatusTool(t *testing.T) {
	logger := zerolog.Nop()

	// Define a simple getJobFunc
	getJobFunc := func(jobID string) (*JobInfo, error) {
		return &JobInfo{
			JobID:  jobID,
			Status: "completed",
		}, nil
	}

	tool := NewGetJobStatusTool(logger, getJobFunc)

	if tool == nil {
		t.Error("NewGetJobStatusTool should not return nil")
	}
	if tool.getJobFunc == nil {
		t.Error("Expected getJobFunc to be set")
	}
}

// Test CreateMockJobStatusTool constructor
func TestCreateMockJobStatusTool(t *testing.T) {
	logger := zerolog.Nop()

	tool := CreateMockJobStatusTool(logger)

	if tool == nil {
		t.Error("CreateMockJobStatusTool should not return nil")
	}
	if tool.getJobFunc == nil {
		t.Error("Expected getJobFunc to be set in mock tool")
	}

	// Test the mock function
	jobInfo, err := tool.getJobFunc("test-job")
	if err != nil {
		t.Errorf("Mock getJobFunc should not return error, got %v", err)
	}
	if jobInfo == nil {
		t.Error("Mock getJobFunc should return jobInfo")
	}
	if jobInfo.JobID != "test-job" {
		t.Errorf("Expected mock JobID to be 'test-job', got '%s'", jobInfo.JobID)
	}
	if jobInfo.Type != "build" {
		t.Errorf("Expected mock Type to be 'build', got '%s'", jobInfo.Type)
	}
	if jobInfo.Status != "completed" {
		t.Errorf("Expected mock Status to be 'completed', got '%s'", jobInfo.Status)
	}
	if jobInfo.Progress != 1.0 {
		t.Errorf("Expected mock Progress to be 1.0, got %f", jobInfo.Progress)
	}
	if len(jobInfo.Logs) != 2 {
		t.Errorf("Expected 2 mock logs, got %d", len(jobInfo.Logs))
	}
}

// Test GetJobStatusTool struct
func TestGetJobStatusToolStruct(t *testing.T) {
	logger := zerolog.Nop()

	getJobFunc := func(jobID string) (*JobInfo, error) {
		return &JobInfo{JobID: jobID, Status: "pending"}, nil
	}

	tool := GetJobStatusTool{
		logger:     logger,
		getJobFunc: getJobFunc,
	}

	if tool.getJobFunc == nil {
		t.Error("Expected getJobFunc to be set")
	}

	// Test that we can call the function
	result, err := tool.getJobFunc("test")
	if err != nil {
		t.Errorf("getJobFunc should not return error, got %v", err)
	}
	if result.JobID != "test" {
		t.Errorf("Expected result JobID to be 'test', got '%s'", result.JobID)
	}
}
