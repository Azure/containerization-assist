package orchestration

import (
	"context"
	"fmt"
	"sync"
	"time"

	commonUtils "github.com/Azure/container-kit/pkg/commonutils"
	"github.com/rs/zerolog"
)

// WorkflowTemplateManager manages reusable workflow templates
type WorkflowTemplateManager struct {
	logger             zerolog.Logger
	templates          map[string]*WorkflowTemplate
	templateCategories map[string][]string
	templateRegistry   TemplateRegistry
	validator          TemplateValidator
	mutex              sync.RWMutex
	versionManager     *TemplateVersionManager
}

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
	MinValue      interface{}      `json:"min_value,omitempty"`
	MaxValue      interface{}      `json:"max_value,omitempty"`
	Pattern       string           `json:"pattern,omitempty"`
	AllowedValues []interface{}    `json:"allowed_values,omitempty"`
	CustomRules   []ValidationRule `json:"custom_rules,omitempty"`
}

// ValidationRule defines a custom validation rule
type ValidationRule struct {
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
	Type      string      `json:"type"` // "parameter", "variable", "context", "previous_stage"
	Field     string      `json:"field"`
	Operator  string      `json:"operator"`
	Value     interface{} `json:"value"`
	LogicalOp string      `json:"logical_op,omitempty"` // "AND", "OR"
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
	Type       string                 `json:"type"` // "set_variable", "call_tool", "send_notification"
	Parameters map[string]interface{} `json:"parameters"`
}

