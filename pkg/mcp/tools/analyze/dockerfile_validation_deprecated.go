package analyze

import (
	"context"
	"log/slog"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/application/api"
	"github.com/Azure/container-kit/pkg/mcp/core"
)

// Register deprecated Dockerfile validation tools with compatibility wrappers
func init() {
	// Register deprecated tools with deprecation warnings
	core.RegisterTool("validate_dockerfile_atomic", func() api.Tool {
		return &DeprecatedValidateDockerfileAtomicTool{}
	})

	core.RegisterTool("dockerfile_validation_core", func() api.Tool {
		return &DeprecatedDockerfileValidationCoreTool{}
	})
}

// DeprecatedValidateDockerfileAtomicTool - Compatibility wrapper for validate_dockerfile_atomic
type DeprecatedValidateDockerfileAtomicTool struct {
	consolidatedTool *ConsolidatedValidateDockerfileTool
}

func (t *DeprecatedValidateDockerfileAtomicTool) Name() string {
	return "validate_dockerfile_atomic"
}

func (t *DeprecatedValidateDockerfileAtomicTool) Description() string {
	return "DEPRECATED: Use 'validate_dockerfile' instead. Atomic Dockerfile validation tool."
}

func (t *DeprecatedValidateDockerfileAtomicTool) Schema() api.ToolSchema {
	return api.ToolSchema{
		Name:        "validate_dockerfile_atomic",
		Description: "DEPRECATED: Use 'validate_dockerfile' instead. Atomic Dockerfile validation tool.",
		Version:     "1.0.0-deprecated",
		InputSchema: map[string]interface{}{
			"deprecated": true,
			"message":    "This tool is deprecated. Use 'validate_dockerfile' instead.",
		},
	}
}

func (t *DeprecatedValidateDockerfileAtomicTool) Execute(ctx context.Context, input api.ToolInput) (api.ToolOutput, error) {
	// Log deprecation warning
	logger := slog.Default().With("tool", "validate_dockerfile_atomic")
	logger.Warn("DEPRECATED TOOL USED",
		"tool", "validate_dockerfile_atomic",
		"replacement", "validate_dockerfile",
		"message", "This tool will be removed in a future version. Please use 'validate_dockerfile' instead.")

	// Create consolidated tool if not exists
	if t.consolidatedTool == nil {
		t.consolidatedTool = &ConsolidatedValidateDockerfileTool{
			logger: logger,
		}
	}

	// Convert input format for backward compatibility
	convertedInput := t.convertAtomicInput(input)

	// Delegate to consolidated tool
	return t.consolidatedTool.Execute(ctx, convertedInput)
}

// convertAtomicInput converts legacy atomic tool input to new format
func (t *DeprecatedValidateDockerfileAtomicTool) convertAtomicInput(input api.ToolInput) api.ToolInput {
	converted := api.ToolInput{
		Arguments: make(map[string]interface{}),
	}

	switch v := input.Arguments.(type) {
	case map[string]interface{}:
		// Map legacy parameters to new format
		if sessionID, ok := v["session_id"].(string); ok {
			converted.Arguments.(map[string]interface{})["session_id"] = sessionID
		}
		if dockerfilePath, ok := v["dockerfile_path"].(string); ok {
			converted.Arguments.(map[string]interface{})["dockerfile_path"] = dockerfilePath
		}
		if dockerfileContent, ok := v["dockerfile_content"].(string); ok {
			converted.Arguments.(map[string]interface{})["dockerfile_content"] = dockerfileContent
		}
		if severity, ok := v["severity"].(string); ok {
			converted.Arguments.(map[string]interface{})["severity"] = severity
		}
		if checkSecurity, ok := v["check_security"].(bool); ok {
			converted.Arguments.(map[string]interface{})["check_security"] = checkSecurity
		}
		if useHadolint, ok := v["use_hadolint"].(bool); ok {
			converted.Arguments.(map[string]interface{})["use_hadolint"] = useHadolint
		}

		// Enable features that were default in atomic tool
		converted.Arguments.(map[string]interface{})["check_best_practices"] = true
		converted.Arguments.(map[string]interface{})["include_analysis"] = true
	}

	return converted
}

// DeprecatedDockerfileValidationCoreTool - Compatibility wrapper for dockerfile_validation_core
type DeprecatedDockerfileValidationCoreTool struct {
	consolidatedTool *ConsolidatedValidateDockerfileTool
}

func (t *DeprecatedDockerfileValidationCoreTool) Name() string {
	return "dockerfile_validation_core"
}

func (t *DeprecatedDockerfileValidationCoreTool) Description() string {
	return "DEPRECATED: Use 'validate_dockerfile' instead. Core Dockerfile validation functionality."
}

