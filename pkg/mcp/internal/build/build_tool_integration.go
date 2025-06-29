package build

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Azure/container-kit/pkg/mcp"
	mcptypes "github.com/Azure/container-kit/pkg/mcp"
	"github.com/rs/zerolog"
)

// BuildImageWithFixes demonstrates how to integrate fixing with the build image atomic tool
type BuildImageWithFixes struct {
	originalTool interface{} // Reference to AtomicBuildImageTool
	fixingMixin  *AtomicToolFixingMixin
	logger       zerolog.Logger
}

// NewBuildImageWithFixes creates a build tool with integrated fixing
func NewBuildImageWithFixes(analyzer mcp.AIAnalyzer, logger zerolog.Logger) *BuildImageWithFixes {
	return &BuildImageWithFixes{
		fixingMixin: NewAtomicToolFixingMixin(analyzer, "atomic_build_image", logger),
		logger:      logger.With().Str("component", "build_image_with_fixes").Logger(),
	}
}

// ExecuteWithFixes demonstrates the pattern for adding fixes to atomic tools
func (b *BuildImageWithFixes) ExecuteWithFixes(ctx context.Context, sessionID string, imageName string, dockerfilePath string, buildContext string) error {
	// Validate inputs
	if imageName == "" {
		return fmt.Errorf("image name is required")
	}
	if dockerfilePath == "" {
		dockerfilePath = filepath.Join(buildContext, "Dockerfile")
	}
	b.logger.Info().
		Str("session_id", sessionID).
		Str("image_name", imageName).
		Str("dockerfile_path", dockerfilePath).
		Str("build_context", buildContext).
		Msg("Starting Docker build with AI-driven fixing")
	// Create the fixable operation
	operation := &IntegratedDockerBuildOperation{
		SessionID:      sessionID,
		ImageName:      imageName,
		DockerfilePath: dockerfilePath,
		BuildContext:   buildContext,
		logger:         b.logger,
	}
	// Execute with retry and fixing
	return b.fixingMixin.ExecuteWithRetry(ctx, sessionID, buildContext, operation)
}

// IntegratedDockerBuildOperation implements mcptypes.FixableOperation for Docker builds
type IntegratedDockerBuildOperation struct {
	SessionID      string
	ImageName      string
	DockerfilePath string
	BuildContext   string
	logger         zerolog.Logger
	lastError      error
}

// ExecuteOnce performs a single Docker build attempt
func (op *IntegratedDockerBuildOperation) ExecuteOnce(ctx context.Context) error {
	op.logger.Debug().
		Str("image_name", op.ImageName).
		Str("dockerfile_path", op.DockerfilePath).
		Msg("Executing Docker build")
	// Check if Dockerfile exists
	if _, err := os.Stat(op.DockerfilePath); os.IsNotExist(err) {
		return &mcp.RichError{
			Code:     "DOCKERFILE_NOT_FOUND",
			Type:     "dockerfile_error",
			Severity: "High",
			Message:  fmt.Sprintf("Dockerfile not found at %s", op.DockerfilePath),
		}
	}
	// Simulate Docker build execution
	// In real implementation, this would call the actual Docker build
	buildError := op.simulateBuild(ctx)
	return buildError
}

// GetFailureAnalysis analyzes why the Docker build failed
func (op *IntegratedDockerBuildOperation) GetFailureAnalysis(ctx context.Context, err error) (*mcp.RichError, error) {
	op.logger.Debug().Err(err).Msg("Analyzing Docker build failure")
	// If it's already a RichError, return it
	if richErr, ok := err.(*mcp.RichError); ok {
		return richErr, nil
	}
	// Analyze the error message to categorize the failure
	errorMsg := err.Error()
	if strings.Contains(errorMsg, "no such file or directory") {
		return &mcp.RichError{
			Code:     "FILE_NOT_FOUND",
			Type:     "dockerfile_error",
			Severity: "High",
			Message:  errorMsg,
		}, nil
	}
	if strings.Contains(errorMsg, "unable to find image") {
		return &mcp.RichError{
			Code:     "BASE_IMAGE_NOT_FOUND",
			Type:     "dependency_error",
			Severity: "High",
			Message:  errorMsg,
		}, nil
	}
	if strings.Contains(errorMsg, "package not found") || strings.Contains(errorMsg, "command not found") {
		return &mcp.RichError{
			Code:     "PACKAGE_INSTALL_FAILED",
			Type:     "dependency_error",
			Severity: "Medium",
			Message:  errorMsg,
		}, nil
	}
	// Default categorization
	return &mcp.RichError{
		Code:     "BUILD_FAILED",
		Type:     "build_error",
		Severity: "High",
		Message:  errorMsg,
	}, nil
}

