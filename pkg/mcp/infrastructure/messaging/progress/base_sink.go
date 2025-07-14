// Package progress provides shared functionality for progress sink implementations
package progress

import (
	"fmt"
	"log/slog"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/domain/progress"
)

// baseSink provides common functionality shared between sink implementations
type baseSink struct {
	logger        *slog.Logger
	lastHeartbeat time.Time
}

// newBaseSink creates a new base sink with common functionality
func newBaseSink(logger *slog.Logger, component string) *baseSink {
	return &baseSink{
		logger: logger.With("component", component),
	}
}

// shouldThrottleHeartbeat determines if a heartbeat update should be throttled
func (b *baseSink) shouldThrottleHeartbeat(u progress.Update, throttleDuration time.Duration) bool {
	if kind, ok := u.UserMeta["kind"].(string); ok && kind == "heartbeat" {
		if time.Since(b.lastHeartbeat) < throttleDuration {
			b.logger.Debug("Throttling heartbeat update")
			return true
		}
		b.lastHeartbeat = time.Now()
	}
	return false
}

// extractStepName gets step_name from metadata
func (b *baseSink) extractStepName(u progress.Update) string {
	if stepName, ok := u.UserMeta["step_name"].(string); ok {
		return stepName
	}
	return ""
}

// extractSubstepName gets substep_name from metadata
func (b *baseSink) extractSubstepName(u progress.Update) string {
	if substepName, ok := u.UserMeta["substep_name"].(string); ok {
		return substepName
	}
	return ""
}

// extractCanAbort gets can_abort flag from metadata
func (b *baseSink) extractCanAbort(u progress.Update) bool {
	if canAbort, ok := u.UserMeta["can_abort"].(bool); ok {
		return canAbort
	}
	return false
}

// extractAttempt gets retry attempt number from metadata
func (b *baseSink) extractAttempt(u progress.Update) int {
	if attempt, ok := u.UserMeta["attempt"].(int); ok {
		return attempt
	}
	return 0
}

// extractError gets error message from metadata
func (b *baseSink) extractError(u progress.Update) string {
	if errorMsg, ok := u.UserMeta["error"].(string); ok {
		return errorMsg
	}
	return ""
}

// formatETA creates a human-readable ETA string
func (b *baseSink) formatETA(eta time.Duration) string {
	if eta <= 0 {
		return ""
	}
	return fmt.Sprintf("ETA: %s", eta.Round(time.Second))
}

// formatETAMs returns ETA in milliseconds for API consumption
func (b *baseSink) formatETAMs(eta time.Duration) int64 {
	if eta <= 0 {
		return 0
	}
	return eta.Milliseconds()
}

// buildEnhancedMessage creates a rich message with step and sub-step information
func (b *baseSink) buildEnhancedMessage(u progress.Update) string {
	stepName := b.extractStepName(u)
	substepName := b.extractSubstepName(u)

	// If we have a step name, use it instead of generic message
	if stepName != "" {
		message := stepName

		// Add sub-step information if available
		if substepName != "" {
			message = fmt.Sprintf("%s (%s)", message, substepName)
		}

		// Add retry information
		if attempt := b.extractAttempt(u); attempt > 1 {
			message = fmt.Sprintf("%s - Attempt %d", message, attempt)
		}

		// Add additional context for specific statuses
		switch u.Status {
		case "failed":
			if errorMsg := b.extractError(u); len(errorMsg) > 0 {
				// Show first 50 chars of error for brevity
				if len(errorMsg) > 50 {
					errorMsg = errorMsg[:47] + "..."
				}
				message = fmt.Sprintf("%s - Error: %s", message, errorMsg)
			}
		case "generating": // For LLM operations
			if tokensGenerated, ok := u.UserMeta["tokens_generated"].(int); ok {
				if estimatedTotal, ok := u.UserMeta["estimated_total"].(int); ok {
					message = fmt.Sprintf("AI generating tokens: %d/%d", tokensGenerated, estimatedTotal)
				}
			}
		}

		return message
	}

	// Fallback to original message
	return u.Message
}

// buildPayloadMetadata creates the standard metadata block for API consumption
func (b *baseSink) buildPayloadMetadata(u progress.Update) map[string]interface{} {
	return map[string]interface{}{
		"step":       u.Step,
		"total":      u.Total,
		"percentage": u.Percentage,
		"status":     u.Status,
		"eta_ms":     b.formatETAMs(u.ETA),
		"user_meta":  u.UserMeta,
	}
}

// buildBasePayload creates the common payload structure for both MCP and API sinks
func (b *baseSink) buildBasePayload(u progress.Update) map[string]interface{} {
	payload := map[string]interface{}{
		"step":       u.Step,
		"total":      u.Total,
		"percentage": u.Percentage, // TOP-LEVEL for AI consumption
		"status":     u.Status,     // TOP-LEVEL for AI consumption
		"message":    u.Message,
		"trace_id":   u.TraceID,
		"started_at": u.StartedAt,
		"metadata":   b.buildPayloadMetadata(u), // Backward compatibility
	}

	// Enhanced fields for rich AI experience
	if u.ETA > 0 {
		payload["eta_ms"] = b.formatETAMs(u.ETA)
	}

	if stepName := b.extractStepName(u); stepName != "" {
		payload["step_name"] = stepName
	}

	if substepName := b.extractSubstepName(u); substepName != "" {
		payload["substep_name"] = substepName
	}

	// Always include can_abort flag
	payload["can_abort"] = b.extractCanAbort(u)

	return payload
}

// logDebugInfo logs detailed debug information about the progress update
func (b *baseSink) logDebugInfo(u progress.Update, sinkType string) {
	b.logger.Debug(fmt.Sprintf("%s progress update", sinkType),
		"step", u.Step,
		"total", u.Total,
		"percentage", u.Percentage,
		"step_name", b.extractStepName(u),
		"substep_name", b.extractSubstepName(u),
		"status", u.Status,
		"message", u.Message,
		"attempt", b.extractAttempt(u))
}

// StatusInfo represents status display information
type StatusInfo struct {
	Icon        string
	DisplayName string
	IsTerminal  bool // Whether this status indicates completion
}

// getStatusInfo returns standardized status information for display
func (b *baseSink) getStatusInfo(u progress.Update) StatusInfo {
	attempt := b.extractAttempt(u)

	switch u.Status {
	case "completed":
		return StatusInfo{Icon: "✅", DisplayName: "Completed", IsTerminal: true}
	case "failed":
		return StatusInfo{Icon: "❌", DisplayName: "Failed", IsTerminal: true}
	case "retrying":
		icon := "🔄"
		displayName := "Retrying"
		if attempt > 0 {
			icon = fmt.Sprintf("🔄(%d)", attempt)
			displayName = fmt.Sprintf("Retrying (attempt %d)", attempt)
		}
		return StatusInfo{Icon: icon, DisplayName: displayName, IsTerminal: false}
	case "started":
		return StatusInfo{Icon: "🚀", DisplayName: "Started", IsTerminal: false}
	case "running":
		return StatusInfo{Icon: "⚡", DisplayName: "Running", IsTerminal: false}
	case "generating": // For LLM token generation
		return StatusInfo{Icon: "🧠", DisplayName: "Generating", IsTerminal: false}
	default:
		return StatusInfo{Icon: "▶️", DisplayName: u.Status, IsTerminal: false}
	}
}
