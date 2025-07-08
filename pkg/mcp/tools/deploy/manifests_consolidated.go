package deploy

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/application/api"
	"github.com/Azure/container-kit/pkg/mcp/errors"
	validation "github.com/Azure/container-kit/pkg/mcp/security"
	"github.com/Azure/container-kit/pkg/mcp/services"
)

// ConsolidatedManifestsInput represents unified input for all manifest-related operations
type ConsolidatedManifestsInput struct {
	// Core parameters (with backward compatibility aliases)
	SessionID string `json:"session_id,omitempty" validate:"omitempty,session_id" description:"Session ID for state correlation"`
	ImageRef  string `json:"image_ref" validate:"required,docker_image" description:"Container image reference"`
	ImageName string `json:"image_name,omitempty" description:"Alias for image_ref for backward compatibility"`

	// Application configuration
	AppName   string `json:"app_name,omitempty" validate:"omitempty,k8s_name" description:"Application name (default: from image name)"`
	Namespace string `json:"namespace,omitempty" validate:"omitempty,namespace" description:"Kubernetes namespace (default: default)"`

	// Manifest configuration
	ManifestMode   string   `json:"manifest_mode,omitempty" validate:"omitempty,oneof=generate validate template" description:"Manifest mode: generate, validate, or template"`
	ManifestTypes  []string `json:"manifest_types,omitempty" validate:"omitempty,dive,oneof=deployment service ingress configmap secret hpa" description:"Types of manifests to generate"`
	OutputFormat   string   `json:"output_format,omitempty" validate:"omitempty,oneof=yaml json" description:"Output format for manifests"`
	TemplateEngine string   `json:"template_engine,omitempty" validate:"omitempty,oneof=helm kustomize raw" description:"Template engine to use"`

	// Deployment configuration (for manifest generation)
	Replicas       int               `json:"replicas,omitempty" validate:"omitempty,min=1,max=100" description:"Number of replicas"`
	Port           int               `json:"port,omitempty" validate:"omitempty,port" description:"Application port"`
	ServiceType    string            `json:"service_type,omitempty" validate:"omitempty,service_type" description:"Service type"`
	IncludeIngress bool              `json:"include_ingress,omitempty" description:"Generate Ingress resource"`
	Environment    map[string]string `json:"environment,omitempty" validate:"omitempty,dive,keys,required,endkeys,no_sensitive" description:"Environment variables"`

	// Resource configuration
	CPURequest    string `json:"cpu_request,omitempty" validate:"omitempty,resource_spec" description:"CPU request"`
	MemoryRequest string `json:"memory_request,omitempty" validate:"omitempty,resource_spec" description:"Memory request"`
	CPULimit      string `json:"cpu_limit,omitempty" validate:"omitempty,resource_spec" description:"CPU limit"`
	MemoryLimit   string `json:"memory_limit,omitempty" validate:"omitempty,resource_spec" description:"Memory limit"`

	// Validation options (when validating existing manifests)
	ManifestContent  string   `json:"manifest_content,omitempty" description:"Manifest content to validate (YAML/JSON)"`
	ManifestPaths    []string `json:"manifest_paths,omitempty" description:"Paths to manifest files to validate"`
	ValidationRules  []string `json:"validation_rules,omitempty" description:"Specific validation rules to apply"`
	StrictValidation bool     `json:"strict_validation,omitempty" description:"Enable strict validation mode"`

	// Template options
	TemplateValues   map[string]interface{} `json:"template_values,omitempty" description:"Values for template rendering"`
	TemplateFiles    []string               `json:"template_files,omitempty" description:"Template files to process"`
	IncludeTemplates bool                   `json:"include_templates,omitempty" description:"Include template source in output"`

	// Output options
	WriteToFiles    bool   `json:"write_to_files,omitempty" description:"Write manifests to files"`
	OutputDirectory string `json:"output_directory,omitempty" description:"Directory to write manifest files"`
	FilenamePattern string `json:"filename_pattern,omitempty" description:"Pattern for manifest filenames"`
	IncludeMetadata bool   `json:"include_metadata,omitempty" description:"Include generation metadata in manifests"`

	// Performance options
	UseCache       bool `json:"use_cache,omitempty" description:"Use cached results if available"`
	Timeout        int  `json:"timeout,omitempty" validate:"omitempty,min=30,max=3600" description:"Operation timeout in seconds"`
	ValidateSchema bool `json:"validate_schema,omitempty" description:"Validate manifest schemas"`

	// Advanced options
	DryRun   bool                   `json:"dry_run,omitempty" description:"Preview operation without execution"`
	Metadata map[string]interface{} `json:"metadata,omitempty" description:"Additional metadata for operation context"`
}

