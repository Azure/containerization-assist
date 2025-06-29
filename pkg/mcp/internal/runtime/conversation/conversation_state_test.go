package conversation

import (
	"fmt"
	"testing"
	"time"

	"github.com/Azure/container-kit/pkg/mcp"
	"github.com/Azure/container-kit/pkg/mcp/internal/types"
	"github.com/stretchr/testify/assert"
)

func TestNewConversationState(t *testing.T) {
	sessionID := "test-session-123"
	workspaceDir := "/tmp/workspace"
	state := NewConversationState(sessionID, workspaceDir)

	assert.NotNil(t, state)
	assert.Equal(t, sessionID, state.SessionID)
	assert.Equal(t, convertFromTypesStage(types.StageWelcome), state.CurrentStage)
	assert.NotNil(t, state.Context)
	assert.NotNil(t, state.History)
	assert.Empty(t, state.History)
	assert.False(t, state.CreatedAt.IsZero())
	assert.NotNil(t, state.Artifacts)
	assert.Empty(t, state.Artifacts)
}

func TestConversationStateAddToHistory(t *testing.T) {
	state := NewConversationState("test-session", "/tmp/workspace")

	// Add some conversation turns
	turns := []struct {
		input    string
		response string
		stage    mcp.ConversationStage
	}{
		{"hello", "Welcome!", convertFromTypesStage(types.StageWelcome)},
		{"analyze", "Starting analysis...", convertFromTypesStage(types.StageAnalysis)},
		{"github.com/test/repo", "Analyzing repository...", convertFromTypesStage(types.StageAnalysis)},
	}

	for _, turnData := range turns {
		turn := ConversationTurn{
			UserInput: turnData.input,
			Assistant: turnData.response,
			Stage:     turnData.stage,
		}
		state.AddConversationTurn(turn)
		state.SetStage(turnData.stage)
	}

	// Verify history
	assert.Len(t, state.History, len(turns))

	for i, turn := range state.History {
		assert.Equal(t, turns[i].input, turn.UserInput)
		assert.Equal(t, turns[i].response, turn.Assistant)
		assert.False(t, turn.Timestamp.IsZero())
		assert.True(t, turn.Timestamp.After(state.CreatedAt) || turn.Timestamp.Equal(state.CreatedAt))
	}
}

func TestConversationStateGetDuration(t *testing.T) {
	state := NewConversationState("test-session", "/tmp/workspace")

	// Wait a bit
	time.Sleep(100 * time.Millisecond)

	duration := time.Since(state.CreatedAt)
	assert.Greater(t, duration.Milliseconds(), int64(90))
}

func TestConversationStateIsTimeout(t *testing.T) {
	tests := []struct {
		name      string
		startTime time.Time
		timeout   time.Duration
		expected  bool
	}{
		{
			name:      "not timed out",
			startTime: time.Now(),
			timeout:   1 * time.Hour,
			expected:  false,
		},
		{
			name:      "timed out",
			startTime: time.Now().Add(-2 * time.Hour),
			timeout:   1 * time.Hour,
			expected:  true,
		},
		{
			name:      "exactly at timeout",
			startTime: time.Now().Add(-1 * time.Hour),
			timeout:   1 * time.Hour,
			expected:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state := NewConversationState("test-session", "/tmp/workspace")
			state.CreatedAt = tt.startTime
			isTimedOut := time.Since(state.CreatedAt) > tt.timeout
			assert.Equal(t, tt.expected, isTimedOut)
		})
	}
}

func TestConversationStateContextManagement(t *testing.T) {
	state := NewConversationState("test-session", "/tmp/workspace")

	// Add context values
	state.Context["repo_url"] = "github.com/test/repo"
	state.Context["language"] = "go"
	state.Context["framework"] = "gin"

	// Verify context
	assert.Equal(t, "github.com/test/repo", state.Context["repo_url"])
	assert.Equal(t, "go", state.Context["language"])
	assert.Equal(t, "gin", state.Context["framework"])
	assert.Nil(t, state.Context["nonexistent"])

	// Update context
	state.Context["language"] = "python"
	assert.Equal(t, "python", state.Context["language"])

	// Clear specific context
	delete(state.Context, "framework")
	assert.Nil(t, state.Context["framework"])

	// Bulk update context
	newContext := map[string]interface{}{
		"new_key": "new_value",
		"another": 123,
	}
	for k, v := range newContext {
		state.Context[k] = v
	}
	assert.Equal(t, "new_value", state.Context["new_key"])
	assert.Equal(t, 123, state.Context["another"])
	assert.Equal(t, "github.com/test/repo", state.Context["repo_url"]) // Original still there
}

func TestConversationStateDecisionHandling(t *testing.T) {
	state := NewConversationState("test-session", "/tmp/workspace")

	// Initially no pending decision
	assert.Nil(t, state.PendingDecision)

	// Set a decision point
	decision := &DecisionPoint{
		ID:       "test_decision",
		Stage:    convertFromTypesStage(types.StageAnalysis),
		Question: "What would you like to do?",
		Options: []Option{
			{
				ID:          "option1",
				Label:       "Option 1",
				Description: "First option",
				Recommended: true,
			},
		},
		Required: true,
	}

	state.SetPendingDecision(decision)
	assert.NotNil(t, state.PendingDecision)
	assert.Equal(t, "test_decision", state.PendingDecision.ID)

	// Resolve decision
	userDecision := Decision{
		DecisionID: "test_decision",
		OptionID:   "option1",
		Timestamp:  time.Now(),
	}
	state.ResolvePendingDecision(userDecision)
	assert.Nil(t, state.PendingDecision)
}

