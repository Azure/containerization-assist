package server

import (
	"context"
	"testing"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/internal/types"
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

// Test GetJobStatusResult type (updated for consolidated API)
func TestGetJobStatusResult(t *testing.T) {
	result := GetJobStatusResult{
		BaseToolResponse: types.BaseToolResponse{},
		JobID:            "job-789",
		Status:           "running",
		Progress:         50,
		Output:           "Job in progress",
	}

	if result.JobID != "job-789" {
		t.Errorf("Expected JobID to be 'job-789', got '%s'", result.JobID)
	}
	if result.Status != "running" {
		t.Errorf("Expected Status to be 'running', got '%s'", result.Status)
	}
	if result.Progress != 50 {
		t.Errorf("Expected Progress to be 50, got %d", result.Progress)
	}
	if result.Output != "Job in progress" {
		t.Errorf("Expected Output to be 'Job in progress', got '%s'", result.Output)
	}
}

// Test GetJobStatusResult extended fields
func TestGetJobStatusResultExtended(t *testing.T) {
	startTime := time.Now().Add(-30 * time.Minute)
	endTime := time.Now()

	result := GetJobStatusResult{
		BaseToolResponse: types.BaseToolResponse{},
		JobID:            "job-test-123",
		Status:           "completed",
		Progress:         100,
		StartTime:        startTime,
		EndTime:          &endTime,
		Output:           "Job completed successfully",
	}

	if result.JobID != "job-test-123" {
		t.Errorf("Expected JobID to be 'job-test-123', got '%s'", result.JobID)
	}
	if result.Status != "completed" {
		t.Errorf("Expected Status to be 'completed', got '%s'", result.Status)
	}
	if result.Progress != 100 {
		t.Errorf("Expected Progress to be 100, got %d", result.Progress)
	}
	if result.StartTime != startTime {
		t.Errorf("Expected StartTime to match, got %v", result.StartTime)
	}
	if result.EndTime == nil {
		t.Error("Expected EndTime to not be nil")
	} else if *result.EndTime != endTime {
		t.Errorf("Expected EndTime to match, got %v", *result.EndTime)
	}
	if result.Output != "Job completed successfully" {
		t.Errorf("Expected Output to be 'Job completed successfully', got '%s'", result.Output)
	}
}

// Test GetJobStatusTool tool metadata and execution
func TestGetJobStatusToolExecution(t *testing.T) {
	tool := &GetJobStatusTool{}

	if tool == nil {
		t.Error("GetJobStatusTool should not be nil")
	}

	// Test metadata
	metadata := tool.GetMetadata()
	if metadata.Name != "get_job_status" {
		t.Errorf("Expected tool name to be 'get_job_status', got '%s'", metadata.Name)
	}
	if metadata.Category != "jobs" {
		t.Errorf("Expected tool category to be 'jobs', got '%s'", metadata.Category)
	}

	// Test execution with valid args
	args := GetJobStatusArgs{
		BaseToolArgs: types.BaseToolArgs{
			SessionID: "test-session",
		},
		JobID: "test-job",
	}

	result, err := tool.Execute(context.Background(), args)
	if err != nil {
		t.Errorf("Execute should not return error, got %v", err)
	}
	if result == nil {
		t.Error("Execute should return a result")
	}

	// Verify result type
	if statusResult, ok := result.(*GetJobStatusResult); ok {
		if statusResult.JobID != "test-job" {
			t.Errorf("Expected JobID to be 'test-job', got '%s'", statusResult.JobID)
		}
	} else {
		t.Error("Expected result to be of type *GetJobStatusResult")
	}
}

// Test GetJobStatusTool validation
func TestGetJobStatusToolValidation(t *testing.T) {
	tool := &GetJobStatusTool{}

	// Test with valid args
	validArgs := GetJobStatusArgs{
		BaseToolArgs: types.BaseToolArgs{
			SessionID: "test-session",
		},
		JobID: "test-job",
	}

	err := tool.Validate(context.Background(), validArgs)
	if err != nil {
		t.Errorf("Validation should pass for valid args, got error: %v", err)
	}

	// Test with invalid args type
	err = tool.Validate(context.Background(), "invalid")
	if err == nil {
		t.Error("Validation should fail for invalid args type")
	}

	// Test execution with invalid args type
	result, err := tool.Execute(context.Background(), "invalid")
	if err == nil {
		t.Error("Execute should fail for invalid args type")
	}
	if result != nil {
		t.Error("Execute should return nil for invalid args")
	}
}

// Test GetJobStatusTool integration
func TestGetJobStatusToolIntegration(t *testing.T) {
	tool := &GetJobStatusTool{}

	// Test multiple job status scenarios
	testCases := []struct {
		name        string
		jobID       string
		expectError bool
	}{
		{"valid job", "job-123", false},
		{"another valid job", "job-456", false},
		{"empty job ID", "", true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			args := GetJobStatusArgs{
				BaseToolArgs: types.BaseToolArgs{
					SessionID: "test-session",
				},
				JobID: tc.jobID,
			}

			result, err := tool.Execute(context.Background(), args)
			if tc.expectError && err == nil {
				t.Error("Expected error but got none")
			}
			if !tc.expectError && err != nil {
				t.Errorf("Expected no error but got: %v", err)
			}
			if !tc.expectError && result != nil {
				if statusResult, ok := result.(*GetJobStatusResult); ok {
					if statusResult.JobID != tc.jobID {
						t.Errorf("Expected JobID '%s', got '%s'", tc.jobID, statusResult.JobID)
					}
				}
			}
		})
	}
}
