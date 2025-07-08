package conversation

import (
	"context"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/Azure/container-kit/pkg/common/validation"
	"github.com/Azure/container-kit/pkg/common/validation-core/core"
)

// ConversationValidator validates input for conversation/chat tools
type ConversationValidator struct {
	*validation.BaseValidator
}

// NewConversationValidator creates a new conversation validator
func NewConversationValidator() *ConversationValidator {
	validator := &ConversationValidator{
		BaseValidator: validation.NewBaseValidator("conversation_validator", "1.0.0"),
	}

	// Add supported input types
	validator.AddSupportedType("ChatToolArgs")

	// Add validation rules
	validator.AddRule(validation.ValidationRule{
		Name:        "message_required",
		Description: "Message is required for conversation",
		Severity:    core.SeverityHigh,
		Category:    validation.CategoryRequired,
		Enabled:     true,
	})

	validator.AddRule(validation.ValidationRule{
		Name:        "message_length_limit",
		Description: "Message must not exceed maximum length",
		Severity:    core.SeverityMedium,
		Category:    validation.CategoryFormat,
		Enabled:     true,
		Config: validation.ValidationRuleConfig{
			MaxLength: 10000, // 10k characters max for conversation messages
		},
	})

	validator.AddRule(validation.ValidationRule{
		Name:        "no_empty_message",
		Description: "Message cannot be empty or only whitespace",
		Severity:    core.SeverityMedium,
		Category:    validation.CategoryRequired,
		Enabled:     true,
	})

	validator.AddRule(validation.ValidationRule{
		Name:        "valid_utf8",
		Description: "Message must contain valid UTF-8 text",
		Severity:    core.SeverityMedium,
		Category:    validation.CategoryFormat,
		Enabled:     true,
	})

	return validator
}

// Validate performs comprehensive input validation for conversation tools
func (v *ConversationValidator) Validate(ctx context.Context, input interface{}) error {
	args, ok := input.(*ChatToolArgs)
	if !ok {
		return validation.NewError("input", "INVALID_TYPE", "expected ChatToolArgs")
	}

	var errors []*validation.ValidationError

	// Validate message (required)
	if err := v.validateMessage(args.Message); err != nil {
		errors = append(errors, err)
	}

	// Validate session ID (optional but if provided should be valid format)
	if args.SessionID != "" {
		if err := v.validateSessionID(args.SessionID); err != nil {
			errors = append(errors, err)
		}
	}

	// Return first error if any (following simple interface)
	if len(errors) > 0 {
		return errors[0]
	}

	return nil
}

// ValidateInput provides comprehensive validation with rich error reporting
func (v *ConversationValidator) ValidateInput(ctx context.Context, input *ChatToolArgs) *core.Result[*ChatToolArgs] {
	result := validation.CreateResult[*ChatToolArgs](v.GetName(), v.GetVersion())

	var errors []*validation.ValidationError

	// Validate message (required)
	if err := v.validateMessage(input.Message); err != nil {
		errors = append(errors, err)
	}

	// Validate session ID (optional)
	if input.SessionID != "" {
		if err := v.validateSessionID(input.SessionID); err != nil {
			errors = append(errors, err)
		}
	}

	// Apply all validation errors to the result
	validation.ApplyValidationErrors(result, errors)

	// Set data and suggestions if validation passed
	if !result.HasErrors() {
		result.SetData(input)
		result.AddSuggestion("Message validated successfully for conversation processing")

		// Add helpful suggestions based on message content
		if len(input.Message) > 5000 {
			result.AddSuggestion("Long messages may take more time to process")
		}

		if strings.Contains(strings.ToLower(input.Message), "error") ||
			strings.Contains(strings.ToLower(input.Message), "problem") {
			result.AddSuggestion("Consider providing specific error details for better assistance")
		}
	}

	return result
}

// validateMessage validates the conversation message
func (v *ConversationValidator) validateMessage(message string) *validation.ValidationError {
	// Check if message is empty
	if message == "" {
		return validation.NewRequiredFieldError("message")
	}

	// Check if message is only whitespace
	if strings.TrimSpace(message) == "" {
		return validation.NewInvalidValueError("message", message, "cannot be empty or only whitespace")
	}

	// Check UTF-8 validity
	if !utf8.ValidString(message) {
		return validation.NewInvalidFormatError("message", "valid UTF-8 text")
	}

	// Check length constraints
	const maxMessageLength = 10000
	if utf8.RuneCountInString(message) > maxMessageLength {
		return validation.NewInvalidValueError("message", len(message),
			fmt.Sprintf("maximum length is %d characters", maxMessageLength))
	}

	// Check for potentially problematic content
	if err := v.validateMessageContent(message); err != nil {
		return err
	}

	return nil
}