// PrepareForRetry applies fixes and prepares for the next build attempt
func (op *IntegratedDockerBuildOperation) PrepareForRetry(ctx context.Context, fixAttempt *mcp.FixAttempt) error {
	op.logger.Info().
		Str("fix_strategy", fixAttempt.FixStrategy.Name).
		Msg("Preparing for retry after fix")
	// Apply fix based on the strategy type
	switch fixAttempt.FixStrategy.Type {
	case "dockerfile":
		return op.applyDockerfileFix(ctx, fixAttempt)
	case "dependency":
		return op.applyDependencyFix(ctx, fixAttempt)
	case "config":
		return op.applyConfigFix(ctx, fixAttempt)
	default:
		op.logger.Warn().
			Str("fix_type", fixAttempt.FixStrategy.Type).
			Msg("Unknown fix type, applying generic fix")
		return op.applyGenericFix(ctx, fixAttempt)
	}
}

// applyDockerfileFix applies fixes to the Dockerfile
func (op *IntegratedDockerBuildOperation) applyDockerfileFix(ctx context.Context, fixAttempt *mcp.FixAttempt) error {
	if fixAttempt.FixedContent == "" {
		return fmt.Errorf("no fixed Dockerfile content provided")
	}
	// Backup the original Dockerfile
	backupPath := op.DockerfilePath + ".backup"
	if err := op.backupFile(op.DockerfilePath, backupPath); err != nil {
		op.logger.Warn().Err(err).Msg("Failed to backup Dockerfile")
	}
	// Write the fixed Dockerfile
	err := os.WriteFile(op.DockerfilePath, []byte(fixAttempt.FixedContent), 0600)
	if err != nil {
		return fmt.Errorf("failed to write fixed Dockerfile: %w", err)
	}
	op.logger.Info().
		Str("dockerfile_path", op.DockerfilePath).
		Msg("Applied Dockerfile fix")
	return nil
}

// applyDependencyFix applies dependency-related fixes
func (op *IntegratedDockerBuildOperation) applyDependencyFix(ctx context.Context, fixAttempt *mcp.FixAttempt) error {
	op.logger.Info().
		Str("fix_type", "dependency").
		Int("file_changes", len(fixAttempt.FixStrategy.FileChanges)).
		Msg("Applying dependency fix")
	// Apply file changes specified in the fix strategy
	for _, change := range fixAttempt.FixStrategy.FileChanges {
		if err := op.applyFileChange(change); err != nil {
			return fmt.Errorf("failed to apply dependency fix to %s: %w", change.FilePath, err)
		}
		op.logger.Info().
			Str("file", change.FilePath).
			Str("operation", change.Operation).
			Str("reason", change.Reason).
			Msg("Applied dependency file change")
	}
	// Execute any commands specified in the fix strategy
	for _, cmd := range fixAttempt.FixStrategy.Commands {
		op.logger.Info().
			Str("command", cmd).
			Msg("Dependency fix command identified (execution delegated to build tool)")
	}
	return nil
}

// applyConfigFix applies configuration-related fixes
func (op *IntegratedDockerBuildOperation) applyConfigFix(ctx context.Context, fixAttempt *mcp.FixAttempt) error {
	op.logger.Info().
		Str("fix_type", "config").
		Int("file_changes", len(fixAttempt.FixStrategy.FileChanges)).
		Msg("Applying configuration fix")
	// Apply file changes for configuration fixes
	for _, change := range fixAttempt.FixStrategy.FileChanges {
		if err := op.applyFileChange(change); err != nil {
			return fmt.Errorf("failed to apply config fix to %s: %w", change.FilePath, err)
		}
		op.logger.Info().
			Str("file", change.FilePath).
			Str("operation", change.Operation).
			Str("reason", change.Reason).
			Msg("Applied configuration file change")
	}
	// Handle specific configuration patterns
	if fixAttempt.FixedContent != "" {
		// If we have fixed content, apply it as a Dockerfile fix
		return op.applyDockerfileFix(ctx, fixAttempt)
	}
	return nil
}

