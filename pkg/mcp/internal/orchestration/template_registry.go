package orchestration

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/rs/zerolog"
)

// FileTemplateRegistry implements TemplateRegistry using file system
type FileTemplateRegistry struct {
	basePath string
	logger   zerolog.Logger
	mutex    sync.RWMutex
}

// MemoryTemplateRegistry implements TemplateRegistry using in-memory storage
type MemoryTemplateRegistry struct {
	templates map[string]*WorkflowTemplate
	mutex     sync.RWMutex
	logger    zerolog.Logger
}

// DefaultTemplateValidator implements TemplateValidator
type DefaultTemplateValidator struct {
	logger zerolog.Logger
}

// NewFileTemplateRegistry creates a new file-based template registry
func NewFileTemplateRegistry(basePath string, logger zerolog.Logger) (*FileTemplateRegistry, error) {
	// Ensure base path exists
	if err := os.MkdirAll(basePath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create template directory: %w", err)
	}

	return &FileTemplateRegistry{
		basePath: basePath,
		logger:   logger.With().Str("component", "file_template_registry").Logger(),
	}, nil
}

// SaveTemplate saves a template to the file system
func (ftr *FileTemplateRegistry) SaveTemplate(template *WorkflowTemplate) error {
	ftr.mutex.Lock()
	defer ftr.mutex.Unlock()

	// Create category directory if it doesn't exist
	categoryPath := filepath.Join(ftr.basePath, template.Category)
	if err := os.MkdirAll(categoryPath, 0755); err != nil {
		return fmt.Errorf("failed to create category directory: %w", err)
	}

	// Create template file path
	filename := fmt.Sprintf("%s.json", template.ID)
	templatePath := filepath.Join(categoryPath, filename)

	// Marshal template to JSON
	data, err := json.MarshalIndent(template, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal template: %w", err)
	}

	// Write to file
	if err := os.WriteFile(templatePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write template file: %w", err)
	}

	ftr.logger.Info().
		Str("template_id", template.ID).
		Str("path", templatePath).
		Msg("Template saved to file system")

	return nil
}

// LoadTemplate loads a template from the file system
func (ftr *FileTemplateRegistry) LoadTemplate(id string) (*WorkflowTemplate, error) {
	ftr.mutex.RLock()
	defer ftr.mutex.RUnlock()

	// Search for template file across all categories
	templatePath, err := ftr.findTemplateFile(id)
	if err != nil {
		return nil, fmt.Errorf("template not found: %w", err)
	}

	// Read file
	data, err := os.ReadFile(templatePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read template file: %w", err)
	}

	// Unmarshal template
	var template WorkflowTemplate
	if err := json.Unmarshal(data, &template); err != nil {
		return nil, fmt.Errorf("failed to unmarshal template: %w", err)
	}

	ftr.logger.Debug().
		Str("template_id", id).
		Str("path", templatePath).
		Msg("Template loaded from file system")

	return &template, nil
}

// ListTemplates lists all available templates
func (ftr *FileTemplateRegistry) ListTemplates() ([]*WorkflowTemplate, error) {
	ftr.mutex.RLock()
	defer ftr.mutex.RUnlock()

	var templates []*WorkflowTemplate

	// Walk through all category directories
	err := filepath.Walk(ftr.basePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories and non-JSON files
		if info.IsDir() || !strings.HasSuffix(info.Name(), ".json") {
			return nil
		}

		// Read and parse template file
		data, err := os.ReadFile(path)
		if err != nil {
			ftr.logger.Warn().
				Err(err).
				Str("path", path).
				Msg("Failed to read template file")
			return nil // Continue processing other files
		}

		var template WorkflowTemplate
		if err := json.Unmarshal(data, &template); err != nil {
			ftr.logger.Warn().
				Err(err).
				Str("path", path).
				Msg("Failed to unmarshal template file")
			return nil // Continue processing other files
		}

		templates = append(templates, &template)
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to walk template directory: %w", err)
	}

	ftr.logger.Debug().
		Int("count", len(templates)).
		Msg("Listed templates from file system")

	return templates, nil
}