// validateMessageContent checks for potentially problematic message content
func (v *ConversationValidator) validateMessageContent(message string) *validation.ValidationError {
	// Check for excessively repetitive content
	if v.isExcessivelyRepetitive(message) {
		return validation.NewInvalidValueError("message", message, "contains excessive repetitive content")
	}

	// Check for potential injection attempts (basic)
	if v.containsSuspiciousPatterns(message) {
		return validation.NewInvalidValueError("message", message, "contains potentially unsafe patterns")
	}

	return nil
}

// validateSessionID validates the session ID format
func (v *ConversationValidator) validateSessionID(sessionID string) *validation.ValidationError {
	if sessionID == "" {
		return nil // Session ID is optional
	}

	// Basic session ID format validation
	if len(sessionID) < 8 {
		return validation.NewInvalidValueError("session_id", sessionID, "minimum length is 8 characters")
	}

	if len(sessionID) > 256 {
		return validation.NewInvalidValueError("session_id", sessionID, "maximum length is 256 characters")
	}

	// Check for valid characters (alphanumeric, hyphens, underscores)
	for _, char := range sessionID {
		if !((char >= 'a' && char <= 'z') ||
			(char >= 'A' && char <= 'Z') ||
			(char >= '0' && char <= '9') ||
			char == '-' || char == '_') {
			return validation.NewInvalidFormatError("session_id", "alphanumeric characters, hyphens, and underscores only")
		}
	}

	return nil
}

// isExcessivelyRepetitive checks if message contains too much repetitive content
func (v *ConversationValidator) isExcessivelyRepetitive(message string) bool {
	// Simple heuristic: if any single character appears more than 50% of the message
	if len(message) < 10 {
		return false // Too short to be considered repetitive
	}

	charCount := make(map[rune]int)
	totalChars := 0

	for _, char := range message {
		if char != ' ' && char != '\n' && char != '\t' { // Ignore whitespace
			charCount[char]++
			totalChars++
		}
	}

	for _, count := range charCount {
		if float64(count)/float64(totalChars) > 0.5 {
			return true
		}
	}

	return false
}

// containsSuspiciousPatterns checks for potentially malicious patterns
func (v *ConversationValidator) containsSuspiciousPatterns(message string) bool {
	// Convert to lowercase for case-insensitive matching
	lowerMessage := strings.ToLower(message)

	// Check for potential script injection patterns
	suspiciousPatterns := []string{
		"<script",
		"javascript:",
		"eval(",
		"document.cookie",
		"window.location",
		"alert(",
		"confirm(",
		"prompt(",
	}

	for _, pattern := range suspiciousPatterns {
		if strings.Contains(lowerMessage, pattern) {
			return true
		}
	}

	// Check for potential command injection patterns
	commandPatterns := []string{
		"rm -rf",
		"sudo ",
		"; cat ",
		"| sh",
		"& del",
		"&& rm",
	}

	for _, pattern := range commandPatterns {
		if strings.Contains(lowerMessage, pattern) {
			return true
		}
	}

	return false
}

// GetValidationRules returns the validation rules for this validator
func (v *ConversationValidator) GetValidationRules() []validation.ValidationRule {
	return v.BaseValidator.GetValidationRules()
}

// GetSupportedInputTypes returns the input types this validator supports
func (v *ConversationValidator) GetSupportedInputTypes() []string {
	return v.BaseValidator.GetSupportedInputTypes()
}

// ValidateConversationFlow validates conversation flow and context
func (v *ConversationValidator) ValidateConversationFlow(ctx context.Context, args *ChatToolArgs, conversationHistory []string) *core.Result[*ChatToolArgs] {
	result := v.ValidateInput(ctx, args)

	// Additional flow-specific validations
	if len(conversationHistory) > 100 {
		result.AddSuggestion("Long conversation history - consider starting a new session for better performance")
	}

	// Check for conversation loops (same message repeated)
	if len(conversationHistory) > 0 {
		lastMessage := conversationHistory[len(conversationHistory)-1]
		if strings.TrimSpace(args.Message) == strings.TrimSpace(lastMessage) {
			warning := core.NewWarning("REPEATED_MESSAGE", "This message appears to be identical to the previous one")
			result.AddWarning(warning)
		}
	}

	return result
}