func (t *DeprecatedDockerfileValidationCoreTool) Schema() api.ToolSchema {
	return api.ToolSchema{
		Name:        "dockerfile_validation_core",
		Description: "DEPRECATED: Use 'validate_dockerfile' instead. Core Dockerfile validation functionality.",
		Version:     "1.0.0-deprecated",
		InputSchema: map[string]interface{}{
			"deprecated": true,
			"message":    "This tool is deprecated. Use 'validate_dockerfile' instead.",
		},
	}
}

func (t *DeprecatedDockerfileValidationCoreTool) Execute(ctx context.Context, input api.ToolInput) (api.ToolOutput, error) {
	// Log deprecation warning
	logger := slog.Default().With("tool", "dockerfile_validation_core")
	logger.Warn("DEPRECATED TOOL USED",
		"tool", "dockerfile_validation_core",
		"replacement", "validate_dockerfile",
		"message", "This tool will be removed in a future version. Please use 'validate_dockerfile' instead.")

	// Create consolidated tool if not exists
	if t.consolidatedTool == nil {
		t.consolidatedTool = &ConsolidatedValidateDockerfileTool{
			logger: logger,
		}
	}

	// Convert input format for backward compatibility
	convertedInput := t.convertCoreInput(input)

	// Delegate to consolidated tool
	return t.consolidatedTool.Execute(ctx, convertedInput)
}

// convertCoreInput converts legacy core tool input to new format
func (t *DeprecatedDockerfileValidationCoreTool) convertCoreInput(input api.ToolInput) api.ToolInput {
	converted := api.ToolInput{
		Arguments: make(map[string]interface{}),
	}

	switch v := input.Arguments.(type) {
	case map[string]interface{}:
		// Map legacy parameters to new format
		if sessionID, ok := v["session_id"].(string); ok {
			converted.Arguments.(map[string]interface{})["session_id"] = sessionID
		}
		if dockerfilePath, ok := v["dockerfile_path"].(string); ok {
			converted.Arguments.(map[string]interface{})["dockerfile_path"] = dockerfilePath
		}
		if dockerfileContent, ok := v["dockerfile_content"].(string); ok {
			converted.Arguments.(map[string]interface{})["dockerfile_content"] = dockerfileContent
		}
		if severity, ok := v["severity"].(string); ok {
			converted.Arguments.(map[string]interface{})["severity"] = severity
		}

		// Enable core features
		converted.Arguments.(map[string]interface{})["check_security"] = true
		converted.Arguments.(map[string]interface{})["check_best_practices"] = true
		converted.Arguments.(map[string]interface{})["rule_set"] = "basic"
	}

	return converted
}

// DockerfileValidationMigrationGuide provides migration guidance for Dockerfile validation tools
type DockerfileValidationMigrationGuide struct {
	OldTool          string
	NewTool          string
	ParameterChanges map[string]string
	Examples         []DockerfileValidationMigrationExample
}

// DockerfileValidationMigrationExample shows how to migrate from old to new tool usage
type DockerfileValidationMigrationExample struct {
	OldUsage    string
	NewUsage    string
	Description string
}