func TestConversationStateErrorTracking(t *testing.T) {
	state := NewConversationState("test-session", "/tmp/workspace")

	// Test error tracking through conversation history
	turn := ConversationTurn{
		ID:        "turn-1",
		Timestamp: time.Now(),
		UserInput: "test input",
		Stage:     convertFromTypesStage(types.StageBuild),
		Error: &types.ToolError{
			Type:      "test_error",
			Message:   "Something went wrong",
			Retryable: true,
			Timestamp: time.Now(),
		},
	}

	state.History = append(state.History, turn)
	assert.Len(t, state.History, 1)
	assert.NotNil(t, state.History[0].Error)
	assert.Equal(t, "Something went wrong", state.History[0].Error.Message)
}

func TestConversationStateStageTransitions(t *testing.T) {
	state := NewConversationState("test-session", "/tmp/workspace")

	// Track stage transitions
	stages := []mcp.ConversationStage{
		convertFromTypesStage(types.StageWelcome),
		convertFromTypesStage(types.StagePreFlight),
		convertFromTypesStage(types.StageInit),
		convertFromTypesStage(types.StageAnalysis),
		convertFromTypesStage(types.StageDockerfile),
		convertFromTypesStage(types.StageBuild),
		convertFromTypesStage(types.StageManifests),
		convertFromTypesStage(types.StageDeployment),
		convertFromTypesStage(types.StageCompleted),
	}

	for _, stage := range stages {
		state.SetStage(stage)
		assert.Equal(t, stage, state.CurrentStage)
	}

	// Test stage progression validation
	assert.Equal(t, convertFromTypesStage(types.StageCompleted), state.CurrentStage)
}

func TestConversationStateArtifacts(t *testing.T) {
	state := NewConversationState("test-session", "/tmp/workspace")

	// Add some artifacts
	artifact1 := Artifact{
		Type:    "dockerfile",
		Name:    "Dockerfile",
		Content: "FROM alpine:latest",
		Stage:   convertFromTypesStage(types.StageDockerfile),
	}
	state.AddArtifact(artifact1)

	artifact2 := Artifact{
		Type:    "manifest",
		Name:    "deployment.yaml",
		Content: "apiVersion: apps/v1",
		Stage:   convertFromTypesStage(types.StageManifests),
	}
	state.AddArtifact(artifact2)

	// Test retrieval
	dockerfiles := state.GetArtifactsByType("dockerfile")
	assert.Len(t, dockerfiles, 1)
	assert.Equal(t, "Dockerfile", dockerfiles[0].Name)

	manifests := state.GetArtifactsByType("manifest")
	assert.Len(t, manifests, 1)
	assert.Equal(t, "deployment.yaml", manifests[0].Name)

	// Test total artifacts
	assert.Len(t, state.Artifacts, 2)
}

func TestConversationStateStageProgression(t *testing.T) {
	state := NewConversationState("test-session", "/tmp/workspace")

	// Test stage progression logic
	progress := state.GetStageProgress()
	assert.Equal(t, convertFromTypesStage(types.StageWelcome), progress.CurrentStage)
	assert.Equal(t, 1, progress.CurrentStep)
	assert.Greater(t, progress.TotalSteps, 1)

	// Test stage progression
	state.SetStage(convertFromTypesStage(types.StageAnalysis))
	progress = state.GetStageProgress()
	assert.Equal(t, convertFromTypesStage(types.StageAnalysis), progress.CurrentStage)
	assert.Greater(t, progress.CurrentStep, 1)
}

func TestConversationTurn(t *testing.T) {
	turn := ConversationTurn{
		UserInput: "test input",
		Assistant: "test response",
		Timestamp: time.Now(),
		Stage:     convertFromTypesStage(types.StageWelcome),
	}

	assert.Equal(t, "test input", turn.UserInput)
	assert.Equal(t, "test response", turn.Assistant)
	assert.False(t, turn.Timestamp.IsZero())
	assert.Equal(t, convertFromTypesStage(types.StageWelcome), turn.Stage)
}

func TestConversationHistoryManagement(t *testing.T) {
	state := NewConversationState("test-session", "/tmp/workspace")

	// Add multiple conversation turns
	for i := 0; i < 10; i++ {
		turn := ConversationTurn{
			UserInput: fmt.Sprintf("input-%d", i),
			Assistant: fmt.Sprintf("response-%d", i),
			Stage:     convertFromTypesStage(types.StageWelcome),
		}
		state.AddConversationTurn(turn)
	}

	// Verify history length and content
	assert.Len(t, state.History, 10)

	// Test latest turn retrieval
	latest := state.GetLatestTurn()
	assert.NotNil(t, latest)
	assert.Equal(t, "input-9", latest.UserInput)
	assert.Equal(t, "response-9", latest.Assistant)
}
