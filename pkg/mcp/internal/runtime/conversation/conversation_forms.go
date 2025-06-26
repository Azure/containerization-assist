package conversation

import (
	"encoding/json"
	"strings"
)

// StructuredForm represents a form with multiple related fields
type StructuredForm struct {
	ID          string      `json:"id"`
	Title       string      `json:"title"`
	Description string      `json:"description"`
	Fields      []FormField `json:"fields"`
	CanSkip     bool        `json:"can_skip"`
	SkipLabel   string      `json:"skip_label,omitempty"`
}

// FormField represents a single field in a structured form
type FormField struct {
	ID           string           `json:"id"`
	Label        string           `json:"label"`
	Type         FormFieldType    `json:"type"`
	Required     bool             `json:"required"`
	DefaultValue interface{}      `json:"default_value,omitempty"`
	Options      []FormOption     `json:"options,omitempty"`
	Validation   *FieldValidation `json:"validation,omitempty"`
	Description  string           `json:"description,omitempty"`
	Placeholder  string           `json:"placeholder,omitempty"`
}

// FormFieldType defines the type of form field
type FormFieldType string

const (
	FieldTypeText        FormFieldType = "text"
	FieldTypeSelect      FormFieldType = "select"
	FieldTypeMultiSelect FormFieldType = "multi_select"
	FieldTypeNumber      FormFieldType = "number"
	FieldTypeBoolean     FormFieldType = "boolean"
	FieldTypeTextArea    FormFieldType = "textarea"
	FieldTypePassword    FormFieldType = "password"
	FieldTypeEmail       FormFieldType = "email"
	FieldTypeURL         FormFieldType = "url"
)

// FormOption represents an option in a select field
type FormOption struct {
	Value       string `json:"value"`
	Label       string `json:"label"`
	Description string `json:"description,omitempty"`
	Recommended bool   `json:"recommended,omitempty"`
}

// FieldValidation defines validation rules for a field
type FieldValidation struct {
	MinLength *int     `json:"min_length,omitempty"`
	MaxLength *int     `json:"max_length,omitempty"`
	Min       *float64 `json:"min,omitempty"`
	Max       *float64 `json:"max,omitempty"`
	Pattern   string   `json:"pattern,omitempty"`
	Message   string   `json:"message,omitempty"`
}

// FormResponse represents a user's response to a structured form
type FormResponse struct {
	FormID  string                 `json:"form_id"`
	Values  map[string]interface{} `json:"values"`
	Skipped bool                   `json:"skipped"`
}

// ConversationResponseWithForm extends ConversationResponse to include forms
type ConversationResponseWithForm struct {
	*ConversationResponse
	Form *StructuredForm `json:"form,omitempty"`
}

// Form creation helpers

// NewRepositoryAnalysisForm creates a form for repository analysis preferences
func NewRepositoryAnalysisForm() *StructuredForm {
	return &StructuredForm{
		ID:          "repository_analysis",
		Title:       "Repository Analysis Preferences",
		Description: "Configure how the repository should be analyzed",
		CanSkip:     true,
		SkipLabel:   "Use defaults",
		Fields: []FormField{
			{
				ID:           "branch",
				Label:        "Git Branch",
				Type:         FieldTypeText,
				Required:     false,
				DefaultValue: "main",
				Description:  "Which branch to analyze (default: main)",
				Placeholder:  "main",
			},
			{
				ID:           "skip_file_tree",
				Label:        "Skip File Tree Analysis",
				Type:         FieldTypeBoolean,
				Required:     false,
				DefaultValue: false,
				Description:  "Skip detailed file structure analysis for faster processing",
			},
			{
				ID:           "optimization",
				Label:        "Optimization Priority",
				Type:         FieldTypeSelect,
				Required:     false,
				DefaultValue: "balanced",
				Description:  "What aspect should be prioritized in the analysis",
				Options: []FormOption{
					{Value: "speed", Label: "Speed", Description: "Fast analysis, basic recommendations"},
					{Value: "balanced", Label: "Balanced", Description: "Good balance of speed and thoroughness", Recommended: true},
					{Value: "thorough", Label: "Thorough", Description: "Comprehensive analysis, may take longer"},
				},
			},
		},
	}
}