// DeleteTemplate deletes a template from the file system
func (ftr *FileTemplateRegistry) DeleteTemplate(id string) error {
	ftr.mutex.Lock()
	defer ftr.mutex.Unlock()

	// Find template file
	templatePath, err := ftr.findTemplateFile(id)
	if err != nil {
		return fmt.Errorf("template not found: %w", err)
	}

	// Delete file
	if err := os.Remove(templatePath); err != nil {
		return fmt.Errorf("failed to delete template file: %w", err)
	}

	ftr.logger.Info().
		Str("template_id", id).
		Str("path", templatePath).
		Msg("Template deleted from file system")

	return nil
}

// SearchTemplates searches templates based on query criteria
func (ftr *FileTemplateRegistry) SearchTemplates(query TemplateQuery) ([]*WorkflowTemplate, error) {
	// Get all templates first
	allTemplates, err := ftr.ListTemplates()
	if err != nil {
		return nil, err
	}

	var matchingTemplates []*WorkflowTemplate

	for _, template := range allTemplates {
		if ftr.templateMatchesQuery(template, query) {
			matchingTemplates = append(matchingTemplates, template)
		}
	}

	ftr.logger.Debug().
		Int("total", len(allTemplates)).
		Int("matches", len(matchingTemplates)).
		Msg("Template search completed")

	return matchingTemplates, nil
}

// findTemplateFile finds the file path for a template ID
func (ftr *FileTemplateRegistry) findTemplateFile(id string) (string, error) {
	filename := fmt.Sprintf("%s.json", id)
	var foundPath string

	err := filepath.Walk(ftr.basePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !info.IsDir() && info.Name() == filename {
			foundPath = path
			return filepath.SkipDir // Stop searching
		}

		return nil
	})

	if err != nil {
		return "", err
	}

	if foundPath == "" {
		return "", fmt.Errorf("template file %s not found", filename)
	}

	return foundPath, nil
}

// templateMatchesQuery checks if a template matches the search query
func (ftr *FileTemplateRegistry) templateMatchesQuery(template *WorkflowTemplate, query TemplateQuery) bool {
	// Check category
	if query.Category != "" && template.Category != query.Category {
		return false
	}

	// Check author
	if query.Author != "" && template.Author != query.Author {
		return false
	}

	// Check version
	if query.Version != "" && template.Version != query.Version {
		return false
	}

	// Check name pattern
	if query.NamePattern != "" && !contains(strings.ToLower(template.Name), strings.ToLower(query.NamePattern)) {
		return false
	}

	// Check tags
	if len(query.Tags) > 0 {
		templateTags := make(map[string]bool)
		for _, tag := range template.Tags {
			templateTags[tag] = true
		}

		for _, queryTag := range query.Tags {
			if !templateTags[queryTag] {
				return false
			}
		}
	}

	// Check metadata
	if len(query.Metadata) > 0 {
		for key, value := range query.Metadata {
			if templateValue, exists := template.Metadata[key]; !exists {
				return false
			} else if templateValueStr, ok := templateValue.(string); !ok || templateValueStr != value {
				return false
			}
		}
	}

	return true
}

// NewMemoryTemplateRegistry creates a new in-memory template registry
func NewMemoryTemplateRegistry(logger zerolog.Logger) *MemoryTemplateRegistry {
	return &MemoryTemplateRegistry{
		templates: make(map[string]*WorkflowTemplate),
		logger:    logger.With().Str("component", "memory_template_registry").Logger(),
	}
}

// SaveTemplate saves a template in memory
func (mtr *MemoryTemplateRegistry) SaveTemplate(template *WorkflowTemplate) error {
	mtr.mutex.Lock()
	defer mtr.mutex.Unlock()

	// Create a copy to avoid external modifications
	templateCopy := *template
	mtr.templates[template.ID] = &templateCopy

	mtr.logger.Debug().
		Str("template_id", template.ID).
		Msg("Template saved to memory")

	return nil
}