// TemplateCondition defines when a template can be used
type TemplateCondition struct {
	Type        string      `json:"type"` // "platform", "environment", "capability"
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

// TemplateRegistry manages template storage and retrieval
type TemplateRegistry interface {
	SaveTemplate(template *WorkflowTemplate) error
	LoadTemplate(id string) (*WorkflowTemplate, error)
	ListTemplates() ([]*WorkflowTemplate, error)
	DeleteTemplate(id string) error
	SearchTemplates(query TemplateQuery) ([]*WorkflowTemplate, error)
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

// TemplateValidator validates templates and their usage
type TemplateValidator interface {
	ValidateTemplate(template *WorkflowTemplate) error
	ValidateParameters(template *WorkflowTemplate, parameters map[string]interface{}) error
	ValidateConditions(template *WorkflowTemplate, context map[string]interface{}) error
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

// NewWorkflowTemplateManager creates a new template manager
func NewWorkflowTemplateManager(logger zerolog.Logger, registry TemplateRegistry, validator TemplateValidator) *WorkflowTemplateManager {
	return &WorkflowTemplateManager{
		logger:             logger.With().Str("component", "workflow_template_manager").Logger(),
		templates:          make(map[string]*WorkflowTemplate),
		templateCategories: make(map[string][]string),
		templateRegistry:   registry,
		validator:          validator,
		versionManager:     NewTemplateVersionManager(),
	}
}

// RegisterTemplate registers a new workflow template
func (wtm *WorkflowTemplateManager) RegisterTemplate(template *WorkflowTemplate) error {
	wtm.mutex.Lock()
	defer wtm.mutex.Unlock()

	// Validate template
	if err := wtm.validator.ValidateTemplate(template); err != nil {
		return fmt.Errorf("template validation failed: %w", err)
	}

	// Set timestamps
	now := time.Now()
	if template.CreatedAt.IsZero() {
		template.CreatedAt = now
	}
	template.UpdatedAt = now

	// Store in memory
	wtm.templates[template.ID] = template

	// Update categories
	wtm.addToCategory(template.Category, template.ID)

	// Save to registry
	if err := wtm.templateRegistry.SaveTemplate(template); err != nil {
		return fmt.Errorf("failed to save template to registry: %w", err)
	}

	// Add to version manager
	wtm.versionManager.AddVersion(template.ID, &TemplateVersion{
		Version:     template.Version,
		Template:    template,
		ChangedAt:   now,
		ChangedBy:   template.Author,
		ChangeNotes: "Template registered",
	})

	wtm.logger.Info().
		Str("template_id", template.ID).
		Str("name", template.Name).
		Str("version", template.Version).
		Str("category", template.Category).
		Msg("Template registered successfully")

	return nil
}

// GetTemplate retrieves a template by ID
func (wtm *WorkflowTemplateManager) GetTemplate(id string) (*WorkflowTemplate, error) {
	wtm.mutex.RLock()
	defer wtm.mutex.RUnlock()

	if template, exists := wtm.templates[id]; exists {
		return template, nil
	}

	// Try loading from registry
	template, err := wtm.templateRegistry.LoadTemplate(id)
	if err != nil {
		return nil, fmt.Errorf("template not found: %w", err)
	}

	// Cache in memory
	wtm.templates[id] = template
	wtm.addToCategory(template.Category, template.ID)

	return template, nil
}

// ListTemplates lists all available templates
func (wtm *WorkflowTemplateManager) ListTemplates() ([]*WorkflowTemplate, error) {
	wtm.mutex.RLock()
	defer wtm.mutex.RUnlock()

	templates := make([]*WorkflowTemplate, 0, len(wtm.templates))
	for _, template := range wtm.templates {
		templates = append(templates, template)
	}

	return templates, nil
}

// ListTemplatesByCategory lists templates by category
func (wtm *WorkflowTemplateManager) ListTemplatesByCategory(category string) ([]*WorkflowTemplate, error) {
	wtm.mutex.RLock()
	defer wtm.mutex.RUnlock()

	templateIDs, exists := wtm.templateCategories[category]
	if !exists {
		return []*WorkflowTemplate{}, nil
	}

	templates := make([]*WorkflowTemplate, 0, len(templateIDs))
	for _, id := range templateIDs {
		if template, exists := wtm.templates[id]; exists {
			templates = append(templates, template)
		}
	}

	return templates, nil
}

// SearchTemplates searches templates based on query criteria
func (wtm *WorkflowTemplateManager) SearchTemplates(query TemplateQuery) ([]*WorkflowTemplate, error) {
	return wtm.templateRegistry.SearchTemplates(query)
}

// InstantiateWorkflow creates a workflow from a template
func (wtm *WorkflowTemplateManager) InstantiateWorkflow(ctx context.Context, templateID string, parameters map[string]interface{}) (*WorkflowInstantiation, error) {
	// Get template
	template, err := wtm.GetTemplate(templateID)
	if err != nil {
		return nil, fmt.Errorf("failed to get template: %w", err)
	}

	// Validate parameters
	if err := wtm.validator.ValidateParameters(template, parameters); err != nil {
		return nil, fmt.Errorf("parameter validation failed: %w", err)
	}

	// Check template conditions
	contextData := map[string]interface{}{
		"parameters": parameters,
	}
	if err := wtm.validator.ValidateConditions(template, contextData); err != nil {
		return nil, fmt.Errorf("template conditions not met: %w", err)
	}

	// Create workflow specification
	workflowSpec, err := wtm.createWorkflowSpec(template, parameters)
	if err != nil {
		return nil, fmt.Errorf("failed to create workflow spec: %w", err)
	}

	// Create instantiation
	instantiation := &WorkflowInstantiation{
		ID:           wtm.generateInstantiationID(),
		TemplateID:   template.ID,
		TemplateName: template.Name,
		Version:      template.Version,
		Parameters:   parameters,
		CreatedAt:    time.Now(),
		WorkflowSpec: workflowSpec,
	}

	wtm.logger.Info().
		Str("instantiation_id", instantiation.ID).
		Str("template_id", templateID).
		Str("template_name", template.Name).
		Msg("Workflow instantiated from template")

	return instantiation, nil
}

// createWorkflowSpec creates a WorkflowSpec from a template
func (wtm *WorkflowTemplateManager) createWorkflowSpec(template *WorkflowTemplate, parameters map[string]interface{}) (*WorkflowSpec, error) {
	// Merge template variables with parameters
	variables := make(map[string]interface{})
	for k, v := range template.Variables {
		variables[k] = v
	}
	for k, v := range parameters {
		variables[k] = v
	}

	// Convert template stages to execution stages
	stages := make([]ExecutionStage, 0, len(template.Stages))
	for _, templateStage := range template.Stages {
		// Check stage conditions
		if !wtm.evaluateStageConditions(templateStage.Conditions, parameters, variables) {
			continue
		}

		// Resolve parameters
		resolvedParams, err := wtm.resolveParameters(templateStage.Parameters, parameters, variables)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve parameters for stage %s: %w", templateStage.ID, err)
		}

		stage := ExecutionStage{
			ID:        templateStage.ID,
			Name:      templateStage.Name,
			Type:      templateStage.Type,
			Tools:     []string{templateStage.ToolName},
			Variables: resolvedParams,
			DependsOn: templateStage.DependsOn,
			Parallel:  templateStage.Parallel,
		}

		// Convert retry policy if present
		if templateStage.RetryPolicy != nil {
			stage.RetryPolicy = &RetryPolicyExecution{
				MaxAttempts:  templateStage.RetryPolicy.MaxAttempts,
				Delay:        templateStage.RetryPolicy.InitialDelay,
				BackoffType:  templateStage.RetryPolicy.BackoffType,
				InitialDelay: templateStage.RetryPolicy.InitialDelay,
				MaxDelay:     templateStage.RetryPolicy.MaxDelay,
			}
		}

		// Set timeout if present
		if templateStage.Timeout != nil {
			stage.Timeout = templateStage.Timeout
		}

		stages = append(stages, stage)
	}

	workflowSpec := &WorkflowSpec{
		ID:        wtm.generateWorkflowSpecID(),
		Name:      fmt.Sprintf("%s (from %s)", template.Name, template.ID),
		Version:   template.Version,
		Stages:    stages,
		Variables: variables,
		Metadata: WorkflowMetadata{
			Name:        template.Name,
			Description: template.Description,
			Version:     template.Version,
			Labels: map[string]string{
				"template_id":   template.ID,
				"template_name": template.Name,
			},
		},
	}

	return workflowSpec, nil
}

// evaluateStageConditions evaluates stage conditions
func (wtm *WorkflowTemplateManager) evaluateStageConditions(conditions []TemplateStageCondition, parameters, variables map[string]interface{}) bool {
	if len(conditions) == 0 {
		return true
	}

	result := true
	currentLogicalOp := "AND"

	for _, condition := range conditions {
		conditionResult := wtm.evaluateStageCondition(condition, parameters, variables)

		switch currentLogicalOp {
		case "AND":
			result = result && conditionResult
		case "OR":
			result = result || conditionResult
		}

		if condition.LogicalOp != "" {
			currentLogicalOp = condition.LogicalOp
		}
	}

	return result
}

// evaluateStageCondition evaluates a single stage condition
func (wtm *WorkflowTemplateManager) evaluateStageCondition(condition TemplateStageCondition, parameters, variables map[string]interface{}) bool {
	var actualValue interface{}

	switch condition.Type {
	case "parameter":
		actualValue = parameters[condition.Field]
	case "variable":
		actualValue = variables[condition.Field]
	case "context":
		// Context evaluation would need to be implemented based on the specific context structure
		return true // Placeholder
	default:
		return false
	}

	return wtm.compareValues(actualValue, condition.Operator, condition.Value)
}

// resolveParameters resolves template parameters with actual values
func (wtm *WorkflowTemplateManager) resolveParameters(templateParams map[string]interface{}, parameters, variables map[string]interface{}) (map[string]interface{}, error) {
	resolved := make(map[string]interface{})

	for key, value := range templateParams {
		resolvedValue, err := wtm.resolveValue(value, parameters, variables)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve parameter %s: %w", key, err)
		}
		resolved[key] = resolvedValue
	}

	return resolved, nil
}

// resolveValue resolves a single value (handles templates like ${param})
func (wtm *WorkflowTemplateManager) resolveValue(value interface{}, parameters, variables map[string]interface{}) (interface{}, error) {
	if str, ok := value.(string); ok {
		// Simple template resolution (${param_name})
		if len(str) > 3 && str[:2] == "${" && str[len(str)-1:] == "}" {
			paramName := str[2 : len(str)-1]

			// Look in parameters first, then variables
			if paramValue, exists := parameters[paramName]; exists {
				return paramValue, nil
			}
			if varValue, exists := variables[paramName]; exists {
				return varValue, nil
			}

			return nil, fmt.Errorf("parameter or variable %s not found", paramName)
		}
	}

	return value, nil
}

// compareValues compares values using the specified operator
func (wtm *WorkflowTemplateManager) compareValues(actual interface{}, operator string, expected interface{}) bool {
	switch operator {
	case "equals", "==":
		return actual == expected
	case "not_equals", "!=":
		return actual != expected
	case "contains":
		if actualStr, ok := actual.(string); ok {
			if expectedStr, ok := expected.(string); ok {
				return commonUtils.Contains(actualStr, expectedStr)
			}
		}
	case "greater_than", ">":
		return wtm.compareNumeric(actual, expected, ">")
	case "less_than", "<":
		return wtm.compareNumeric(actual, expected, "<")
	case "greater_equal", ">=":
		return wtm.compareNumeric(actual, expected, ">=")
	case "less_equal", "<=":
		return wtm.compareNumeric(actual, expected, "<=")
	}
	return false
}

// compareNumeric compares numeric values
func (wtm *WorkflowTemplateManager) compareNumeric(actual, expected interface{}, op string) bool {
	actualNum, actualOk := wtm.toFloat64(actual)
	expectedNum, expectedOk := wtm.toFloat64(expected)

	if !actualOk || !expectedOk {
		return false
	}

	switch op {
	case ">":
		return actualNum > expectedNum
	case "<":
		return actualNum < expectedNum
	case ">=":
		return actualNum >= expectedNum
	case "<=":
		return actualNum <= expectedNum
	}
	return false
}

// toFloat64 converts various numeric types to float64
func (wtm *WorkflowTemplateManager) toFloat64(value interface{}) (float64, bool) {
	switch v := value.(type) {
	case int:
		return float64(v), true
	case int64:
		return float64(v), true
	case float64:
		return v, true
	case float32:
		return float64(v), true
	default:
		return 0, false
	}
}

// addToCategory adds a template to a category
func (wtm *WorkflowTemplateManager) addToCategory(category, templateID string) {
	if category == "" {
		category = "uncategorized"
	}

	if wtm.templateCategories[category] == nil {
		wtm.templateCategories[category] = []string{}
	}

	// Check if already exists
	for _, id := range wtm.templateCategories[category] {
		if id == templateID {
			return
		}
	}

	wtm.templateCategories[category] = append(wtm.templateCategories[category], templateID)
}

// generateInstantiationID generates a unique instantiation ID
func (wtm *WorkflowTemplateManager) generateInstantiationID() string {
	return fmt.Sprintf("inst_%d", time.Now().UnixNano())
}

// generateWorkflowSpecID generates a unique workflow spec ID
func (wtm *WorkflowTemplateManager) generateWorkflowSpecID() string {
	return fmt.Sprintf("spec_%d", time.Now().UnixNano())
}

// GetCategories returns all template categories
func (wtm *WorkflowTemplateManager) GetCategories() []string {
	wtm.mutex.RLock()
	defer wtm.mutex.RUnlock()

	categories := make([]string, 0, len(wtm.templateCategories))
	for category := range wtm.templateCategories {
		categories = append(categories, category)
	}
	return categories
}

// NewTemplateVersionManager creates a new version manager
func NewTemplateVersionManager() *TemplateVersionManager {
	return &TemplateVersionManager{
		versions: make(map[string][]TemplateVersion),
	}
}

// AddVersion adds a new version of a template
func (tvm *TemplateVersionManager) AddVersion(templateID string, version *TemplateVersion) {
	tvm.mutex.Lock()
	defer tvm.mutex.Unlock()

	if tvm.versions[templateID] == nil {
		tvm.versions[templateID] = []TemplateVersion{}
	}

	tvm.versions[templateID] = append(tvm.versions[templateID], *version)
}

// GetVersions returns all versions of a template
func (tvm *TemplateVersionManager) GetVersions(templateID string) ([]TemplateVersion, error) {
	tvm.mutex.RLock()
	defer tvm.mutex.RUnlock()

	versions, exists := tvm.versions[templateID]
	if !exists {
		return nil, fmt.Errorf("no versions found for template %s", templateID)
	}

	return versions, nil
}

// GetLatestVersion returns the latest version of a template
func (tvm *TemplateVersionManager) GetLatestVersion(templateID string) (*TemplateVersion, error) {
	versions, err := tvm.GetVersions(templateID)
	if err != nil {
		return nil, err
	}

	if len(versions) == 0 {
		return nil, fmt.Errorf("no versions available for template %s", templateID)
	}

	// Return the most recent version
	latest := &versions[len(versions)-1]
	return latest, nil
}
