package tools

import (
	"context"
	"fmt"

	"github.com/Azure/container-copilot/pkg/mcp/internal/store/session"
	"github.com/Azure/container-copilot/pkg/mcp/internal/types"
	"github.com/rs/zerolog"
)

// GenerateDockerfileAdapterArgs defines strongly-typed arguments for the new interface
type GenerateDockerfileAdapterArgs struct {
	SessionID          string            `json:"session_id" jsonschema:"required,description=Session ID from analyze_repository"`
	DryRun             bool              `json:"dry_run,omitempty" jsonschema:"description=Perform dry run without writing files"`
	BaseImage          string            `json:"base_image,omitempty" jsonschema:"description=Override detected base image"`
	Template           string            `json:"template,omitempty" jsonschema:"description=Use specific template (go, node, python, etc.)"`
	Optimization       string            `json:"optimization,omitempty" jsonschema:"description=Optimization level (size, speed, security),enum=size,enum=speed,enum=security"`
	IncludeHealthCheck bool              `json:"include_health_check,omitempty" jsonschema:"description=Add health check to Dockerfile"`
	BuildArgs          map[string]string `json:"build_args,omitempty" jsonschema:"description=Docker build arguments"`
	Platform           string            `json:"platform,omitempty" jsonschema:"description=Target platform (e.g. linux/amd64)"`
}

// GenerateDockerfileAdapterResult defines strongly-typed response
type GenerateDockerfileAdapterResult struct {
	Success      bool     `json:"success" jsonschema:"description=Whether generation succeeded"`
	SessionID    string   `json:"session_id" jsonschema:"description=Session ID"`
	Content      string   `json:"content" jsonschema:"description=Generated Dockerfile content"`
	BaseImage    string   `json:"base_image" jsonschema:"description=Base image used"`
	ExposedPorts []int    `json:"exposed_ports" jsonschema:"description=Ports exposed by the application"`
	HealthCheck  string   `json:"health_check,omitempty" jsonschema:"description=Health check command if included"`
	BuildSteps   []string `json:"build_steps" jsonschema:"description=Build steps included"`
	Template     string   `json:"template_used" jsonschema:"description=Template that was used"`
	FilePath     string   `json:"file_path" jsonschema:"description=Path where Dockerfile was written"`
	Message      string   `json:"message,omitempty" jsonschema:"description=Additional information"`
}

// GenerateDockerfileAdapter adapts the legacy tool to the new interface
type GenerateDockerfileAdapter struct {
	legacyTool *GenerateDockerfileTool
	logger     zerolog.Logger
}

// NewGenerateDockerfileAdapter creates a new adapter for the generate_dockerfile tool
func NewGenerateDockerfileAdapter(sessionManager *session.SessionManager, logger zerolog.Logger) ExecutableTool[GenerateDockerfileAdapterArgs, GenerateDockerfileAdapterResult] {
	legacyTool := NewGenerateDockerfileTool(sessionManager, logger)

	adapter := &GenerateDockerfileAdapter{
		legacyTool: legacyTool,
		logger:     logger.With().Str("component", "dockerfile_adapter").Logger(),
	}

	// Return the adapter directly - no legacy wrapper needed
	return adapter
}

// Execute implements the new ExecutableTool interface
func (a *GenerateDockerfileAdapter) Execute(ctx context.Context, args GenerateDockerfileAdapterArgs) (*GenerateDockerfileAdapterResult, error) {
	a.logger.Info().
		Str("session_id", args.SessionID).
		Str("template", args.Template).
		Str("optimization", args.Optimization).
		Bool("dry_run", args.DryRun).
		Msg("Executing Dockerfile generation via adapter")

	// Convert new args to legacy args
	legacyArgs := GenerateDockerfileArgs{
		BaseToolArgs: types.BaseToolArgs{
			SessionID: args.SessionID,
			DryRun:    args.DryRun,
		},
		BaseImage:          args.BaseImage,
		Template:           args.Template,
		Optimization:       args.Optimization,
		IncludeHealthCheck: args.IncludeHealthCheck,
		BuildArgs:          args.BuildArgs,
		Platform:           args.Platform,
	}

	// Execute legacy tool
	legacyResult, err := a.legacyTool.Execute(ctx, legacyArgs)
	if err != nil {
		return nil, types.NewRichError("INTERNAL_SERVER_ERROR", "legacy tool execution failed: "+err.Error(), "execution_error")
	}

	// Convert legacy result to new result
	// Success is true if we got here without error
	result := &GenerateDockerfileAdapterResult{
		Success:      true, // If we reach here, execution was successful
		SessionID:    legacyResult.SessionID,
		Content:      legacyResult.Content,
		BaseImage:    legacyResult.BaseImage,
		ExposedPorts: legacyResult.ExposedPorts,
		HealthCheck:  legacyResult.HealthCheck,
		BuildSteps:   legacyResult.BuildSteps,
		Template:     legacyResult.Template,
		FilePath:     legacyResult.FilePath,
		Message:      legacyResult.Message,
	}

	a.logger.Info().
		Str("session_id", args.SessionID).
		Bool("success", result.Success).
		Str("file_path", result.FilePath).
		Msg("Dockerfile generation completed via adapter")

	return result, nil
}

// PreValidate implements validation for the new interface
func (a *GenerateDockerfileAdapter) PreValidate(ctx context.Context, args GenerateDockerfileAdapterArgs) error {
	// Validate required fields
	if args.SessionID == "" {
		return types.NewRichError("INVALID_ARGUMENTS", "session_id is required", "validation_error")
	}

	// Validate optimization level if provided
	if args.Optimization != "" {
		validOptimizations := map[string]bool{
			"size":     true,
			"speed":    true,
			"security": true,
		}
		if !validOptimizations[args.Optimization] {
			return types.NewRichError("INVALID_ARGUMENTS", fmt.Sprintf("invalid optimization level: %s (must be size, speed, or security)", args.Optimization), "validation_error")
		}
	}

	// Validate platform format if provided
	if args.Platform != "" && args.Platform != "linux/amd64" && args.Platform != "linux/arm64" {
		a.logger.Warn().
			Str("platform", args.Platform).
			Msg("Unusual platform specified, this may cause build issues")
	}

	a.logger.Debug().
		Str("session_id", args.SessionID).
		Msg("Pre-validation passed for generate_dockerfile")

	return nil
}

// Tool interface implementation
func (a *GenerateDockerfileAdapter) GetName() string {
	return "generate_dockerfile"
}

func (a *GenerateDockerfileAdapter) GetDescription() string {
	return "Generate optimized Dockerfile based on repository analysis and templates"
}

func (a *GenerateDockerfileAdapter) GetVersion() string {
	return "1.0.0"
}

func (a *GenerateDockerfileAdapter) GetCapabilities() ToolCapabilities {
	return ToolCapabilities{
		SupportsDryRun:    true,
		SupportsStreaming: false,
		IsLongRunning:     false,
		RequiresAuth:      false,
	}
}

func (a *GenerateDockerfileAdapter) GetInputSchema() map[string]interface{} {
	// Return empty schema - GoMCP will auto-generate from struct tags
	return map[string]interface{}{}
}

func (a *GenerateDockerfileAdapter) GetOutputSchema() map[string]interface{} {
	// Return empty schema - GoMCP will auto-generate from struct tags
	return map[string]interface{}{}
}

// Example of how to register this tool in the registry
func RegisterGenerateDockerfileAdapter(registry *ToolRegistry, sessionManager *session.SessionManager, logger zerolog.Logger) error {
	adapter := NewGenerateDockerfileAdapter(sessionManager, logger)
	return RegisterTool(registry, adapter)
}
