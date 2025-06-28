package conversation

import (
	"fmt"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/internal/types"
	mcptypes "github.com/Azure/container-kit/pkg/mcp/types"
)

// RetryState tracks retry attempts for a specific operation
type RetryState struct {
	Attempts    int       `json:"attempts"`
	LastAttempt time.Time `json:"last_attempt"`
	LastError   string    `json:"last_error,omitempty"`
}

// ConversationState extends SessionState with conversation-specific fields
type ConversationState struct {
	*mcptypes.SessionState

	// Conversation flow
	CurrentStage    types.ConversationStage `json:"current_stage"`
	History         []ConversationTurn      `json:"conversation_history"`
	Preferences     types.UserPreferences   `json:"user_preferences"`
	PendingDecision *DecisionPoint          `json:"pending_decision,omitempty"`

	// Conversation context
	Context   map[string]interface{} `json:"conversation_context"`
	Artifacts map[string]Artifact    `json:"artifacts"`

	// Security scan state
	SecurityScanCompleted bool `json:"security_scan_completed"`
	SecurityScore         int  `json:"security_score"`

	// Retry tracking
	RetryStates map[string]*RetryState `json:"retry_states,omitempty"`
}

// ConversationTurn represents a single turn in the conversation
type ConversationTurn struct {
	ID        string                  `json:"id"`
	Timestamp time.Time               `json:"timestamp"`
	UserInput string                  `json:"user_input"`
	Assistant string                  `json:"assistant_response"`
	Stage     types.ConversationStage `json:"stage"`
	ToolCalls []ToolCall              `json:"tool_calls,omitempty"`
	Decision  *Decision               `json:"decision,omitempty"`
	Error     *types.ToolError        `json:"error,omitempty"`
}

// ToolCall represents a tool invocation within a conversation turn
type ToolCall struct {
	Tool       string                 `json:"tool"`
	Parameters map[string]interface{} `json:"parameters"`
	Result     interface{}            `json:"result,omitempty"`
	Error      *types.ToolError       `json:"error,omitempty"`
	Duration   time.Duration          `json:"duration"`
}

// DecisionPoint represents a point where user input is needed
type DecisionPoint struct {
	ID       string                  `json:"id"`
	Stage    types.ConversationStage `json:"stage"`
	Question string                  `json:"question"`
	Options  []Option                `json:"options"`
	Default  string                  `json:"default,omitempty"`
	Required bool                    `json:"required"`
	Context  map[string]interface{}  `json:"context,omitempty"`
}

// Option represents a choice in a decision point
type Option struct {
	ID          string      `json:"id"`
	Label       string      `json:"label"`
	Description string      `json:"description,omitempty"`
	Recommended bool        `json:"recommended"`
	Value       interface{} `json:"value,omitempty"`
}

// Decision represents a user's choice at a decision point
type Decision struct {
	DecisionID  string      `json:"decision_id"`
	OptionID    string      `json:"option_id,omitempty"`
	CustomValue interface{} `json:"custom_value,omitempty"`
	Timestamp   time.Time   `json:"timestamp"`
}

// Artifact represents a generated file or output
type Artifact struct {
	ID        string                  `json:"id"`
	Type      string                  `json:"type"` // "dockerfile", "manifest", "config"
	Name      string                  `json:"name"`
	Content   string                  `json:"content"`
	Path      string                  `json:"path,omitempty"`
	CreatedAt time.Time               `json:"created_at"`
	UpdatedAt time.Time               `json:"updated_at"`
	Stage     types.ConversationStage `json:"stage"`
	Metadata  map[string]interface{}  `json:"metadata,omitempty"`
}

// NewConversationState creates a new conversation state
func NewConversationState(sessionID, workspaceDir string) *ConversationState {
	return &ConversationState{
		SessionState: &mcptypes.SessionState{
			SessionID:    sessionID,
			WorkspaceDir: workspaceDir,
			CreatedAt:    time.Now(),
			UpdatedAt:    time.Now(),
			Metadata:     make(map[string]interface{}),
		},
		CurrentStage: types.StageWelcome,
		History:      make([]ConversationTurn, 0),
		Preferences: types.UserPreferences{
			Namespace:          "default",
			Replicas:           1,
			ServiceType:        "ClusterIP",
			IncludeHealthCheck: true,
		},
		Context:   make(map[string]interface{}),
		Artifacts: make(map[string]Artifact),
	}
}

// AddConversationTurn adds a new turn to the conversation history
func (cs *ConversationState) AddConversationTurn(turn ConversationTurn) {
	turn.ID = generateTurnID()
	turn.Timestamp = time.Now()
	cs.History = append(cs.History, turn)
	cs.SessionState.UpdatedAt = time.Now()
}

// SetStage updates the current conversation stage
func (cs *ConversationState) SetStage(stage types.ConversationStage) {
	cs.CurrentStage = stage
	cs.SessionState.UpdatedAt = time.Now()
}

// SetPendingDecision sets a decision point that needs user input
func (cs *ConversationState) SetPendingDecision(decision *DecisionPoint) {
	cs.PendingDecision = decision
	cs.SessionState.UpdatedAt = time.Now()
}

