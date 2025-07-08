package workflow

import (
	"sync"
	"time"
)

// WorkflowTemplate represents a reusable workflow template
type WorkflowTemplate struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Category    string                 `json:"category"`
	Version     string                 `json:"version"`
	Author      string                 `json:"author"`
	CreatedAt   time.Time              `json:"created_at"`
	UpdatedAt   time.Time              `json:"updated_at"`
	Tags        []string               `json:"tags"`
	Parameters  []TemplateParameter    `json:"parameters"`
	Stages      []TemplateStage        `json:"stages"`
	Variables   map[string]interface{} `json:"variables"`
	Conditions  []TemplateCondition    `json:"conditions"`
	Metadata    map[string]interface{} `json:"metadata"`
	Validation  TemplateValidation     `json:"validation"`
}

// TemplateParameter defines a template parameter
type TemplateParameter struct {
	Name         string              `json:"name"`
	Type         string              `json:"type"`
	Description  string              `json:"description"`
	Required     bool                `json:"required"`
	DefaultValue interface{}         `json:"default_value,omitempty"`
	Validation   ParameterValidation `json:"validation"`
}

// ParameterValidation defines validation rules for parameters
type ParameterValidation struct {
	MinValue      interface{}              `json:"min_value,omitempty"`
	MaxValue      interface{}              `json:"max_value,omitempty"`
	Pattern       string                   `json:"pattern,omitempty"`
	AllowedValues []interface{}            `json:"allowed_values,omitempty"`
	CustomRules   []WorkflowValidationRule `json:"custom_rules,omitempty"`
}

// WorkflowValidationRule defines a custom validation rule for workflow templates
type WorkflowValidationRule struct {
	Type         string `json:"type"`
	Expression   string `json:"expression"`
	ErrorMessage string `json:"error_message"`
}

// TemplateStage represents a stage in a workflow template
type TemplateStage struct {
	ID          string                   `json:"id"`
	Name        string                   `json:"name"`
	Type        string                   `json:"type"`
	ToolName    string                   `json:"tool_name"`
	Parameters  map[string]interface{}   `json:"parameters"`
	DependsOn   []string                 `json:"depends_on"`
	Conditions  []TemplateStageCondition `json:"conditions"`
	RetryPolicy *RetryPolicyTemplate     `json:"retry_policy,omitempty"`
	Timeout     *time.Duration           `json:"timeout,omitempty"`
	OnSuccess   []ActionTemplate         `json:"on_success,omitempty"`
	OnFailure   []ActionTemplate         `json:"on_failure,omitempty"`
	Parallel    bool                     `json:"parallel"`
	Optional    bool                     `json:"optional"`
}

// TemplateStageCondition defines when a stage should execute
type TemplateStageCondition struct {
	Type      string      `json:"type"`
	Field     string      `json:"field"`
	Operator  string      `json:"operator"`
	Value     interface{} `json:"value"`
	LogicalOp string      `json:"logical_op,omitempty"`
}

// RetryPolicyTemplate defines retry behavior for a template stage
type RetryPolicyTemplate struct {
	MaxAttempts  int           `json:"max_attempts"`
	InitialDelay time.Duration `json:"initial_delay"`
	MaxDelay     time.Duration `json:"max_delay"`
	BackoffType  string        `json:"backoff_type"`
	RetryOn      []string      `json:"retry_on"`
	AbortOn      []string      `json:"abort_on"`
}

// ActionTemplate defines actions to take on stage completion
type ActionTemplate struct {
	Type       string                 `json:"type"`
	Parameters map[string]interface{} `json:"parameters"`
}

// TemplateCondition defines when a template can be used
type TemplateCondition struct {
	Type        string      `json:"type"`
	Field       string      `json:"field"`
	Operator    string      `json:"operator"`
	Value       interface{} `json:"value"`
	Description string      `json:"description"`
}

// TemplateValidation defines validation rules for the template
type TemplateValidation struct {
	RequiredCapabilities []string           `json:"required_capabilities"`
	MinVersion           string             `json:"min_version"`
	MaxVersion           string             `json:"max_version"`
	Platforms            []string           `json:"platforms"`
	CustomValidations    []CustomValidation `json:"custom_validations"`
}

// CustomValidation defines custom validation logic
type CustomValidation struct {
	Name         string                 `json:"name"`
	Type         string                 `json:"type"`
	Expression   string                 `json:"expression"`
	ErrorMessage string                 `json:"error_message"`
	Parameters   map[string]interface{} `json:"parameters"`
}

// TemplateQuery defines search criteria for templates
type TemplateQuery struct {
	Category    string            `json:"category"`
	Tags        []string          `json:"tags"`
	Author      string            `json:"author"`
	NamePattern string            `json:"name_pattern"`
	Version     string            `json:"version"`
	Metadata    map[string]string `json:"metadata"`
}

// TemplateVersionManager manages template versions
type TemplateVersionManager struct {
	versions map[string][]TemplateVersion
	mutex    sync.RWMutex
}

// TemplateVersion represents a version of a template
type TemplateVersion struct {
	Version     string            `json:"version"`
	Template    *WorkflowTemplate `json:"template"`
	ChangedAt   time.Time         `json:"changed_at"`
	ChangedBy   string            `json:"changed_by"`
	ChangeNotes string            `json:"change_notes"`
	Deprecated  bool              `json:"deprecated"`
}

// WorkflowInstantiation represents an instantiated workflow from a template
type WorkflowInstantiation struct {
	ID           string                 `json:"id"`
	TemplateID   string                 `json:"template_id"`
	TemplateName string                 `json:"template_name"`
	Version      string                 `json:"version"`
	Parameters   map[string]interface{} `json:"parameters"`
	CreatedAt    time.Time              `json:"created_at"`
	WorkflowSpec *WorkflowSpec          `json:"workflow_spec"`
}
