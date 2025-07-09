package conversation

import (
	"fmt"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/domain/shared"
)

type ConversationResponse struct {
	SessionID     string                    `json:"session_id"`
	Message       string                    `json:"message"`
	Stage         shared.ConversationStage  `json:"stage"`
	Status        ResponseStatus            `json:"status"`
	Options       []Option                  `json:"options,omitempty"`
	Artifacts     []ArtifactSummary         `json:"artifacts,omitempty"`
	NextSteps     []string                  `json:"next_steps,omitempty"`
	Progress      *StageProgress            `json:"progress,omitempty"`
	ToolCalls     []ToolCall                `json:"tool_calls,omitempty"`
	RequiresInput bool                      `json:"requires_input"`
	NextStage     *shared.ConversationStage `json:"next_stage,omitempty"`
	AutoAdvance   *AutoAdvanceConfig        `json:"auto_advance,omitempty"`
	Form          *StructuredForm           `json:"form,omitempty"`
	ErrorRecovery *ErrorRecoveryGuidance    `json:"error_recovery,omitempty"`
}
type ResponseStatus string

const (
	ResponseStatusSuccess      ResponseStatus = "success"
	ResponseStatusError        ResponseStatus = "error"
	ResponseStatusWaitingInput ResponseStatus = "waiting_input"
	ResponseStatusProcessing   ResponseStatus = "processing"
	ResponseStatusWarning      ResponseStatus = "warning"
)

type AutoAdvanceConfig struct {
	DelaySeconds  int     `json:"delay_seconds,omitempty"`
	Confidence    float64 `json:"confidence,omitempty"`
	Reason        string  `json:"reason,omitempty"`
	CanCancel     bool    `json:"can_cancel,omitempty"`
	DefaultAction string  `json:"default_action,omitempty"`
}
type ArtifactSummary struct {
	ID        string    `json:"id"`
	Type      string    `json:"type"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
	Size      int       `json:"size_bytes"`
}
type ErrorRecoveryGuidance struct {
	ErrorType          string   `json:"error_type"`
	AttemptCount       int      `json:"attempt_count"`
	ProgressAssessment string   `json:"progress_assessment"`
	FocusAreas         []string `json:"focus_areas"`
	RecommendedTools   []string `json:"recommended_tools"`
	NextSteps          []string `json:"next_steps"`
	SuccessIndicators  []string `json:"success_indicators"`
	AvoidRepeating     []string `json:"avoid_repeating"`
	IsProgressive      bool     `json:"is_progressive"`
}

func (r *ConversationResponse) WithAutoAdvance(nextStage shared.ConversationStage, config AutoAdvanceConfig) *ConversationResponse {
	r.RequiresInput = false
	r.NextStage = &nextStage
	r.AutoAdvance = &config
	return r
}
func (r *ConversationResponse) WithUserInput() *ConversationResponse {
	r.RequiresInput = true
	r.NextStage = nil
	r.AutoAdvance = nil
	return r
}
func (r *ConversationResponse) CanAutoAdvance() bool {
	return !r.RequiresInput && r.NextStage != nil
}
func (r *ConversationResponse) WithErrorRecovery(guidance *ErrorRecoveryGuidance) *ConversationResponse {
	r.ErrorRecovery = guidance
	r.Status = ResponseStatusError
	return r
}
func (r *ConversationResponse) HasErrorRecovery() bool {
	return r.ErrorRecovery != nil
}
func (r *ConversationResponse) ShouldAutoAdvance(userPrefs shared.UserPreferences) bool {
	if !r.CanAutoAdvance() {
		return false
	}
	if !userPrefs.SkipConfirmations {
		return false
	}
	if r.AutoAdvance != nil && r.AutoAdvance.Confidence > 0 {

		return r.AutoAdvance.Confidence >= 0.8
	}

	return true
}
func (r *ConversationResponse) GetAutoAdvanceMessage() string {
	if !r.CanAutoAdvance() || r.AutoAdvance == nil {
		return ""
	}

	baseMsg := r.Message
	if r.AutoAdvance.Reason != "" {
		baseMsg += fmt.Sprintf("\n\n🤖 **Autopilot**: %s", r.AutoAdvance.Reason)
	}

	if r.AutoAdvance.DelaySeconds > 0 {
		baseMsg += fmt.Sprintf(" (advancing in %d seconds)", r.AutoAdvance.DelaySeconds)
	} else {
		baseMsg += " (advancing automatically)"
	}

	if r.AutoAdvance.CanCancel {
		baseMsg += "\n\n💡 You can type 'stop' or 'wait' to pause autopilot mode."
	}

	return baseMsg
}
func convertFromTypesStage(stage shared.ConversationStage) shared.ConversationStage {
	// This function now seems redundant since both input and output are shared.ConversationStage
	// Keeping it for compatibility but it just returns the input
	return stage
}
func mapMCPStageToDetailedStage(stage shared.ConversationStage, _ map[string]interface{}) shared.ConversationStage {
	// This function now seems redundant since both input and output are shared.ConversationStage
	// Keeping it for compatibility but it just returns the input
	return stage
}