// LoadTemplate loads a template from memory
func (mtr *MemoryTemplateRegistry) LoadTemplate(id string) (*WorkflowTemplate, error) {
	mtr.mutex.RLock()
	defer mtr.mutex.RUnlock()

	template, exists := mtr.templates[id]
	if !exists {
		return nil, fmt.Errorf("template %s not found", id)
	}

	// Return a copy to avoid external modifications
	templateCopy := *template
	return &templateCopy, nil
}

// ListTemplates lists all templates in memory
func (mtr *MemoryTemplateRegistry) ListTemplates() ([]*WorkflowTemplate, error) {
	mtr.mutex.RLock()
	defer mtr.mutex.RUnlock()

	templates := make([]*WorkflowTemplate, 0, len(mtr.templates))
	for _, template := range mtr.templates {
		// Create copies to avoid external modifications
		templateCopy := *template
		templates = append(templates, &templateCopy)
	}

	return templates, nil
}

// DeleteTemplate deletes a template from memory
func (mtr *MemoryTemplateRegistry) DeleteTemplate(id string) error {
	mtr.mutex.Lock()
	defer mtr.mutex.Unlock()

	if _, exists := mtr.templates[id]; !exists {
		return fmt.Errorf("template %s not found", id)
	}

	delete(mtr.templates, id)

	mtr.logger.Debug().
		Str("template_id", id).
		Msg("Template deleted from memory")

	return nil
}

// SearchTemplates searches templates in memory
func (mtr *MemoryTemplateRegistry) SearchTemplates(query TemplateQuery) ([]*WorkflowTemplate, error) {
	templates, err := mtr.ListTemplates()
	if err != nil {
		return nil, err
	}

	var matchingTemplates []*WorkflowTemplate
	for _, template := range templates {
		if mtr.templateMatchesQuery(template, query) {
			matchingTemplates = append(matchingTemplates, template)
		}
	}

	return matchingTemplates, nil
}

// templateMatchesQuery checks if a template matches the search query (same logic as file registry)
func (mtr *MemoryTemplateRegistry) templateMatchesQuery(template *WorkflowTemplate, query TemplateQuery) bool {
	// Same implementation as FileTemplateRegistry
	if query.Category != "" && template.Category != query.Category {
		return false
	}

	if query.Author != "" && template.Author != query.Author {
		return false
	}

	if query.Version != "" && template.Version != query.Version {
		return false
	}

	if query.NamePattern != "" && !contains(strings.ToLower(template.Name), strings.ToLower(query.NamePattern)) {
		return false
	}

	if len(query.Tags) > 0 {
		templateTags := make(map[string]bool)
		for _, tag := range template.Tags {
			templateTags[tag] = true
		}

		for _, queryTag := range query.Tags {
			if !templateTags[queryTag] {
				return false
			}
		}
	}

	if len(query.Metadata) > 0 {
		for key, value := range query.Metadata {
			if templateValue, exists := template.Metadata[key]; !exists {
				return false
			} else if templateValueStr, ok := templateValue.(string); !ok || templateValueStr != value {
				return false
			}
		}
	}

	return true
}

// NewDefaultTemplateValidator creates a new default template validator
func NewDefaultTemplateValidator(logger zerolog.Logger) *DefaultTemplateValidator {
	return &DefaultTemplateValidator{
		logger: logger.With().Str("component", "template_validator").Logger(),
	}
}

