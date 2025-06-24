package interfaces

import (
	"context"
)

// ProgressStage represents a stage in a multi-step operation
type ProgressStage struct {
	Name        string  // Human-readable stage name
	Weight      float64 // Relative weight (0.0-1.0) of this stage in overall progress
	Description string  // Optional detailed description
}

// ProgressReporter provides stage-aware progress reporting
type ProgressReporter interface {
	// ReportStage reports progress for the current stage
	ReportStage(stageProgress float64, message string)

	// NextStage advances to the next stage and reports its start
	NextStage(message string)

	// SetStage explicitly sets the current stage index
	SetStage(stageIndex int, message string)

	// ReportOverall reports overall progress directly (bypassing stage calculation)
	ReportOverall(progress float64, message string)

	// GetCurrentStage returns the current stage information
	GetCurrentStage() (int, ProgressStage)
}

// ProgressTracker provides centralized progress reporting for tools
type ProgressTracker interface {
	// RunWithProgress executes an operation with standardized progress reporting
	RunWithProgress(
		ctx context.Context,
		operation string,
		stages []ProgressStage,
		fn func(ctx context.Context, reporter ProgressReporter) error,
	) error
}

// Standard stage definitions for consistent progress reporting across tools

// StandardBuildStages provides common stages for build operations
func StandardBuildStages() []ProgressStage {
	return []ProgressStage{
		{Name: "Initialize", Weight: 0.10, Description: "Loading session and validating inputs"},
		{Name: "Analyze", Weight: 0.20, Description: "Analyzing build context and dependencies"},
		{Name: "Build", Weight: 0.50, Description: "Building Docker image"},
		{Name: "Verify", Weight: 0.15, Description: "Running post-build verification"},
		{Name: "Finalize", Weight: 0.05, Description: "Cleaning up and saving results"},
	}
}

// StandardDeployStages provides common stages for deployment operations
func StandardDeployStages() []ProgressStage {
	return []ProgressStage{
		{Name: "Initialize", Weight: 0.10, Description: "Loading session and validating inputs"},
		{Name: "Generate", Weight: 0.30, Description: "Generating Kubernetes manifests"},
		{Name: "Deploy", Weight: 0.40, Description: "Deploying to cluster"},
		{Name: "Verify", Weight: 0.15, Description: "Verifying deployment health"},
		{Name: "Finalize", Weight: 0.05, Description: "Saving deployment status"},
	}
}

// StandardScanStages provides common stages for security scanning operations
func StandardScanStages() []ProgressStage {
	return []ProgressStage{
		{Name: "Initialize", Weight: 0.10, Description: "Preparing scan environment"},
		{Name: "Scan", Weight: 0.60, Description: "Running security analysis"},
		{Name: "Analyze", Weight: 0.20, Description: "Processing scan results"},
		{Name: "Report", Weight: 0.10, Description: "Generating security report"},
	}
}

// StandardAnalysisStages provides common stages for repository analysis
func StandardAnalysisStages() []ProgressStage {
	return []ProgressStage{
		{Name: "Initialize", Weight: 0.10, Description: "Setting up analysis environment"},
		{Name: "Discover", Weight: 0.30, Description: "Discovering project structure"},
		{Name: "Analyze", Weight: 0.40, Description: "Analyzing dependencies and frameworks"},
		{Name: "Generate", Weight: 0.15, Description: "Generating recommendations"},
		{Name: "Finalize", Weight: 0.05, Description: "Saving analysis results"},
	}
}

// StandardPushStages provides common stages for registry push operations
func StandardPushStages() []ProgressStage {
	return []ProgressStage{
		{Name: "Initialize", Weight: 0.10, Description: "Loading session and validating inputs"},
		{Name: "Authenticate", Weight: 0.15, Description: "Authenticating with registry"},
		{Name: "Push", Weight: 0.60, Description: "Pushing Docker image layers"},
		{Name: "Verify", Weight: 0.10, Description: "Verifying push results"},
		{Name: "Finalize", Weight: 0.05, Description: "Updating session state"},
	}
}