// applyGenericFix applies generic fixes
func (op *IntegratedDockerBuildOperation) applyGenericFix(ctx context.Context, fixAttempt *mcp.FixAttempt) error {
	// Generic fix application
	if fixAttempt.FixedContent != "" {
		return op.applyDockerfileFix(ctx, fixAttempt)
	}
	op.logger.Info().Msg("Applied generic fix (no specific action needed)")
	return nil
}

// applyFileChange applies a single file change operation
func (op *IntegratedDockerBuildOperation) applyFileChange(change mcptypes.FileChange) error {
	filePath := filepath.Join(op.BuildContext, change.FilePath)
	switch change.Operation {
	case "create":
		// Create directory if needed
		dir := filepath.Dir(filePath)
		if err := os.MkdirAll(dir, 0750); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
		// Write the new file
		if err := os.WriteFile(filePath, []byte(change.NewContent), 0600); err != nil {
			return fmt.Errorf("failed to create file %s: %w", filePath, err)
		}
	case "update", "replace":
		// Create backup
		backupPath := filePath + ".backup"
		if err := op.backupFile(filePath, backupPath); err != nil {
			op.logger.Warn().Err(err).Msg("Failed to create backup")
		}
		// Write the updated content
		if err := os.WriteFile(filePath, []byte(change.NewContent), 0600); err != nil {
			return fmt.Errorf("failed to update file %s: %w", filePath, err)
		}
	case "delete":
		// Create backup before deletion
		backupPath := filePath + ".backup"
		if err := op.backupFile(filePath, backupPath); err != nil {
			op.logger.Warn().Err(err).Msg("Failed to create backup before deletion")
		}
		// Remove the file
		if err := os.Remove(filePath); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("failed to delete file %s: %w", filePath, err)
		}
	default:
		return fmt.Errorf("unknown file operation: %s", change.Operation)
	}
	return nil
}

// backupFile creates a backup of a file
func (op *IntegratedDockerBuildOperation) backupFile(source, backup string) error {
	// Clean paths to prevent directory traversal
	cleanSource := filepath.Clean(source)
	cleanBackup := filepath.Clean(backup)
	data, err := os.ReadFile(cleanSource)
	if err != nil {
		return err
	}
	return os.WriteFile(cleanBackup, data, 0600)
}

// simulateBuild simulates a Docker build for demonstration
func (op *IntegratedDockerBuildOperation) simulateBuild(ctx context.Context) error {
	// This is a simulation - in real implementation, this would:
	// 1. Execute docker build command
	// 2. Parse build output
	// 3. Return appropriate errors
	// Read Dockerfile to simulate analysis
	dockerfileContent, err := os.ReadFile(op.DockerfilePath)
	if err != nil {
		return fmt.Errorf("failed to read Dockerfile: %w", err)
	}
	content := string(dockerfileContent)
	// Simulate some common build failures
	if strings.Contains(content, "FROM nonexistent:latest") {
		return fmt.Errorf("unable to find image 'nonexistent:latest' locally")
	}
	if strings.Contains(content, "RUN apt-get install nonexistent-package") {
		return fmt.Errorf("E: Unable to locate package nonexistent-package")
	}
	if strings.Contains(content, "COPY nonexistent-file") {
		return fmt.Errorf("COPY failed: file not found in build context")
	}
	// Simulate successful build for valid Dockerfiles
	op.logger.Info().
		Str("image_name", op.ImageName).
		Msg("Docker build completed successfully (simulated)")
	return nil
}

// Execute runs the operation
func (op *IntegratedDockerBuildOperation) Execute(ctx context.Context) error {
	err := op.ExecuteOnce(ctx)
	if err != nil {
		op.lastError = err
	}
	return err
}

// CanRetry determines if the operation can be retried
func (op *IntegratedDockerBuildOperation) CanRetry() bool {
	// Docker builds can generally be retried
	return true
}

// GetLastError returns the last error encountered
func (op *IntegratedDockerBuildOperation) GetLastError() error {
	return op.lastError
}
