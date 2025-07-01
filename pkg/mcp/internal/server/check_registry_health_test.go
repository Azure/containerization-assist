package server

import (
	"testing"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/internal/types"
)

// Test CheckRegistryHealthArgs type
func TestCheckRegistryHealthArgs(t *testing.T) {
	args := CheckRegistryHealthArgs{
		BaseToolArgs: types.BaseToolArgs{
			SessionID: "session-456",
			DryRun:    true,
		},
		Registries:     []string{"docker.io", "gcr.io", "quay.io"},
		Detailed:       true,
		IncludeMetrics: false,
		ForceRefresh:   true,
		Timeout:        60,
	}

	if args.SessionID != "session-456" {
		t.Errorf("Expected SessionID to be 'session-456', got '%s'", args.SessionID)
	}
	if !args.DryRun {
		t.Error("Expected DryRun to be true")
	}
	if len(args.Registries) != 3 {
		t.Errorf("Expected 3 registries, got %d", len(args.Registries))
	}
	if args.Registries[0] != "docker.io" {
		t.Errorf("Expected first registry to be 'docker.io', got '%s'", args.Registries[0])
	}
	if !args.Detailed {
		t.Error("Expected Detailed to be true")
	}
	if args.IncludeMetrics {
		t.Error("Expected IncludeMetrics to be false")
	}
	if !args.ForceRefresh {
		t.Error("Expected ForceRefresh to be true")
	}
	if args.Timeout != 60 {
		t.Errorf("Expected Timeout to be 60, got %d", args.Timeout)
	}
}

// Test CheckRegistryHealthResult type
func TestCheckRegistryHealthResult(t *testing.T) {
	checkTime := time.Now()
	duration := time.Second * 5

	result := CheckRegistryHealthResult{
		BaseToolResponse: types.BaseToolResponse{
			SessionID: "session-789",
			Tool:      "check_registry_health",
		},
		AllHealthy:   true,
		TotalChecked: 3,
		HealthyCount: 3,
		CheckTime:    checkTime,
		Duration:     duration,
	}

	if result.SessionID != "session-789" {
		t.Errorf("Expected SessionID to be 'session-789', got '%s'", result.SessionID)
	}
	if result.Tool != "check_registry_health" {
		t.Errorf("Expected Tool to be 'check_registry_health', got '%s'", result.Tool)
	}
	if !result.AllHealthy {
		t.Error("Expected AllHealthy to be true")
	}
	if result.TotalChecked != 3 {
		t.Errorf("Expected TotalChecked to be 3, got %d", result.TotalChecked)
	}
	if result.HealthyCount != 3 {
		t.Errorf("Expected HealthyCount to be 3, got %d", result.HealthyCount)
	}
	if result.CheckTime != checkTime {
		t.Errorf("Expected CheckTime to match, got %v", result.CheckTime)
	}
	if result.Duration != duration {
		t.Errorf("Expected Duration to be %v, got %v", duration, result.Duration)
	}
}

// Test CheckRegistryHealthTool constructor (updated for consolidated API)
func TestCheckRegistryHealthTool(t *testing.T) {
	// Test that the tool struct can be created
	tool := &CheckRegistryHealthTool{}

	if tool == nil {
		t.Error("CheckRegistryHealthTool should not be nil")
	}

	// Test metadata
	metadata := tool.GetMetadata()
	if metadata.Name != "check_registry_health" {
		t.Errorf("Expected tool name to be 'check_registry_health', got '%s'", metadata.Name)
	}
	if metadata.Category != "registry" {
		t.Errorf("Expected tool category to be 'registry', got '%s'", metadata.Category)
	}
}

// Test CheckRegistryHealthTool validation
func TestCheckRegistryHealthToolValidation(t *testing.T) {
	tool := &CheckRegistryHealthTool{}

	// Test with valid args
	validArgs := CheckRegistryHealthArgs{
		BaseToolArgs: types.BaseToolArgs{
			SessionID: "test-session",
		},
		Registries: []string{"docker.io"},
		Timeout:    30,
	}

	err := tool.Validate(nil, validArgs)
	if err != nil {
		t.Errorf("Validation should pass for valid args, got error: %v", err)
	}

	// Test with invalid args
	err = tool.Validate(nil, "invalid")
	if err == nil {
		t.Error("Validation should fail for invalid args type")
	}
}

// Test various CheckRegistryHealthArgs configurations
func TestCheckRegistryHealthArgsVariations(t *testing.T) {
	// Test minimal args
	minimalArgs := CheckRegistryHealthArgs{
		BaseToolArgs: types.BaseToolArgs{
			SessionID: "minimal-session",
		},
	}

	if minimalArgs.SessionID != "minimal-session" {
		t.Errorf("Expected SessionID to be 'minimal-session', got '%s'", minimalArgs.SessionID)
	}
	if len(minimalArgs.Registries) != 0 {
		t.Errorf("Expected 0 registries (empty), got %d", len(minimalArgs.Registries))
	}
	if minimalArgs.Detailed {
		t.Error("Expected Detailed to be false by default")
	}
	if minimalArgs.Timeout != 0 {
		t.Errorf("Expected Timeout to be 0 (default), got %d", minimalArgs.Timeout)
	}

	// Test maximal args
	maximalArgs := CheckRegistryHealthArgs{
		BaseToolArgs: types.BaseToolArgs{
			SessionID: "maximal-session",
			DryRun:    true,
		},
		Registries:     []string{"registry1.com", "registry2.com", "registry3.com", "registry4.com"},
		Detailed:       true,
		IncludeMetrics: true,
		ForceRefresh:   true,
		Timeout:        120,
	}

	if len(maximalArgs.Registries) != 4 {
		t.Errorf("Expected 4 registries, got %d", len(maximalArgs.Registries))
	}
	if !maximalArgs.Detailed {
		t.Error("Expected Detailed to be true")
	}
	if !maximalArgs.IncludeMetrics {
		t.Error("Expected IncludeMetrics to be true")
	}
	if !maximalArgs.ForceRefresh {
		t.Error("Expected ForceRefresh to be true")
	}
	if maximalArgs.Timeout != 120 {
		t.Errorf("Expected Timeout to be 120, got %d", maximalArgs.Timeout)
	}
}

// Test CheckRegistryHealthResult with failure scenario
func TestCheckRegistryHealthResultFailure(t *testing.T) {
	checkTime := time.Now()
	duration := time.Second * 10

	result := CheckRegistryHealthResult{
		BaseToolResponse: types.BaseToolResponse{
			SessionID: "session-fail",
			Tool:      "check_registry_health",
		},
		AllHealthy:   false,
		TotalChecked: 5,
		HealthyCount: 3,
		CheckTime:    checkTime,
		Duration:     duration,
	}

	if result.Tool != "check_registry_health" {
		t.Errorf("Expected Tool to be 'check_registry_health', got '%s'", result.Tool)
	}
	if result.AllHealthy {
		t.Error("Expected AllHealthy to be false")
	}
	if result.TotalChecked != 5 {
		t.Errorf("Expected TotalChecked to be 5, got %d", result.TotalChecked)
	}
	if result.HealthyCount != 3 {
		t.Errorf("Expected HealthyCount to be 3, got %d", result.HealthyCount)
	}

	// Calculate unhealthy count
	unhealthyCount := result.TotalChecked - result.HealthyCount
	if unhealthyCount != 2 {
		t.Errorf("Expected 2 unhealthy registries, got %d", unhealthyCount)
	}
}