// GetDockerfileValidationMigrationGuide returns migration guidance for deprecated Dockerfile validation tools
func GetDockerfileValidationMigrationGuide(oldTool string) *DockerfileValidationMigrationGuide {
	guides := map[string]*DockerfileValidationMigrationGuide{
		"validate_dockerfile_atomic": {
			OldTool: "validate_dockerfile_atomic",
			NewTool: "validate_dockerfile",
			ParameterChanges: map[string]string{
				"session_id":         "session_id",
				"dockerfile_path":    "dockerfile_path",
				"dockerfile_content": "dockerfile_content",
				"severity":           "severity",
				"check_security":     "check_security",
				"use_hadolint":       "use_hadolint",
			},
			Examples: []DockerfileValidationMigrationExample{
				{
					OldUsage: `{
  "tool": "validate_dockerfile_atomic",
  "parameters": {
    "session_id": "session-123",
    "dockerfile_path": "./Dockerfile",
    "severity": "warning",
    "check_security": true,
    "use_hadolint": true
  }
}`,
					NewUsage: `{
  "tool": "validate_dockerfile",
  "parameters": {
    "session_id": "session-123",
    "dockerfile_path": "./Dockerfile",
    "severity": "warning",
    "check_security": true,
    "check_best_practices": true,
    "use_hadolint": true,
    "include_analysis": true,
    "include_fixes": true
  }
}`,
					Description: "Atomic Dockerfile validation with enhanced analysis and fixes",
				},
			},
		},
		"dockerfile_validation_core": {
			OldTool: "dockerfile_validation_core",
			NewTool: "validate_dockerfile",
			ParameterChanges: map[string]string{
				"session_id":         "session_id",
				"dockerfile_path":    "dockerfile_path",
				"dockerfile_content": "dockerfile_content",
				"severity":           "severity",
			},
			Examples: []DockerfileValidationMigrationExample{
				{
					OldUsage: `{
  "tool": "dockerfile_validation_core",
  "parameters": {
    "session_id": "session-123",
    "dockerfile_path": "./Dockerfile",
    "severity": "error"
  }
}`,
					NewUsage: `{
  "tool": "validate_dockerfile",
  "parameters": {
    "session_id": "session-123",
    "dockerfile_path": "./Dockerfile",
    "severity": "error",
    "check_security": true,
    "check_best_practices": true,
    "rule_set": "basic",
    "include_analysis": true
  }
}`,
					Description: "Core Dockerfile validation with comprehensive analysis",
				},
			},
		},
		"dockerfile_validation_analysis": {
			OldTool: "dockerfile_validation_analysis",
			NewTool: "validate_dockerfile_analysis",
			ParameterChanges: map[string]string{
				"dockerfile_path":    "dockerfile_path",
				"dockerfile_content": "dockerfile_content",
			},
			Examples: []DockerfileValidationMigrationExample{
				{
					OldUsage: `{
  "tool": "dockerfile_validation_analysis",
  "parameters": {
    "dockerfile_path": "./Dockerfile"
  }
}`,
					NewUsage: `{
  "tool": "validate_dockerfile_analysis",
  "parameters": {
    "dockerfile_path": "./Dockerfile"
  }
}`,
					Description: "Comprehensive Dockerfile analysis with automatic enhancements",
				},
			},
		},
	}

	return guides[oldTool]
}

// DockerfileValidationDeprecationNotice provides structured deprecation information
type DockerfileValidationDeprecationNotice struct {
	Tool            string                              `json:"tool"`
	Status          string                              `json:"status"`
	Replacement     string                              `json:"replacement"`
	RemovalVersion  string                              `json:"removal_version"`
	MigrationGuide  *DockerfileValidationMigrationGuide `json:"migration_guide"`
	DeprecationDate time.Time                           `json:"deprecation_date"`
	SupportEndDate  time.Time                           `json:"support_end_date"`
}

// GetDockerfileValidationDeprecationNotice returns structured deprecation information
func GetDockerfileValidationDeprecationNotice(tool string) *DockerfileValidationDeprecationNotice {
	notices := map[string]*DockerfileValidationDeprecationNotice{
		"validate_dockerfile_atomic": {
			Tool:            "validate_dockerfile_atomic",
			Status:          "deprecated",
			Replacement:     "validate_dockerfile",
			RemovalVersion:  "3.0.0",
			MigrationGuide:  GetDockerfileValidationMigrationGuide("validate_dockerfile_atomic"),
			DeprecationDate: time.Now(),
			SupportEndDate:  time.Now().Add(90 * 24 * time.Hour),
		},
		"dockerfile_validation_core": {
			Tool:            "dockerfile_validation_core",
			Status:          "deprecated",
			Replacement:     "validate_dockerfile",
			RemovalVersion:  "3.0.0",
			MigrationGuide:  GetDockerfileValidationMigrationGuide("dockerfile_validation_core"),
			DeprecationDate: time.Now(),
			SupportEndDate:  time.Now().Add(90 * 24 * time.Hour),
		},
		"dockerfile_validation_analysis": {
			Tool:            "dockerfile_validation_analysis",
			Status:          "deprecated",
			Replacement:     "validate_dockerfile_analysis",
			RemovalVersion:  "3.0.0",
			MigrationGuide:  GetDockerfileValidationMigrationGuide("dockerfile_validation_analysis"),
			DeprecationDate: time.Now(),
			SupportEndDate:  time.Now().Add(90 * 24 * time.Hour),
		},
	}

	return notices[tool]
}

// PrintDockerfileValidationMigrationGuide prints migration guidance to logs
func PrintDockerfileValidationMigrationGuide(oldTool string, logger *slog.Logger) {
	guide := GetDockerfileValidationMigrationGuide(oldTool)
	if guide == nil {
		return
	}

	logger.Info("DOCKERFILE VALIDATION MIGRATION GUIDE",
		"old_tool", guide.OldTool,
		"new_tool", guide.NewTool,
		"parameter_changes", guide.ParameterChanges)

	for _, example := range guide.Examples {
		logger.Info("DOCKERFILE VALIDATION MIGRATION EXAMPLE",
			"description", example.Description,
			"old_usage", example.OldUsage,
			"new_usage", example.NewUsage)
	}
}