// NewDockerfileConfigForm creates a form for Dockerfile configuration
func NewDockerfileConfigForm() *StructuredForm {
	return &StructuredForm{
		ID:          "dockerfile_config",
		Title:       "Dockerfile Configuration",
		Description: "Configure your Dockerfile generation preferences",
		CanSkip:     true,
		SkipLabel:   "Use smart defaults",
		Fields: []FormField{
			{
				ID:          "base_image",
				Label:       "Base Image",
				Type:        FieldTypeText,
				Required:    false,
				Description: "Custom base image (leave empty for auto-selection)",
				Placeholder: "e.g., node:18-alpine, python:3.11-slim",
			},
			{
				ID:           "optimization",
				Label:        "Optimization Strategy",
				Type:         FieldTypeSelect,
				Required:     false,
				DefaultValue: "size",
				Description:  "Primary optimization goal for the Dockerfile",
				Options: []FormOption{
					{Value: "size", Label: "Size", Description: "Minimize image size", Recommended: true},
					{Value: "speed", Label: "Speed", Description: "Optimize for build and runtime speed"},
					{Value: "security", Label: "Security", Description: "Maximize security hardening"},
				},
			},
			{
				ID:           "include_health_check",
				Label:        "Include Health Check",
				Type:         FieldTypeBoolean,
				Required:     false,
				DefaultValue: true,
				Description:  "Add a health check instruction to the Dockerfile",
			},
			{
				ID:          "platform",
				Label:       "Target Platform",
				Type:        FieldTypeSelect,
				Required:    false,
				Description: "Target architecture for the container",
				Options: []FormOption{
					{Value: "", Label: "Auto-detect", Recommended: true},
					{Value: "linux/amd64", Label: "Linux AMD64", Description: "x86_64 architecture"},
					{Value: "linux/arm64", Label: "Linux ARM64", Description: "ARM 64-bit architecture"},
					{Value: "linux/arm/v7", Label: "Linux ARM v7", Description: "ARM 32-bit architecture"},
				},
			},
		},
	}
}

// NewKubernetesDeploymentForm creates a form for Kubernetes deployment settings
func NewKubernetesDeploymentForm() *StructuredForm {
	return &StructuredForm{
		ID:          "kubernetes_deployment",
		Title:       "Kubernetes Deployment Configuration",
		Description: "Configure your Kubernetes deployment settings",
		CanSkip:     false, // This form is usually required
		Fields: []FormField{
			{
				ID:          "app_name",
				Label:       "Application Name",
				Type:        FieldTypeText,
				Required:    true,
				Description: "Name for your application in Kubernetes",
				Placeholder: "my-app",
				Validation: &FieldValidation{
					MinLength: intPtr(1),
					MaxLength: intPtr(63),
					Pattern:   "^[a-z0-9]([a-z0-9-]*[a-z0-9])?$",
					Message:   "Must be valid Kubernetes name (lowercase, alphanumeric, hyphens)",
				},
			},
			{
				ID:           "namespace",
				Label:        "Namespace",
				Type:         FieldTypeText,
				Required:     false,
				DefaultValue: "default",
				Description:  "Kubernetes namespace to deploy to",
				Placeholder:  "default",
			},
			{
				ID:           "replicas",
				Label:        "Number of Replicas",
				Type:         FieldTypeNumber,
				Required:     false,
				DefaultValue: 3,
				Description:  "Number of pod replicas to run",
				Validation: &FieldValidation{
					Min:     float64Ptr(1),
					Max:     float64Ptr(20),
					Message: "Must be between 1 and 20 replicas",
				},
			},
			{
				ID:           "service_type",
				Label:        "Service Type",
				Type:         FieldTypeSelect,
				Required:     false,
				DefaultValue: "ClusterIP",
				Description:  "How the service should be exposed",
				Options: []FormOption{
					{Value: "ClusterIP", Label: "ClusterIP", Description: "Internal cluster access only", Recommended: true},
					{Value: "NodePort", Label: "NodePort", Description: "Expose on each node's IP at a static port"},
					{Value: "LoadBalancer", Label: "LoadBalancer", Description: "Expose via cloud load balancer"},
				},
			},
		},
	}
}

