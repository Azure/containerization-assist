package session

import (
	"encoding/json"
	"testing"
	"time"
)

func TestSessionTypesSerialization(t *testing.T) {
	// Test CreateSessionRequest
	request := &CreateSessionRequest{
		Name:        "test-session",
		Description: "A test session",
		Type:        SessionTypeInteractive,
		Labels:      map[string]string{"env": "test"},
		TTL:         time.Hour,
	}

	data, err := json.Marshal(request)
	if err != nil {
		t.Fatalf("Failed to marshal CreateSessionRequest: %v", err)
	}

	var decoded CreateSessionRequest
	err = json.Unmarshal(data, &decoded)
	if err != nil {
		t.Fatalf("Failed to unmarshal CreateSessionRequest: %v", err)
	}

	if decoded.Name != request.Name {
		t.Errorf("Expected Name %s, got %s", request.Name, decoded.Name)
	}
	if decoded.Type != request.Type {
		t.Errorf("Expected Type %s, got %s", request.Type, decoded.Type)
	}
}

func TestSessionMetadataSerialization(t *testing.T) {
	// Test SessionMetadata
	metadata := &SessionMetadata{
		Version:     "1.0",
		Environment: "test",
		Platform:    "linux",
		Resources: SessionResources{
			MaxMemory:     "1GB",
			MaxCPU:        "2",
			MaxExecutions: 100,
			Timeout:       time.Minute * 30,
		},
		Permissions: []string{"read", "write"},
		Tags:        []string{"test", "development"},
		Custom:      map[string]interface{}{"key": "value"},
	}

	data, err := json.Marshal(metadata)
	if err != nil {
		t.Fatalf("Failed to marshal SessionMetadata: %v", err)
	}

	var decoded SessionMetadata
	err = json.Unmarshal(data, &decoded)
	if err != nil {
		t.Fatalf("Failed to unmarshal SessionMetadata: %v", err)
	}

	if decoded.Version != metadata.Version {
		t.Errorf("Expected Version %s, got %s", metadata.Version, decoded.Version)
	}
	if decoded.Resources.MaxMemory != metadata.Resources.MaxMemory {
		t.Errorf("Expected MaxMemory %s, got %s", metadata.Resources.MaxMemory, decoded.Resources.MaxMemory)
	}
}

func TestSessionFilterSerialization(t *testing.T) {
	// Test SessionFilter
	now := time.Now()
	filter := &SessionFilter{
		IDs:           []string{"session1", "session2"},
		Types:         []SessionType{SessionTypeInteractive, SessionTypeWorkflow},
		Statuses:      []SessionStatus{"active", "inactive"},
		CreatedAfter:  &now,
		CreatedBefore: &now,
		Limit:         10,
		Offset:        0,
	}

	data, err := json.Marshal(filter)
	if err != nil {
		t.Fatalf("Failed to marshal SessionFilter: %v", err)
	}

	var decoded SessionFilter
	err = json.Unmarshal(data, &decoded)
	if err != nil {
		t.Fatalf("Failed to unmarshal SessionFilter: %v", err)
	}

	if len(decoded.IDs) != len(filter.IDs) {
		t.Errorf("Expected %d IDs, got %d", len(filter.IDs), len(decoded.IDs))
	}
	if decoded.Limit != filter.Limit {
		t.Errorf("Expected Limit %d, got %d", filter.Limit, decoded.Limit)
	}
}

func TestHistoryEntrySerialization(t *testing.T) {
	// Test HistoryEntry
	entry := &HistoryEntry{
		ID:        "hist-123",
		Timestamp: time.Now(),
		Action:    "execute_tool",
		Tool:      "analyze",
		Input:     map[string]interface{}{"path": "/repo"},
		Output:    map[string]interface{}{"result": "success"},
		Duration:  time.Second * 5,
	}

	data, err := json.Marshal(entry)
	if err != nil {
		t.Fatalf("Failed to marshal HistoryEntry: %v", err)
	}

	var decoded HistoryEntry
	err = json.Unmarshal(data, &decoded)
	if err != nil {
		t.Fatalf("Failed to unmarshal HistoryEntry: %v", err)
	}

	if decoded.ID != entry.ID {
		t.Errorf("Expected ID %s, got %s", entry.ID, decoded.ID)
	}
	if decoded.Action != entry.Action {
		t.Errorf("Expected Action %s, got %s", entry.Action, decoded.Action)
	}
}

func TestSessionStatsSerialization(t *testing.T) {
	// Test SessionStats
	lastExecution := time.Now()
	stats := &SessionStats{
		TotalExecutions:      10,
		SuccessfulExecutions: 8,
		FailedExecutions:     2,
		TotalDuration:        time.Minute * 15,
		LastExecutionTime:    &lastExecution,
		ResourceUsage: ResourceUsage{
			CPUSeconds: 120.5,
			MemoryMB:   256.0,
			StorageMB:  1024.0,
			NetworkMB:  50.0,
		},
	}

	data, err := json.Marshal(stats)
	if err != nil {
		t.Fatalf("Failed to marshal SessionStats: %v", err)
	}

	var decoded SessionStats
	err = json.Unmarshal(data, &decoded)
	if err != nil {
		t.Fatalf("Failed to unmarshal SessionStats: %v", err)
	}

	if decoded.TotalExecutions != stats.TotalExecutions {
		t.Errorf("Expected TotalExecutions %d, got %d", stats.TotalExecutions, decoded.TotalExecutions)
	}
	if decoded.ResourceUsage.CPUSeconds != stats.ResourceUsage.CPUSeconds {
		t.Errorf("Expected CPUSeconds %f, got %f", stats.ResourceUsage.CPUSeconds, decoded.ResourceUsage.CPUSeconds)
	}
}