// ValidateTemplate validates a workflow template
func (dtv *DefaultTemplateValidator) ValidateTemplate(template *WorkflowTemplate) error {
	// Check required fields
	if template.ID == "" {
		return fmt.Errorf("template ID is required")
	}

	if template.Name == "" {
		return fmt.Errorf("template name is required")
	}

	if template.Version == "" {
		return fmt.Errorf("template version is required")
	}

	if len(template.Stages) == 0 {
		return fmt.Errorf("template must have at least one stage")
	}

	// Validate parameters
	for _, param := range template.Parameters {
		if err := dtv.validateParameter(param); err != nil {
			return fmt.Errorf("invalid parameter %s: %w", param.Name, err)
		}
	}

	// Validate stages
	stageIDs := make(map[string]bool)
	for _, stage := range template.Stages {
		if err := dtv.validateTemplateStage(stage); err != nil {
			return fmt.Errorf("invalid stage %s: %w", stage.ID, err)
		}

		// Check for duplicate stage IDs
		if stageIDs[stage.ID] {
			return fmt.Errorf("duplicate stage ID: %s", stage.ID)
		}
		stageIDs[stage.ID] = true
	}

	// Validate stage dependencies
	for _, stage := range template.Stages {
		for _, dep := range stage.DependsOn {
			if !stageIDs[dep] {
				return fmt.Errorf("stage %s depends on non-existent stage %s", stage.ID, dep)
			}
		}
	}

	// Check for circular dependencies
	if dtv.hasCircularDependencies(template.Stages) {
		return fmt.Errorf("template has circular dependencies")
	}

	dtv.logger.Debug().
		Str("template_id", template.ID).
		Msg("Template validation completed successfully")

	return nil
}

// ValidateParameters validates template parameters against provided values
func (dtv *DefaultTemplateValidator) ValidateParameters(template *WorkflowTemplate, parameters map[string]interface{}) error {
	// Check required parameters
	for _, param := range template.Parameters {
		value, provided := parameters[param.Name]

		if param.Required && !provided {
			return fmt.Errorf("required parameter %s not provided", param.Name)
		}

		if provided {
			if err := dtv.validateParameterValue(param, value); err != nil {
				return fmt.Errorf("invalid value for parameter %s: %w", param.Name, err)
			}
		}
	}

	// Check for unexpected parameters
	expectedParams := make(map[string]bool)
	for _, param := range template.Parameters {
		expectedParams[param.Name] = true
	}

	for paramName := range parameters {
		if !expectedParams[paramName] {
			dtv.logger.Warn().
				Str("parameter", paramName).
				Str("template_id", template.ID).
				Msg("Unexpected parameter provided")
		}
	}

	return nil
}

// ValidateConditions validates template conditions against context
func (dtv *DefaultTemplateValidator) ValidateConditions(template *WorkflowTemplate, context map[string]interface{}) error {
	for _, condition := range template.Conditions {
		if !dtv.evaluateTemplateCondition(condition, context) {
			return fmt.Errorf("template condition not met: %s", condition.Description)
		}
	}

	return nil
}

// validateParameter validates a template parameter definition
func (dtv *DefaultTemplateValidator) validateParameter(param TemplateParameter) error {
	if param.Name == "" {
		return fmt.Errorf("parameter name is required")
	}

	if param.Type == "" {
		return fmt.Errorf("parameter type is required")
	}

	validTypes := map[string]bool{
		"string": true, "int": true, "float": true, "bool": true,
		"array": true, "object": true,
	}

	if !validTypes[param.Type] {
		return fmt.Errorf("invalid parameter type: %s", param.Type)
	}

	return nil
}

// validateTemplateStage validates a template stage
func (dtv *DefaultTemplateValidator) validateTemplateStage(stage TemplateStage) error {
	if stage.ID == "" {
		return fmt.Errorf("stage ID is required")
	}

	if stage.Name == "" {
		return fmt.Errorf("stage name is required")
	}

	if stage.ToolName == "" {
		return fmt.Errorf("stage tool name is required")
	}

	// Validate retry policy if present
	if stage.RetryPolicy != nil {
		if stage.RetryPolicy.MaxAttempts < 1 {
			return fmt.Errorf("max attempts must be at least 1")
		}
		if stage.RetryPolicy.InitialDelay < 0 {
			return fmt.Errorf("initial delay cannot be negative")
		}
	}

	return nil
}