// NewRegistryConfigForm creates a form for registry configuration
func NewRegistryConfigForm() *StructuredForm {
	return &StructuredForm{
		ID:          "registry_config",
		Title:       "Container Registry Configuration",
		Description: "Configure where to push your container image",
		CanSkip:     true,
		SkipLabel:   "Skip push (local only)",
		Fields: []FormField{
			{
				ID:          "registry_url",
				Label:       "Registry URL",
				Type:        FieldTypeURL,
				Required:    true,
				Description: "Container registry URL",
				Placeholder: "docker.io, gcr.io/project, myregistry.azurecr.io",
			},
			{
				ID:          "image_name",
				Label:       "Image Name",
				Type:        FieldTypeText,
				Required:    false,
				Description: "Custom image name (auto-generated if empty)",
				Placeholder: "my-app",
			},
			{
				ID:           "tag",
				Label:        "Image Tag",
				Type:         FieldTypeText,
				Required:     false,
				DefaultValue: "latest",
				Description:  "Image tag to use",
				Placeholder:  "latest, v1.0.0, dev",
			},
		},
	}
}

// Helper functions for form processing

// ParseFormResponse parses a form response from JSON or structured input
func ParseFormResponse(input, expectedFormID string) (*FormResponse, error) {
	// Try to parse as JSON first
	var response FormResponse
	if err := json.Unmarshal([]byte(input), &response); err == nil {
		if response.FormID == expectedFormID {
			return &response, nil
		}
	}

	// Fall back to parsing natural language responses
	// This is a simplified parser - could be enhanced with LLM assistance
	response = FormResponse{
		FormID: expectedFormID,
		Values: make(map[string]interface{}),
	}

	// Check for skip indicators
	lowerInput := strings.ToLower(input)
	if strings.Contains(lowerInput, "skip") || strings.Contains(lowerInput, "default") {
		response.Skipped = true
		return &response, nil
	}

	// Basic key-value parsing (example: "branch=main optimization=speed")
	// This could be enhanced with more sophisticated parsing
	parts := strings.Fields(input)
	for _, part := range parts {
		if strings.Contains(part, "=") {
			kv := strings.SplitN(part, "=", 2)
			if len(kv) == 2 {
				response.Values[kv[0]] = kv[1]
			}
		}
	}

	return &response, nil
}

// ApplyFormResponse applies form values to conversation state
func (form *StructuredForm) ApplyFormResponse(response *FormResponse, state *ConversationState) error {
	if response.Skipped {
		// Use defaults - mark in context that form was skipped
		state.Context[form.ID+"_skipped"] = true
		return nil
	}

	// Apply field values to conversation context
	for fieldID, value := range response.Values {
		contextKey := form.ID + "_" + fieldID
		state.Context[contextKey] = value
	}

	// Mark form as completed
	state.Context[form.ID+"_completed"] = true

	return nil
}

// GetFormValue retrieves a form value from conversation state
func GetFormValue(state *ConversationState, formID, fieldID string, defaultValue interface{}) interface{} {
	contextKey := formID + "_" + fieldID
	if value, exists := state.Context[contextKey]; exists {
		return value
	}
	return defaultValue
}

// Utility functions
func intPtr(i int) *int {
	return &i
}

func float64Ptr(f float64) *float64 {
	return &f
}

// WithForm creates a conversation response that includes a structured form
func (r *ConversationResponse) WithForm(form *StructuredForm) *ConversationResponseWithForm {
	return &ConversationResponseWithForm{
		ConversationResponse: r,
		Form:                 form,
	}
}
