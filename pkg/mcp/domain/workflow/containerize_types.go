package workflow

// ContainerizeAndDeployArgs represents the input arguments for the complete containerization workflow.
// This structure defines all the parameters needed to execute the full 10-step containerization
// and deployment process, from repository analysis through Kubernetes deployment.
//
// The workflow process includes:
//  1. Repository analysis and technology detection
//  2. Dockerfile generation
//  3. Container image building
//  4. Security vulnerability scanning (optional)
//  5. Image tagging and versioning
//  6. Container registry push
//  7. Kubernetes manifest generation
//  8. Cluster setup and validation
//  9. Application deployment
//  10. Health verification and validation
type ContainerizeAndDeployArgs struct {
	// RepoURL is the Git repository URL to containerize (required)
	// Supports HTTPS and SSH Git URLs
	RepoURL string `json:"repo_url"`

	// Branch specifies the Git branch to use (default: main/master)
	Branch string `json:"branch,omitempty"`

	// Scan enables security vulnerability scanning with Trivy/Grype
	// When true, the workflow will scan the built image for vulnerabilities
	Scan bool `json:"scan,omitempty"`

	// Deploy controls whether to perform Kubernetes deployment
	// Uses pointer to distinguish between unset (auto-decide) and explicitly false
	// nil = auto-decide, false = skip deployment, true = force deployment
	Deploy *bool `json:"deploy,omitempty"`

	// TestMode skips actual Docker and Kubernetes operations for testing
	// When true, the workflow simulates operations without external dependencies
	TestMode bool `json:"test_mode,omitempty"`

	// StrictMode controls error handling behavior for non-critical operations
	// When true, warnings (e.g., scan failures, health check failures) will fail the workflow
	// When false (default), warnings are logged but the workflow continues
	StrictMode bool `json:"strict_mode,omitempty"`
}

// ContainerizeAndDeployResult represents the complete workflow output and execution summary.
// This structure contains all information about the workflow execution, including
// success/failure status, generated artifacts, and detailed step-by-step execution history.
//
// The result provides comprehensive information for:
//   - Deployment verification (endpoint, image reference)
//   - Security analysis (scan reports)
//   - Debugging and troubleshooting (step details, errors)
//   - Monitoring and observability (execution metrics)
type ContainerizeAndDeployResult struct {
	// Success indicates overall workflow completion status
	// True only if all required steps completed successfully
	Success bool `json:"success"`

	// Endpoint is the deployed application's access URL
	// Only populated if deployment was successful and service is accessible
	Endpoint string `json:"endpoint,omitempty"`

	// ImageRef is the full container image reference with registry and tag
	// Format: registry.com/namespace/image:tag or digest
	ImageRef string `json:"image_ref,omitempty"`

	// Namespace is the Kubernetes namespace where the application was deployed
	Namespace string `json:"k8s_namespace,omitempty"`

	// ScanReport contains security vulnerability scan results
	// Only populated if Scan was enabled and scanning completed
	ScanReport map[string]interface{} `json:"scan_report,omitempty"`

	// Steps contains detailed execution information for each workflow step
	// Provides full audit trail of the workflow execution
	Steps []WorkflowStep `json:"steps"`

	// Error contains the overall error message if the workflow failed
	// Empty if Success is true
	Error string `json:"error,omitempty"`
}

// WorkflowStep represents the execution details of a single workflow step.
// Each step in the 10-step containerization process is tracked with comprehensive
// execution metadata for monitoring, debugging, and audit purposes.
//
// Step statuses:
//   - "pending": Step not yet started
//   - "running": Step currently executing
//   - "completed": Step finished successfully
//   - "failed": Step failed (may be retried)
//   - "skipped": Step was skipped due to conditions
type WorkflowStep struct {
	// Name is the step identifier (e.g., "analyze", "build", "deploy")
	Name string `json:"name"`

	// Status indicates the current execution state of the step
	Status string `json:"status"`

	// Duration is the total execution time for the step
	// Format: "1m30s", "45.2s", etc.
	Duration string `json:"duration"`

	// Error contains the error message if the step failed
	// Empty for successful steps
	Error string `json:"error,omitempty"`

	// Retries is the number of retry attempts made for this step
	// 0 indicates successful execution on first attempt
	Retries int `json:"retries,omitempty"`

	// Progress indicates the step's position in the overall workflow
	// Format: "3/10", "7/10", etc.
	Progress string `json:"progress,omitempty"`

	// Message provides human-readable status or result information
	// Contains step-specific details about execution or outcomes
	Message string `json:"message,omitempty"`
}