// validateParameterValue validates a parameter value against its definition
func (dtv *DefaultTemplateValidator) validateParameterValue(param TemplateParameter, value interface{}) error {
	// Type validation
	if err := dtv.validateParameterType(param.Type, value); err != nil {
		return err
	}

	// Validation rules
	validation := param.Validation

	// Check allowed values
	if len(validation.AllowedValues) > 0 {
		found := false
		for _, allowed := range validation.AllowedValues {
			if value == allowed {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("value not in allowed values list")
		}
	}

	// Check pattern for strings
	if validation.Pattern != "" && param.Type == "string" {
		if strValue, ok := value.(string); ok {
			// Simple pattern matching (contains check)
			if !contains(strValue, validation.Pattern) {
				return fmt.Errorf("value does not match required pattern")
			}
		}
	}

	// Check numeric ranges
	if validation.MinValue != nil || validation.MaxValue != nil {
		if param.Type == "int" || param.Type == "float" {
			numValue, ok := dtv.toFloat64(value)
			if !ok {
				return fmt.Errorf("expected numeric value")
			}

			if validation.MinValue != nil {
				minValue, ok := dtv.toFloat64(validation.MinValue)
				if ok && numValue < minValue {
					return fmt.Errorf("value below minimum: %v", validation.MinValue)
				}
			}

			if validation.MaxValue != nil {
				maxValue, ok := dtv.toFloat64(validation.MaxValue)
				if ok && numValue > maxValue {
					return fmt.Errorf("value above maximum: %v", validation.MaxValue)
				}
			}
		}
	}

	return nil
}

// validateParameterType validates the type of a parameter value
func (dtv *DefaultTemplateValidator) validateParameterType(paramType string, value interface{}) error {
	switch paramType {
	case "string":
		if _, ok := value.(string); !ok {
			return fmt.Errorf("expected string, got %T", value)
		}
	case "int":
		if _, ok := value.(int); !ok {
			if _, ok := value.(int64); !ok {
				return fmt.Errorf("expected int, got %T", value)
			}
		}
	case "float":
		if _, ok := value.(float64); !ok {
			if _, ok := value.(float32); !ok {
				return fmt.Errorf("expected float, got %T", value)
			}
		}
	case "bool":
		if _, ok := value.(bool); !ok {
			return fmt.Errorf("expected bool, got %T", value)
		}
	case "array":
		// Check if it's a slice or array
		switch value.(type) {
		case []interface{}, []string, []int, []float64:
			// Valid array types
		default:
			return fmt.Errorf("expected array, got %T", value)
		}
	case "object":
		if _, ok := value.(map[string]interface{}); !ok {
			return fmt.Errorf("expected object, got %T", value)
		}
	}

	return nil
}

// toFloat64 converts numeric values to float64 for comparison
func (dtv *DefaultTemplateValidator) toFloat64(value interface{}) (float64, bool) {
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

// evaluateTemplateCondition evaluates a template condition
func (dtv *DefaultTemplateValidator) evaluateTemplateCondition(condition TemplateCondition, context map[string]interface{}) bool {
	// Simple implementation - extend as needed
	switch condition.Type {
	case "platform":
		// Platform-specific conditions
		return true // Placeholder
	case "environment":
		// Environment-specific conditions
		return true // Placeholder
	case "capability":
		// Capability-specific conditions
		return true // Placeholder
	default:
		return true
	}
}

// hasCircularDependencies checks for circular dependencies in template stages
func (dtv *DefaultTemplateValidator) hasCircularDependencies(stages []TemplateStage) bool {
	// Build dependency graph
	graph := make(map[string][]string)
	for _, stage := range stages {
		graph[stage.ID] = stage.DependsOn
	}

	// Check for cycles using DFS
	visited := make(map[string]bool)
	recursionStack := make(map[string]bool)

	var hasCycle func(string) bool
	hasCycle = func(stageID string) bool {
		visited[stageID] = true
		recursionStack[stageID] = true

		for _, dep := range graph[stageID] {
			if !visited[dep] && hasCycle(dep) {
				return true
			} else if recursionStack[dep] {
				return true
			}
		}

		recursionStack[stageID] = false
		return false
	}

	for _, stage := range stages {
		if !visited[stage.ID] && hasCycle(stage.ID) {
			return true
		}
	}

	return false
}
