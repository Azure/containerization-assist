package prompts

import "context"

// Manager defines the interface for prompt template management.
// This interface is implemented by infrastructure layer.
type Manager interface {
	// GetPrompt retrieves a prompt template by ID
	GetPrompt(ctx context.Context, promptID string) (Template, error)

	// ListPrompts lists all available prompts
	ListPrompts(ctx context.Context) ([]PromptSummary, error)

	// RenderPrompt renders a prompt template with given variables
	RenderPrompt(ctx context.Context, promptID string, variables map[string]interface{}) (string, error)

	// RegisterPrompt registers a new prompt template
	RegisterPrompt(ctx context.Context, template Template) error
}

// Template represents a prompt template
type Template struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Content     string                 `json:"content"`
	Variables   []Variable             `json:"variables"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// Variable represents a template variable
type Variable struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Required    bool   `json:"required"`
	Default     string `json:"default,omitempty"`
	Description string `json:"description,omitempty"`
}

// PromptSummary represents a lightweight prompt summary
type PromptSummary struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
}