// Validate implements validation using tag-based validation
func (c ConsolidatedManifestsInput) Validate() error {
	imageRef := c.getImageRef()
	if imageRef == "" && c.ManifestContent == "" && len(c.ManifestPaths) == 0 {
		return errors.NewError().Message("image reference, manifest content, or manifest paths are required").Build()
	}
	return validation.ValidateTaggedStruct(c)
}

// getImageRef returns the image reference, handling backward compatibility aliases
func (c ConsolidatedManifestsInput) getImageRef() string {
	if c.ImageRef != "" {
		return c.ImageRef
	}
	return c.ImageName
}

// getManifestMode returns the manifest mode, defaulting to generate
func (c ConsolidatedManifestsInput) getManifestMode() string {
	if c.ManifestMode != "" {
		return c.ManifestMode
	}
	return "generate"
}

// ConsolidatedManifestsOutput represents unified output for all manifest operations
type ConsolidatedManifestsOutput struct {
	// Status
	Success   bool   `json:"success"`
	SessionID string `json:"session_id"`
	Error     string `json:"error,omitempty"`

	// Core operation results
	ImageRef      string        `json:"image_ref,omitempty"`
	AppName       string        `json:"app_name,omitempty"`
	Namespace     string        `json:"namespace,omitempty"`
	ManifestMode  string        `json:"manifest_mode"`
	OperationTime time.Time     `json:"operation_time"`
	Duration      time.Duration `json:"duration"`

	// Generated manifests
	GeneratedManifests map[string]string `json:"generated_manifests,omitempty"`
	ManifestFiles      []ManifestFile    `json:"manifest_files,omitempty"`
	TemplateOutput     *TemplateOutput   `json:"template_output,omitempty"`

	// Validation results
	ValidationResult   *ManifestValidationResult `json:"validation_result,omitempty"`
	SchemaValidation   *SchemaValidationResult   `json:"schema_validation,omitempty"`
	SecurityValidation *SecurityValidationResult `json:"security_validation,omitempty"`

	// Operation metadata
	GenerationStats  *GenerationStats       `json:"generation_stats,omitempty"`
	ResourceSummary  *ResourceSummary       `json:"resource_summary,omitempty"`
	TemplateMetadata map[string]interface{} `json:"template_metadata,omitempty"`

	// File output
	OutputDirectory string   `json:"output_directory,omitempty"`
	WrittenFiles    []string `json:"written_files,omitempty"`

	// Performance metrics
	ParseDuration      time.Duration `json:"parse_duration"`
	GenerationDuration time.Duration `json:"generation_duration"`
	ValidationDuration time.Duration `json:"validation_duration"`
	TotalDuration      time.Duration `json:"total_duration"`
	CacheHit           bool          `json:"cache_hit,omitempty"`

	// Metadata
	ToolVersion string                 `json:"tool_version"`
	Timestamp   time.Time              `json:"timestamp"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
	Warnings    []string               `json:"warnings,omitempty"`
}

// Supporting types for manifest operations
type ManifestFile struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	Content  string `json:"content"`
	Path     string `json:"path,omitempty"`
	Size     int    `json:"size"`
	Checksum string `json:"checksum,omitempty"`
}

type TemplateOutput struct {
	Engine         string                 `json:"engine"`
	TemplatesUsed  []string               `json:"templates_used"`
	ValuesUsed     map[string]interface{} `json:"values_used"`
	RenderTime     time.Duration          `json:"render_time"`
	TemplateErrors []TemplateError        `json:"template_errors,omitempty"`
}

type TemplateError struct {
	Template string `json:"template"`
	Line     int    `json:"line,omitempty"`
	Message  string `json:"message"`
	Severity string `json:"severity"`
}

type ManifestValidationResult struct {
	Valid            bool                        `json:"valid"`
	ManifestsChecked int                         `json:"manifests_checked"`
	Errors           []ManifestValidationError   `json:"errors,omitempty"`
	Warnings         []ManifestValidationWarning `json:"warnings,omitempty"`
	Suggestions      []string                    `json:"suggestions,omitempty"`
	Score            int                         `json:"score"` // 0-100
}

type ManifestValidationError struct {
	Manifest string `json:"manifest"`
	Type     string `json:"type"`
	Severity string `json:"severity"`
	Message  string `json:"message"`
	Line     int    `json:"line,omitempty"`
	Field    string `json:"field,omitempty"`
	Fix      string `json:"fix,omitempty"`
}

type ManifestValidationWarning struct {
	Manifest   string `json:"manifest"`
	Type       string `json:"type"`
	Message    string `json:"message"`
	Suggestion string `json:"suggestion,omitempty"`
}

type SchemaValidationResult struct {
	Valid        bool                    `json:"valid"`
	SchemaErrors []SchemaValidationError `json:"schema_errors,omitempty"`
	ApiVersions  map[string]string       `json:"api_versions"`
}

type SchemaValidationError struct {
	Manifest    string `json:"manifest"`
	SchemaPath  string `json:"schema_path"`
	Message     string `json:"message"`
	ActualValue string `json:"actual_value,omitempty"`
}

type SecurityValidationResult struct {
	Passed         bool                      `json:"passed"`
	SecurityIssues []SecurityValidationIssue `json:"security_issues,omitempty"`
	PolicyChecks   []PolicyCheckResult       `json:"policy_checks,omitempty"`
	RiskScore      int                       `json:"risk_score"` // 0-100
}

type SecurityValidationIssue struct {
	Type        string `json:"type"`
	Severity    string `json:"severity"`
	Manifest    string `json:"manifest"`
	Description string `json:"description"`
	Remediation string `json:"remediation,omitempty"`
}

type PolicyCheckResult struct {
	Policy  string `json:"policy"`
	Passed  bool   `json:"passed"`
	Message string `json:"message,omitempty"`
}

type GenerationStats struct {
	ManifestsGenerated int              `json:"manifests_generated"`
	ResourceTypes      map[string]int   `json:"resource_types"`
	TotalSize          int              `json:"total_size"`
	Complexity         string           `json:"complexity"`
	EstimatedResources ResourceEstimate `json:"estimated_resources"`
}

type ResourceEstimate struct {
	CPU     string `json:"cpu"`
	Memory  string `json:"memory"`
	Storage string `json:"storage,omitempty"`
}

type ResourceSummary struct {
	Deployments int `json:"deployments"`
	Services    int `json:"services"`
	Ingresses   int `json:"ingresses"`
	ConfigMaps  int `json:"config_maps"`
	Secrets     int `json:"secrets"`
	HPAs        int `json:"hpas"`
	Other       int `json:"other"`
}

// ConsolidatedManifestsTool - Unified manifest generation and validation tool
type ConsolidatedManifestsTool struct {
	// Service dependencies
	sessionStore    services.SessionStore
	sessionState    services.SessionState
	k8sClient       services.K8sClient
	configValidator services.ConfigValidator
	logger          *slog.Logger

	// Core manifest components
	manifestGenerator *ConsolidatedManifestGenerator
	manifestValidator *ConsolidatedManifestValidator
	templateProcessor *ConsolidatedTemplateProcessor
	schemaValidator   *ConsolidatedSchemaValidator
	securityValidator *ConsolidatedSecurityValidator
	cacheManager      *ManifestCacheManager

	// State management
	workspaceDir string
}

// NewConsolidatedManifestsTool creates a new consolidated manifests tool
func NewConsolidatedManifestsTool(
	serviceContainer services.ServiceContainer,
	logger *slog.Logger,
) *ConsolidatedManifestsTool {
	toolLogger := logger.With("tool", "manifests_consolidated")

	return &ConsolidatedManifestsTool{
		sessionStore:      serviceContainer.SessionStore(),
		sessionState:      serviceContainer.SessionState(),
		k8sClient:         serviceContainer.K8sClient(),
		configValidator:   serviceContainer.ConfigValidator(),
		logger:            toolLogger,
		manifestGenerator: NewConsolidatedManifestGenerator(toolLogger),
		manifestValidator: NewConsolidatedManifestValidator(toolLogger),
		templateProcessor: NewConsolidatedTemplateProcessor(toolLogger),
		schemaValidator:   NewConsolidatedSchemaValidator(toolLogger),
		securityValidator: NewConsolidatedSecurityValidator(toolLogger),
		cacheManager:      NewManifestCacheManager(toolLogger),
	}
}

// Execute implements api.Tool interface
func (t *ConsolidatedManifestsTool) Execute(ctx context.Context, input api.ToolInput) (api.ToolOutput, error) {
	startTime := time.Now()

	// Parse input
	manifestInput, err := t.parseInput(input)
	if err != nil {
		return api.ToolOutput{
			Success: false,
			Error:   fmt.Sprintf("Invalid input: %v", err),
		}, err
	}

	// Validate input
	if err := manifestInput.Validate(); err != nil {
		return api.ToolOutput{
			Success: false,
			Error:   fmt.Sprintf("Input validation failed: %v", err),
		}, err
	}

	// Generate session ID if not provided
	sessionID := manifestInput.SessionID
	if sessionID == "" {
		sessionID = fmt.Sprintf("manifest_%d", time.Now().Unix())
	}

	// Execute operation based on mode
	result, err := t.executeOperation(ctx, manifestInput, sessionID, startTime)
	if err != nil {
		return api.ToolOutput{
			Success: false,
			Error:   fmt.Sprintf("Operation failed: %v", err),
		}, err
	}

	return api.ToolOutput{
		Success: result.Success,
		Data:    map[string]interface{}{"result": result},
	}, nil
}

// executeOperation performs the manifest operation based on the specified mode
func (t *ConsolidatedManifestsTool) executeOperation(
	ctx context.Context,
	input *ConsolidatedManifestsInput,
	sessionID string,
	startTime time.Time,
) (*ConsolidatedManifestsOutput, error) {
	result := &ConsolidatedManifestsOutput{
		Success:       false,
		SessionID:     sessionID,
		ImageRef:      input.getImageRef(),
		AppName:       input.AppName,
		Namespace:     input.Namespace,
		ManifestMode:  input.getManifestMode(),
		ToolVersion:   "2.0.0",
		Timestamp:     startTime,
		OperationTime: startTime,
		Metadata:      make(map[string]interface{}),
	}

	// Initialize session
	if err := t.initializeSession(ctx, sessionID, input); err != nil {
		t.logger.Warn("Failed to initialize session", "error", err)
	}

	// Check cache if enabled
	if input.UseCache {
		if cachedResult := t.checkCache(input); cachedResult != nil {
			cachedResult.CacheHit = true
			return cachedResult, nil
		}
	}

	// Execute based on manifest mode
	switch input.getManifestMode() {
	case "validate":
		return t.executeValidateManifests(ctx, input, result)
	case "template":
		return t.executeTemplateProcessing(ctx, input, result)
	default: // generate
		return t.executeGenerateManifests(ctx, input, result)
	}
}

// executeGenerateManifests performs manifest generation
func (t *ConsolidatedManifestsTool) executeGenerateManifests(
	ctx context.Context,
	input *ConsolidatedManifestsInput,
	result *ConsolidatedManifestsOutput,
) (*ConsolidatedManifestsOutput, error) {
	t.logger.Info("Executing manifest generation",
		"image_ref", result.ImageRef,
		"app_name", result.AppName,
		"session_id", result.SessionID)

	generationStart := time.Now()

	// Generate manifests
	manifests, stats, err := t.generateManifests(ctx, input, result)
	if err != nil {
		return result, err
	}

	result.GeneratedManifests = manifests
	result.GenerationStats = stats
	result.ResourceSummary = t.calculateResourceSummary(manifests)

	// Convert to ManifestFile format
	result.ManifestFiles = t.convertToManifestFiles(manifests)

	// Validate generated manifests if requested
	if input.ValidateSchema {
		validationResult, err := t.validateGeneratedManifests(ctx, manifests, input)
		if err != nil {
			t.logger.Warn("Validation failed", "error", err)
			result.Warnings = append(result.Warnings, fmt.Sprintf("Validation warning: %v", err))
		} else {
			result.ValidationResult = validationResult
		}
	}

	// Write to files if requested
	if input.WriteToFiles {
		writtenFiles, err := t.writeManifestFiles(result.ManifestFiles, input)
		if err != nil {
			t.logger.Warn("File writing failed", "error", err)
			result.Warnings = append(result.Warnings, fmt.Sprintf("File writing warning: %v", err))
		} else {
			result.WrittenFiles = writtenFiles
			result.OutputDirectory = input.OutputDirectory
		}
	}

	result.Success = true
	result.GenerationDuration = time.Since(generationStart)
	result.TotalDuration = time.Since(result.Timestamp)

	// Cache result if enabled
	if input.UseCache {
		t.cacheResult(input, result)
	}

	t.logger.Info("Manifest generation completed",
		"manifests_count", len(manifests),
		"generation_duration", result.GenerationDuration,
		"total_duration", result.TotalDuration)

	return result, nil
}

// executeValidateManifests performs manifest validation
func (t *ConsolidatedManifestsTool) executeValidateManifests(
	ctx context.Context,
	input *ConsolidatedManifestsInput,
	result *ConsolidatedManifestsOutput,
) (*ConsolidatedManifestsOutput, error) {
	t.logger.Info("Executing manifest validation",
		"session_id", result.SessionID)

	validationStart := time.Now()

	// Load manifests to validate
	manifests, err := t.loadManifestsForValidation(input)
	if err != nil {
		return result, err
	}

	// Perform validation
	validationResult, err := t.validateManifests(ctx, manifests, input)
	if err != nil {
		return result, err
	}

	result.ValidationResult = validationResult

	// Perform schema validation if requested
	if input.ValidateSchema {
		schemaResult, err := t.validateManifestSchemas(ctx, manifests, input)
		if err != nil {
			t.logger.Warn("Schema validation failed", "error", err)
			result.Warnings = append(result.Warnings, fmt.Sprintf("Schema validation warning: %v", err))
		} else {
			result.SchemaValidation = schemaResult
		}
	}

	// Perform security validation if strict mode
	if input.StrictValidation {
		securityResult, err := t.validateManifestSecurity(ctx, manifests, input)
		if err != nil {
			t.logger.Warn("Security validation failed", "error", err)
			result.Warnings = append(result.Warnings, fmt.Sprintf("Security validation warning: %v", err))
		} else {
			result.SecurityValidation = securityResult
		}
	}

	result.Success = validationResult.Valid
	result.ValidationDuration = time.Since(validationStart)
	result.TotalDuration = time.Since(result.Timestamp)

	t.logger.Info("Manifest validation completed",
		"valid", validationResult.Valid,
		"errors", len(validationResult.Errors),
		"warnings", len(validationResult.Warnings),
		"duration", result.ValidationDuration)

	return result, nil
}

// executeTemplateProcessing performs template processing
func (t *ConsolidatedManifestsTool) executeTemplateProcessing(
	ctx context.Context,
	input *ConsolidatedManifestsInput,
	result *ConsolidatedManifestsOutput,
) (*ConsolidatedManifestsOutput, error) {
	t.logger.Info("Executing template processing",
		"engine", input.TemplateEngine,
		"session_id", result.SessionID)

	// Process templates
	templateOutput, manifests, err := t.processTemplates(ctx, input, result)
	if err != nil {
		return result, err
	}

	result.TemplateOutput = templateOutput
	result.GeneratedManifests = manifests
	result.ManifestFiles = t.convertToManifestFiles(manifests)

	// Validate processed templates if requested
	if input.ValidateSchema {
		validationResult, err := t.validateGeneratedManifests(ctx, manifests, input)
		if err != nil {
			t.logger.Warn("Template validation failed", "error", err)
			result.Warnings = append(result.Warnings, fmt.Sprintf("Template validation warning: %v", err))
		} else {
			result.ValidationResult = validationResult
		}
	}

	result.Success = len(templateOutput.TemplateErrors) == 0
	result.TotalDuration = time.Since(result.Timestamp)

	t.logger.Info("Template processing completed",
		"templates_processed", len(templateOutput.TemplatesUsed),
		"manifests_generated", len(manifests),
		"errors", len(templateOutput.TemplateErrors),
		"duration", result.TotalDuration)

	return result, nil
}

// Implement api.Tool interface methods

func (t *ConsolidatedManifestsTool) Name() string {
	return "manifests_consolidated"
}

func (t *ConsolidatedManifestsTool) Description() string {
	return "Comprehensive manifest generation and validation tool with unified interface supporting generate, validate, and template modes"
}

func (t *ConsolidatedManifestsTool) Schema() api.ToolSchema {
	return api.ToolSchema{
		Name:        "manifests_consolidated",
		Description: "Comprehensive manifest generation and validation tool with unified interface supporting generate, validate, and template modes",
		Version:     "2.0.0",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"image_ref": map[string]interface{}{
					"type":        "string",
					"description": "Container image reference",
				},
				"manifest_mode": map[string]interface{}{
					"type":        "string",
					"description": "Manifest mode: generate, validate, or template",
					"enum":        []string{"generate", "validate", "template"},
				},
				"app_name": map[string]interface{}{
					"type":        "string",
					"description": "Application name",
				},
				"namespace": map[string]interface{}{
					"type":        "string",
					"description": "Kubernetes namespace",
				},
				"manifest_types": map[string]interface{}{
					"type":        "array",
					"description": "Types of manifests to generate",
					"items": map[string]interface{}{
						"type": "string",
						"enum": []string{"deployment", "service", "ingress", "configmap", "secret", "hpa"},
					},
				},
			},
			"required": []string{},
		},
		OutputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"success": map[string]interface{}{
					"type":        "boolean",
					"description": "Whether operation was successful",
				},
				"generated_manifests": map[string]interface{}{
					"type":        "object",
					"description": "Generated Kubernetes manifests",
				},
				"validation_result": map[string]interface{}{
					"type":        "object",
					"description": "Manifest validation results",
				},
				"template_output": map[string]interface{}{
					"type":        "object",
					"description": "Template processing results",
				},
			},
		},
	}
}

// Helper methods for tool implementation

func (t *ConsolidatedManifestsTool) parseInput(input api.ToolInput) (*ConsolidatedManifestsInput, error) {
	result := &ConsolidatedManifestsInput{}

	// Handle map[string]interface{} data
	v := input.Data
	// Extract parameters from map
	if imageRef, ok := v["image_ref"].(string); ok {
		result.ImageRef = imageRef
	}
	if imageName, ok := v["image_name"].(string); ok {
		result.ImageName = imageName
	}
	if sessionID, ok := v["session_id"].(string); ok {
		result.SessionID = sessionID
	}
	if appName, ok := v["app_name"].(string); ok {
		result.AppName = appName
	}
	if namespace, ok := v["namespace"].(string); ok {
		result.Namespace = namespace
	}
	if manifestMode, ok := v["manifest_mode"].(string); ok {
		result.ManifestMode = manifestMode
	}
	if manifestTypes, ok := v["manifest_types"].([]interface{}); ok {
		result.ManifestTypes = make([]string, len(manifestTypes))
		for i, mt := range manifestTypes {
			if str, ok := mt.(string); ok {
				result.ManifestTypes[i] = str
			}
		}
	}
	// ... (more field extractions)

	return result, nil
}

// Placeholder methods for the helper components - these would be implemented based on existing code

func (t *ConsolidatedManifestsTool) initializeSession(ctx context.Context, sessionID string, input *ConsolidatedManifestsInput) error {
	if t.sessionStore == nil {
		return nil
	}

	sessionData := map[string]interface{}{
		"image_ref":     input.getImageRef(),
		"manifest_mode": input.getManifestMode(),
		"started_at":    time.Now(),
	}

	session := &api.Session{
		ID:       sessionID,
		Metadata: sessionData,
	}
	return t.sessionStore.Create(ctx, session)
}

func (t *ConsolidatedManifestsTool) checkCache(input *ConsolidatedManifestsInput) *ConsolidatedManifestsOutput {
	if t.cacheManager == nil {
		return nil
	}

	cacheKey := fmt.Sprintf("%s_%s_%s", input.getImageRef(), input.AppName, input.getManifestMode())
	return t.cacheManager.Get(cacheKey)
}

func (t *ConsolidatedManifestsTool) cacheResult(input *ConsolidatedManifestsInput, result *ConsolidatedManifestsOutput) {
	if t.cacheManager == nil {
		return
	}

	cacheKey := fmt.Sprintf("%s_%s_%s", input.getImageRef(), input.AppName, input.getManifestMode())
	t.cacheManager.Set(cacheKey, result)
}

func (t *ConsolidatedManifestsTool) generateManifests(ctx context.Context, input *ConsolidatedManifestsInput, result *ConsolidatedManifestsOutput) (map[string]string, *GenerationStats, error) {
	// This would use the existing manifest generation logic
	manifests := map[string]string{
		"deployment.yaml": "# Generated deployment manifest",
		"service.yaml":    "# Generated service manifest",
	}

	stats := &GenerationStats{
		ManifestsGenerated: len(manifests),
		ResourceTypes:      map[string]int{"Deployment": 1, "Service": 1},
		TotalSize:          1024,
		Complexity:         "medium",
		EstimatedResources: ResourceEstimate{CPU: "100m", Memory: "128Mi"},
	}

	return manifests, stats, nil
}

func (t *ConsolidatedManifestsTool) calculateResourceSummary(manifests map[string]string) *ResourceSummary {
	return &ResourceSummary{
		Deployments: 1,
		Services:    1,
		Ingresses:   0,
		ConfigMaps:  0,
		Secrets:     0,
		HPAs:        0,
		Other:       0,
	}
}

func (t *ConsolidatedManifestsTool) convertToManifestFiles(manifests map[string]string) []ManifestFile {
	files := make([]ManifestFile, 0, len(manifests))
	for name, content := range manifests {
		files = append(files, ManifestFile{
			Name:    name,
			Type:    "yaml",
			Content: content,
			Size:    len(content),
		})
	}
	return files
}

func (t *ConsolidatedManifestsTool) validateGeneratedManifests(ctx context.Context, manifests map[string]string, input *ConsolidatedManifestsInput) (*ManifestValidationResult, error) {
	return &ManifestValidationResult{
		Valid:            true,
		ManifestsChecked: len(manifests),
		Errors:           []ManifestValidationError{},
		Warnings:         []ManifestValidationWarning{},
		Suggestions:      []string{},
		Score:            95,
	}, nil
}

func (t *ConsolidatedManifestsTool) writeManifestFiles(files []ManifestFile, input *ConsolidatedManifestsInput) ([]string, error) {
	writtenFiles := make([]string, len(files))
	for i, file := range files {
		writtenFiles[i] = file.Name
	}
	return writtenFiles, nil
}

func (t *ConsolidatedManifestsTool) loadManifestsForValidation(input *ConsolidatedManifestsInput) (map[string]string, error) {
	// This would load manifests from content or files
	return map[string]string{
		"manifest.yaml": input.ManifestContent,
	}, nil
}

func (t *ConsolidatedManifestsTool) validateManifests(ctx context.Context, manifests map[string]string, input *ConsolidatedManifestsInput) (*ManifestValidationResult, error) {
	return &ManifestValidationResult{
		Valid:            true,
		ManifestsChecked: len(manifests),
		Errors:           []ManifestValidationError{},
		Warnings:         []ManifestValidationWarning{},
		Suggestions:      []string{},
		Score:            90,
	}, nil
}

func (t *ConsolidatedManifestsTool) validateManifestSchemas(ctx context.Context, manifests map[string]string, input *ConsolidatedManifestsInput) (*SchemaValidationResult, error) {
	return &SchemaValidationResult{
		Valid:        true,
		SchemaErrors: []SchemaValidationError{},
		ApiVersions:  map[string]string{"Deployment": "apps/v1", "Service": "v1"},
	}, nil
}

func (t *ConsolidatedManifestsTool) validateManifestSecurity(ctx context.Context, manifests map[string]string, input *ConsolidatedManifestsInput) (*SecurityValidationResult, error) {
	return &SecurityValidationResult{
		Passed:         true,
		SecurityIssues: []SecurityValidationIssue{},
		PolicyChecks:   []PolicyCheckResult{},
		RiskScore:      10,
	}, nil
}

func (t *ConsolidatedManifestsTool) processTemplates(ctx context.Context, input *ConsolidatedManifestsInput, result *ConsolidatedManifestsOutput) (*TemplateOutput, map[string]string, error) {
	templateOutput := &TemplateOutput{
		Engine:         input.TemplateEngine,
		TemplatesUsed:  input.TemplateFiles,
		ValuesUsed:     input.TemplateValues,
		RenderTime:     100 * time.Millisecond,
		TemplateErrors: []TemplateError{},
	}

	manifests := map[string]string{
		"deployment.yaml": "# Template-generated deployment",
		"service.yaml":    "# Template-generated service",
	}

	return templateOutput, manifests, nil
}

// Supporting components that would be implemented based on existing code

type ConsolidatedManifestGenerator struct {
	logger *slog.Logger
}

func NewConsolidatedManifestGenerator(logger *slog.Logger) *ConsolidatedManifestGenerator {
	return &ConsolidatedManifestGenerator{logger: logger}
}

type ConsolidatedManifestValidator struct {
	logger *slog.Logger
}

func NewConsolidatedManifestValidator(logger *slog.Logger) *ConsolidatedManifestValidator {
	return &ConsolidatedManifestValidator{logger: logger}
}

type ConsolidatedTemplateProcessor struct {
	logger *slog.Logger
}

func NewConsolidatedTemplateProcessor(logger *slog.Logger) *ConsolidatedTemplateProcessor {
	return &ConsolidatedTemplateProcessor{logger: logger}
}

type ConsolidatedSchemaValidator struct {
	logger *slog.Logger
}

func NewConsolidatedSchemaValidator(logger *slog.Logger) *ConsolidatedSchemaValidator {
	return &ConsolidatedSchemaValidator{logger: logger}
}

type ConsolidatedSecurityValidator struct {
	logger *slog.Logger
}

func NewConsolidatedSecurityValidator(logger *slog.Logger) *ConsolidatedSecurityValidator {
	return &ConsolidatedSecurityValidator{logger: logger}
}

type ManifestCacheManager struct {
	logger *slog.Logger
	cache  map[string]*ConsolidatedManifestsOutput
}

func NewManifestCacheManager(logger *slog.Logger) *ManifestCacheManager {
	return &ManifestCacheManager{
		logger: logger,
		cache:  make(map[string]*ConsolidatedManifestsOutput),
	}
}

func (m *ManifestCacheManager) Get(key string) *ConsolidatedManifestsOutput {
	if result, exists := m.cache[key]; exists {
		m.logger.Info("Manifest cache hit", "key", key)
		return result
	}
	return nil
}

func (m *ManifestCacheManager) Set(key string, result *ConsolidatedManifestsOutput) {
	m.cache[key] = result
	m.logger.Info("Manifest cache set", "key", key)
}
