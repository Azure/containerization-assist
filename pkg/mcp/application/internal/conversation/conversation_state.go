package conversation

import (
	"fmt"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/domain/session"
)

type RetryState struct {
	Attempts    int       `json:"attempts"`
	LastAttempt time.Time `json:"last_attempt"`
	LastError   string    `json:"last_error,omitempty"`
}
type ConversationState struct {
	*session.SessionState
	CurrentStage          core.ConversationStage `json:"current_stage"`
	History               []ConversationTurn     `json:"conversation_history"`
	Preferences           types.UserPreferences  `json:"user_preferences"`
	PendingDecision       *DecisionPoint         `json:"pending_decision,omitempty"`
	Context               map[string]interface{} `json:"conversation_context"`
	Artifacts             map[string]Artifact    `json:"artifacts"`
	SecurityScanCompleted bool                   `json:"security_scan_completed"`
	SecurityScore         int                    `json:"security_score"`
	RetryStates           map[string]*RetryState `json:"retry_states,omitempty"`
}
type ConversationTurn struct {
	ID        string                 `json:"id"`
	Timestamp time.Time              `json:"timestamp"`
	UserInput string                 `json:"user_input"`
	Assistant string                 `json:"assistant_response"`
	Stage     core.ConversationStage `json:"stage"`
	ToolCalls []ToolCall             `json:"tool_calls,omitempty"`
	Decision  *Decision              `json:"decision,omitempty"`
	Error     *types.ToolError       `json:"error,omitempty"`
}
type ToolCall struct {
	Tool       string                 `json:"tool"`
	Parameters map[string]interface{} `json:"parameters"`
	Result     interface{}            `json:"result,omitempty"`
	Error      *types.ToolError       `json:"error,omitempty"`
	Duration   time.Duration          `json:"duration"`
}
type DecisionPoint struct {
	ID       string                 `json:"id"`
	Stage    core.ConversationStage `json:"stage"`
	Question string                 `json:"question"`
	Options  []Option               `json:"options"`
	Default  string                 `json:"default,omitempty"`
	Required bool                   `json:"required"`
	Context  map[string]interface{} `json:"context,omitempty"`
}
type Option struct {
	ID          string      `json:"id"`
	Label       string      `json:"label"`
	Description string      `json:"description,omitempty"`
	Recommended bool        `json:"recommended"`
	Value       interface{} `json:"value,omitempty"`
}
type Decision struct {
	DecisionID  string      `json:"decision_id"`
	OptionID    string      `json:"option_id,omitempty"`
	CustomValue interface{} `json:"custom_value,omitempty"`
	Timestamp   time.Time   `json:"timestamp"`
}
type Artifact struct {
	ID        string                 `json:"id"`
	Type      string                 `json:"type"`
	Name      string                 `json:"name"`
	Content   string                 `json:"content"`
	Path      string                 `json:"path,omitempty"`
	CreatedAt time.Time              `json:"created_at"`
	UpdatedAt time.Time              `json:"updated_at"`
	Stage     core.ConversationStage `json:"stage"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

func NewConversationState(sessionID, workspaceDir string) *ConversationState {
	return &ConversationState{
		SessionState: &session.SessionState{
			SessionID:    sessionID,
			WorkspaceDir: workspaceDir,
			CreatedAt:    time.Now(),
			LastAccessed: time.Now(),
			Metadata:     make(map[string]interface{}),
		},
		CurrentStage: convertFromTypesStage(types.StageWelcome),
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
func (cs *ConversationState) AddConversationTurn(turn ConversationTurn) {
	turn.ID = generateTurnID()
	turn.Timestamp = time.Now()
	cs.History = append(cs.History, turn)
	cs.SessionState.LastAccessed = time.Now()
	if cs.SessionState.Metadata == nil {
		cs.SessionState.Metadata = make(map[string]interface{})
	}
	historyData := make([]map[string]interface{}, 0, len(cs.History))
	for _, t := range cs.History {
		turnData := map[string]interface{}{
			"id":         t.ID,
			"timestamp":  t.Timestamp,
			"user_input": t.UserInput,
			"assistant":  t.Assistant,
			"stage":      string(t.Stage),
		}

		if len(t.ToolCalls) > 0 {
			toolCallsData := make([]map[string]interface{}, 0, len(t.ToolCalls))
			for _, tc := range t.ToolCalls {
				tcData := map[string]interface{}{
					"tool":       tc.Tool,
					"parameters": tc.Parameters,
					"duration":   tc.Duration.Milliseconds(),
				}
				if tc.Result != nil {
					tcData["result"] = tc.Result
				}
				if tc.Error != nil {
					tcData["error"] = map[string]interface{}{
						"type":      tc.Error.Type,
						"message":   tc.Error.Message,
						"retryable": tc.Error.Retryable,
					}
				}
				toolCallsData = append(toolCallsData, tcData)
			}
			turnData["tool_calls"] = toolCallsData
		}

		historyData = append(historyData, turnData)
	}

	cs.SessionState.Metadata["conversation_history"] = historyData
}
func (cs *ConversationState) SetStage(stage core.ConversationStage) {
	cs.CurrentStage = stage
	cs.SessionState.LastAccessed = time.Now()
}
func (cs *ConversationState) SetPendingDecision(decision *DecisionPoint) {
	cs.PendingDecision = decision
	cs.SessionState.LastAccessed = time.Now()
}
func (cs *ConversationState) ResolvePendingDecision(decision Decision) {
	if cs.PendingDecision != nil && cs.PendingDecision.ID == decision.DecisionID {
		cs.PendingDecision = nil

		if len(cs.History) > 0 {
			cs.History[len(cs.History)-1].Decision = &decision
		}
	}
	cs.SessionState.LastAccessed = time.Now()
}
func (cs *ConversationState) AddArtifact(artifact Artifact) {
	artifact.ID = generateArtifactID()
	artifact.CreatedAt = time.Now()
	artifact.UpdatedAt = time.Now()
	cs.Artifacts[artifact.ID] = artifact
	cs.SessionState.LastAccessed = time.Now()
}
func (cs *ConversationState) UpdateArtifact(artifactID, content string) {
	if artifact, exists := cs.Artifacts[artifactID]; exists {
		artifact.Content = content
		artifact.UpdatedAt = time.Now()
		cs.Artifacts[artifactID] = artifact
		cs.SessionState.LastAccessed = time.Now()
	}
}
func (cs *ConversationState) GetArtifactsByType(artifactType string) []Artifact {
	var artifacts []Artifact
	for _, artifact := range cs.Artifacts {
		if artifact.Type == artifactType {
			artifacts = append(artifacts, artifact)
		}
	}
	return artifacts
}
func (cs *ConversationState) GetLatestTurn() *ConversationTurn {
	if len(cs.History) == 0 {
		return nil
	}
	return &cs.History[len(cs.History)-1]
}
func (cs *ConversationState) CanProceedToStage(stage core.ConversationStage) bool {

	switch stage {
	case core.ConversationStageAnalyze:

		repoURL := ""
		if cs.SessionState.Metadata != nil {
			if url, ok := cs.SessionState.Metadata["repo_url"].(string); ok {
				repoURL = url
			}
		}
		return repoURL != ""

	case core.ConversationStageBuild:

		repoAnalysisExists := false
		if cs.SessionState.Metadata != nil {
			if repoAnalysis, ok := cs.SessionState.Metadata["repo_analysis"].(map[string]interface{}); ok {
				repoAnalysisExists = len(repoAnalysis) > 0
			}
		}
		return repoAnalysisExists

	case core.ConversationStageDeploy:

		imageBuilt := false
		if cs.SessionState.Metadata != nil {
			if built, ok := cs.SessionState.Metadata["image_built"].(bool); ok {
				imageBuilt = built
			}
		}
		return imageBuilt

	case core.ConversationStageScan:

		imageRef := ""
		if cs.SessionState.Metadata != nil {
			if ref, ok := cs.SessionState.Metadata["image_ref"].(string); ok {
				imageRef = ref
			}
		}
		return imageRef != ""

	default:
		return false
	}
}
func (cs *ConversationState) GetStageProgress() StageProgress {

	stages := []core.ConversationStage{
		core.ConversationStagePreFlight,
		core.ConversationStageAnalyze,
		core.ConversationStageBuild,
		core.ConversationStageDeploy,
		core.ConversationStageScan,
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

type StageProgress struct {
	CurrentStage    core.ConversationStage   `json:"current_stage"`
	CurrentStep     int                      `json:"current_step"`
	TotalSteps      int                      `json:"total_steps"`
	Percentage      int                      `json:"percentage"`
	CompletedStages []core.ConversationStage `json:"completed_stages"`
	RemainingStages []core.ConversationStage `json:"remaining_stages"`
}

func generateTurnID() string {
	return fmt.Sprintf("turn-%d", time.Now().UnixNano())
}

func generateArtifactID() string {
	return fmt.Sprintf("artifact-%d", time.Now().UnixNano())
}