// ResolvePendingDecision resolves a pending decision with user's choice
func (cs *ConversationState) ResolvePendingDecision(decision Decision) {
	if cs.PendingDecision != nil && cs.PendingDecision.ID == decision.DecisionID {
		cs.PendingDecision = nil
		// Store the decision in the latest turn
		if len(cs.History) > 0 {
			cs.History[len(cs.History)-1].Decision = &decision
		}
	}
	cs.SessionState.UpdatedAt = time.Now()
}

// AddArtifact adds a generated artifact to the state
func (cs *ConversationState) AddArtifact(artifact Artifact) {
	artifact.ID = generateArtifactID()
	artifact.CreatedAt = time.Now()
	artifact.UpdatedAt = time.Now()
	cs.Artifacts[artifact.ID] = artifact
	cs.SessionState.UpdatedAt = time.Now()
}

// UpdateArtifact updates an existing artifact
func (cs *ConversationState) UpdateArtifact(artifactID, content string) {
	if artifact, exists := cs.Artifacts[artifactID]; exists {
		artifact.Content = content
		artifact.UpdatedAt = time.Now()
		cs.Artifacts[artifactID] = artifact
		cs.SessionState.UpdatedAt = time.Now()
	}
}

// GetArtifactsByType returns all artifacts of a specific type
func (cs *ConversationState) GetArtifactsByType(artifactType string) []Artifact {
	var artifacts []Artifact
	for _, artifact := range cs.Artifacts {
		if artifact.Type == artifactType {
			artifacts = append(artifacts, artifact)
		}
	}
	return artifacts
}

// GetLatestTurn returns the most recent conversation turn
func (cs *ConversationState) GetLatestTurn() *ConversationTurn {
	if len(cs.History) == 0 {
		return nil
	}
	return &cs.History[len(cs.History)-1]
}

// CanProceedToStage checks if the conversation can proceed to a given stage
func (cs *ConversationState) CanProceedToStage(stage types.ConversationStage) bool {
	// Define stage dependencies
	switch stage {
	case types.StageInit:
		return cs.CurrentStage == types.StageWelcome
	case types.StageAnalysis:
		repoURL := ""
		if cs.SessionState.Metadata != nil {
			if url, ok := cs.SessionState.Metadata["repo_url"].(string); ok {
				repoURL = url
			}
		}
		return cs.CurrentStage == types.StageInit && repoURL != ""
	case types.StageDockerfile:
		repoAnalysisExists := false
		if cs.SessionState.Metadata != nil {
			if repoAnalysis, ok := cs.SessionState.Metadata["repo_analysis"].(map[string]interface{}); ok {
				repoAnalysisExists = len(repoAnalysis) > 0
			}
		}
		return cs.CurrentStage == types.StageAnalysis && repoAnalysisExists
	case types.StageManifests:
		dockerfileContent := ""
		if cs.SessionState.Metadata != nil {
			if content, ok := cs.SessionState.Metadata["dockerfile_content"].(string); ok {
				dockerfileContent = content
			}
		}
		return cs.CurrentStage == types.StageDockerfile && dockerfileContent != ""
	case types.StageDeployment:
		k8sManifestsExist := false
		if cs.SessionState.Metadata != nil {
			if manifests, ok := cs.SessionState.Metadata["k8s_manifests"].(map[string]interface{}); ok {
				k8sManifestsExist = len(manifests) > 0
			}
		}
		return cs.CurrentStage == types.StageManifests && k8sManifestsExist
	case types.StageCompleted:
		return cs.CurrentStage == types.StageDeployment
	default:
		return false
	}
}

// GetStageProgress returns the progress through the workflow
func (cs *ConversationState) GetStageProgress() StageProgress {
	stages := []types.ConversationStage{
		types.StageWelcome,
		types.StageInit,
		types.StageAnalysis,
		types.StageDockerfile,
		types.StageManifests,
		types.StageDeployment,
		types.StageCompleted,
	}

	currentIndex := 0
	for i, stage := range stages {
		if stage == cs.CurrentStage {
			currentIndex = i
			break
		}
	}

	return StageProgress{
		CurrentStage:    cs.CurrentStage,
		CurrentStep:     currentIndex + 1,
		TotalSteps:      len(stages),
		Percentage:      (currentIndex * 100) / (len(stages) - 1),
		CompletedStages: stages[:currentIndex],
		RemainingStages: stages[currentIndex+1:],
	}
}

// StageProgress represents progress through the workflow
type StageProgress struct {
	CurrentStage    types.ConversationStage   `json:"current_stage"`
	CurrentStep     int                       `json:"current_step"`
	TotalSteps      int                       `json:"total_steps"`
	Percentage      int                       `json:"percentage"`
	CompletedStages []types.ConversationStage `json:"completed_stages"`
	RemainingStages []types.ConversationStage `json:"remaining_stages"`
}

// Helper functions
func generateTurnID() string {
	return fmt.Sprintf("turn-%d", time.Now().UnixNano())
}

func generateArtifactID() string {
	return fmt.Sprintf("artifact-%d", time.Now().UnixNano())
}
