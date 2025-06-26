package conversation

import (
	"fmt"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/internal/types"
)

// ConversationResponse represents the response to a user prompt
type ConversationResponse struct {
	SessionID string                  `json:"session_id"`
	Message   string                  `json:"message"`
	Stage     types.ConversationStage `json:"stage"`
	Status    ResponseStatus          `json:"status"`
	Options   []Option                `json:"options,omitempty"`
	Artifacts []ArtifactSummary       `json:"artifacts,omitempty"`
	NextSteps []string                `json:"next_steps,omitempty"`
	Progress  *StageProgress          `json:"progress,omitempty"`
	ToolCalls []ToolCall              `json:"tool_calls,omitempty"`

	// Auto-advance support
	RequiresInput bool                     `json:"requires_input"`         // If false, can auto-advance
	NextStage     *types.ConversationStage `json:"next_stage,omitempty"`   // Stage to advance to
	AutoAdvance   *AutoAdvanceConfig       `json:"auto_advance,omitempty"` // Auto-advance configuration

	// Structured forms support
	Form *StructuredForm `json:"form,omitempty"` // Structured form for gathering input
}

// ResponseStatus indicates the status of a response
type ResponseStatus string

const (
	ResponseStatusSuccess      ResponseStatus = "success"
	ResponseStatusError        ResponseStatus = "error"
	ResponseStatusWaitingInput ResponseStatus = "waiting_input"
	ResponseStatusProcessing   ResponseStatus = "processing"
	ResponseStatusWarning      ResponseStatus = "warning"
)

// AutoAdvanceConfig controls automatic progression between stages
type AutoAdvanceConfig struct {
	DelaySeconds  int     `json:"delay_seconds,omitempty"`  // Delay before auto-advance (0 = immediate)
	Confidence    float64 `json:"confidence,omitempty"`     // Confidence level (0.0-1.0)
	Reason        string  `json:"reason,omitempty"`         // Why auto-advancing
	CanCancel     bool    `json:"can_cancel,omitempty"`     // User can cancel auto-advance
	DefaultAction string  `json:"default_action,omitempty"` // Default action to take
}

// ArtifactSummary provides a lightweight view of an artifact
type ArtifactSummary struct {
	ID        string    `json:"id"`
	Type      string    `json:"type"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
	Size      int       `json:"size_bytes"`
}

// Note: InternalToolOrchestrator is imported from the orchestration package

// Note: UserPreferences and ResourceLimits are defined in conversation_state.go

// Auto-advance helper methods

// WithAutoAdvance configures the response for automatic progression to the next stage
func (r *ConversationResponse) WithAutoAdvance(nextStage types.ConversationStage, config AutoAdvanceConfig) *ConversationResponse {
	r.RequiresInput = false
	r.NextStage = &nextStage
	r.AutoAdvance = &config
	return r
}

// WithUserInput marks the response as requiring user input (blocks auto-advance)
func (r *ConversationResponse) WithUserInput() *ConversationResponse {
	r.RequiresInput = true
	r.NextStage = nil
	r.AutoAdvance = nil
	return r
}

// CanAutoAdvance returns true if this response supports automatic progression
func (r *ConversationResponse) CanAutoAdvance() bool {
	return !r.RequiresInput && r.NextStage != nil
}

// ShouldAutoAdvance determines if auto-advance should be triggered based on user preferences
func (r *ConversationResponse) ShouldAutoAdvance(userPrefs types.UserPreferences) bool {
	if !r.CanAutoAdvance() {
		return false
	}

	// Check if user has autopilot enabled (SkipConfirmations)
	if !userPrefs.SkipConfirmations {
		return false
	}

	// Check confidence threshold if specified
	if r.AutoAdvance != nil && r.AutoAdvance.Confidence > 0 {
		// Only auto-advance if confidence is high enough (>= 0.8)
		return r.AutoAdvance.Confidence >= 0.8
	}

	return true
}

// GetAutoAdvanceMessage returns a message explaining the auto-advance behavior
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

// Note: ErrorHandler is now in the errors package for centralized error management

// Note: InternalToolOrchestrator is imported from the orchestration package