// StandardPullStages provides common stages for registry pull operations
func StandardPullStages() []ProgressStage {
	return []ProgressStage{
		{Name: "Initialize", Weight: 0.10, Description: "Loading session and validating inputs"},
		{Name: "Authenticate", Weight: 0.15, Description: "Authenticating with registry"},
		{Name: "Pull", Weight: 0.60, Description: "Pulling Docker image layers"},
		{Name: "Verify", Weight: 0.10, Description: "Verifying image integrity"},
		{Name: "Finalize", Weight: 0.05, Description: "Updating session state"},
	}
}

// StandardTagStages provides common stages for Docker tag operations
func StandardTagStages() []ProgressStage {
	return []ProgressStage{
		{Name: "Initialize", Weight: 0.20, Description: "Loading session and validating inputs"},
		{Name: "Tag", Weight: 0.60, Description: "Creating Docker image tag"},
		{Name: "Verify", Weight: 0.15, Description: "Verifying tag creation"},
		{Name: "Finalize", Weight: 0.05, Description: "Updating session state"},
	}
}

// StandardValidationStages provides common stages for validation operations
func StandardValidationStages() []ProgressStage {
	return []ProgressStage{
		{Name: "Initialize", Weight: 0.10, Description: "Loading session and preparing validation"},
		{Name: "Parse", Weight: 0.20, Description: "Parsing and loading files"},
		{Name: "Validate", Weight: 0.50, Description: "Running validation checks"},
		{Name: "Report", Weight: 0.15, Description: "Generating validation report"},
		{Name: "Finalize", Weight: 0.05, Description: "Saving validation results"},
	}
}

// StandardHealthStages provides common stages for health check operations
func StandardHealthStages() []ProgressStage {
	return []ProgressStage{
		{Name: "Initialize", Weight: 0.10, Description: "Preparing health checks"},
		{Name: "Connect", Weight: 0.20, Description: "Connecting to services"},
		{Name: "Check", Weight: 0.50, Description: "Running health checks"},
		{Name: "Analyze", Weight: 0.15, Description: "Analyzing health status"},
		{Name: "Report", Weight: 0.05, Description: "Generating health report"},
	}
}

// StandardGenerateStages provides common stages for generation operations
func StandardGenerateStages() []ProgressStage {
	return []ProgressStage{
		{Name: "Initialize", Weight: 0.10, Description: "Loading session and analyzing requirements"},
		{Name: "Template", Weight: 0.30, Description: "Selecting and preparing templates"},
		{Name: "Generate", Weight: 0.40, Description: "Generating files"},
		{Name: "Validate", Weight: 0.15, Description: "Validating generated content"},
		{Name: "Finalize", Weight: 0.05, Description: "Saving generated files"},
	}
}

// StandardSecurityScanStages provides common stages for image security scanning
func StandardSecurityScanStages() []ProgressStage {
	return []ProgressStage{
		{Name: "Initialize", Weight: 0.10, Description: "Preparing security scan environment"},
		{Name: "Pull", Weight: 0.20, Description: "Pulling image if needed"},
		{Name: "Scan", Weight: 0.50, Description: "Running security scanners"},
		{Name: "Analyze", Weight: 0.15, Description: "Analyzing vulnerabilities"},
		{Name: "Report", Weight: 0.05, Description: "Generating security report"},
	}
}

// StandardDockerfileValidationStages provides common stages for Dockerfile validation
func StandardDockerfileValidationStages() []ProgressStage {
	return []ProgressStage{
		{Name: "Initialize", Weight: 0.10, Description: "Loading session and validation rules"},
		{Name: "Parse", Weight: 0.20, Description: "Parsing Dockerfile"},
		{Name: "Validate Syntax", Weight: 0.20, Description: "Checking syntax and structure"},
		{Name: "Validate Security", Weight: 0.20, Description: "Running security checks"},
		{Name: "Validate Best Practices", Weight: 0.20, Description: "Checking best practices"},
		{Name: "Report", Weight: 0.10, Description: "Generating validation report"},
	}
}
