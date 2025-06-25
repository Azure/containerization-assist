package orchestration

import (
	"time"
)

// Helper function to convert time.Duration to pointer
func timePtr(d time.Duration) *time.Duration {
	return &d
}

// GetExampleWorkflows returns a collection of example workflow specifications
func GetExampleWorkflows() map[string]*WorkflowSpec {
	return map[string]*WorkflowSpec{
		"containerization-pipeline": getContainerizationPipeline(),
		"security-focused-pipeline": getSecurityFocusedPipeline(),
		"development-workflow":      getDevelopmentWorkflow(),
		"production-deployment":     getProductionDeployment(),
		"ci-cd-pipeline":            getCICDPipeline(),
	}
}

// getContainerizationPipeline returns a standard containerization workflow
func getContainerizationPipeline() *WorkflowSpec {
	return &WorkflowSpec{
		APIVersion: "orchestration/v1",
		Kind:       "Workflow",
		Metadata: WorkflowMetadata{
			Name:        "containerization-pipeline",
			Description: "Complete containerization pipeline from source code to deployed application",
			Version:     "1.0.0",
			Labels: map[string]string{
				"type":     "containerization",
				"category": "standard",
			},
		},
		Spec: WorkflowDefinition{
			Stages: []WorkflowStage{
				{
					Name:      "analysis",
					Tools:     []string{"analyze_repository_atomic"},
					DependsOn: []string{},
					Parallel:  false,
					Conditions: []StageCondition{
						{Key: "repo_url", Operator: "required"},
					},
					Timeout: timePtr(10 * time.Minute),
				},
				{
					Name:      "dockerfile-generation",
					Tools:     []string{"generate_dockerfile"},
					DependsOn: []string{"analysis"},
					Parallel:  false,
					Conditions: []StageCondition{
						{Key: "dockerfile_exists", Operator: "not_exists"},
					},
				},
				{
					Name:      "validation",
					Tools:     []string{"validate_dockerfile_atomic", "scan_secrets_atomic"},
					DependsOn: []string{"dockerfile-generation"},
					Parallel:  true,
					Timeout:   timePtr(5 * time.Minute),
				},
				{
					Name:      "build",
					Tools:     []string{"build_image_atomic"},
					DependsOn: []string{"validation"},
					Parallel:  false,
					RetryPolicy: &RetryPolicyExecution{
						MaxAttempts:  3,
						BackoffMode:  "exponential",
						InitialDelay: 30 * time.Second,
						MaxDelay:     5 * time.Minute,
						Multiplier:   2.0,
					},
				},
				{
					Name:      "security-scan",
					Tools:     []string{"scan_image_security_atomic"},
					DependsOn: []string{"build"},
					Parallel:  false,
					Conditions: []StageCondition{
						{Key: "security_scan_enabled", Operator: "equals", Value: true},
					},
				},
				{
					Name:      "deployment-prep",
					Tools:     []string{"push_image_atomic", "generate_manifests_atomic"},
					DependsOn: []string{"security-scan"},
					Parallel:  true,
				},
				{
					Name:      "deployment",
					Tools:     []string{"deploy_kubernetes_atomic"},
					DependsOn: []string{"deployment-prep"},
					Parallel:  false,
					Timeout:   timePtr(15 * time.Minute),
				},
				{
					Name:      "validation",
					Tools:     []string{"check_health_atomic"},
					DependsOn: []string{"deployment"},
					Parallel:  false,
					Timeout:   timePtr(5 * time.Minute),
				},
			},
			Variables: map[string]interface{}{
				"registry":              "myregistry.azurecr.io",
				"namespace":             "default",
				"security_scan_enabled": "true",
			},
			ErrorPolicy: &ErrorPolicy{
				Mode:        "fail_fast",
				MaxFailures: 3,
			},
			Timeout: 60 * time.Minute,
		},
	}
}

// getSecurityFocusedPipeline returns a security-focused workflow
func getSecurityFocusedPipeline() *WorkflowSpec {
	return &WorkflowSpec{
		APIVersion: "orchestration/v1",
		Kind:       "Workflow",
		Metadata: WorkflowMetadata{
			Name:        "security-focused-pipeline",
			Description: "Enhanced security pipeline with comprehensive scanning and validation",
			Version:     "1.0.0",
			Labels: map[string]string{
				"type":     "security",
				"category": "enhanced",
			},
		},
		Spec: WorkflowDefinition{
			Stages: []WorkflowStage{
				{
					Name:      "analysis",
					Tools:     []string{"analyze_repository_atomic"},
					DependsOn: []string{},
					Parallel:  false,
				},
				{
					Name:      "security-validation",
					Tools:     []string{"scan_secrets_atomic", "validate_dockerfile_atomic"},
					DependsOn: []string{"analysis"},
					Parallel:  true,
					OnFailure: &FailureAction{
						Action: "fail",
					},
				},
				{
					Name:      "build",
					Tools:     []string{"build_image_atomic"},
					DependsOn: []string{"security-validation"},
					Parallel:  false,
				},
				{
					Name:      "comprehensive-security-scan",
					Tools:     []string{"scan_image_security_atomic"},
					DependsOn: []string{"build"},
					Parallel:  false,
					Variables: map[string]interface{}{
						"scan_mode":        "comprehensive",
						"fail_on_critical": "true",
					},
					OnFailure: &FailureAction{
						Action: "fail",
					},
				},
				{
					Name:      "tag-and-push",
					Tools:     []string{"tag_image_atomic", "push_image_atomic"},
					DependsOn: []string{"comprehensive-security-scan"},
					Parallel:  false,
				},
				{
					Name:      "secure-deployment",
					Tools:     []string{"generate_manifests_atomic", "deploy_kubernetes_atomic"},
					DependsOn: []string{"tag-and-push"},
					Parallel:  false,
					Variables: map[string]interface{}{
						"gitops_ready":    "true",
						"secret_handling": "auto",
					},
				},
			},
			ErrorPolicy: &ErrorPolicy{
				Mode:        "fail_fast",
				MaxFailures: 1,
			},
		},
	}
}

// getDevelopmentWorkflow returns a development-friendly workflow
func getDevelopmentWorkflow() *WorkflowSpec {
	return &WorkflowSpec{
		APIVersion: "orchestration/v1",
		Kind:       "Workflow",
		Metadata: WorkflowMetadata{
			Name:        "development-workflow",
			Description: "Fast development workflow with minimal security checks",
			Version:     "1.0.0",
			Labels: map[string]string{
				"type":        "development",
				"environment": "dev",
			},
		},
		Spec: WorkflowDefinition{
			Stages: []WorkflowStage{
				{
					Name:      "quick-analysis",
					Tools:     []string{"analyze_repository_atomic"},
					DependsOn: []string{},
					Parallel:  false,
					Timeout:   timePtr(2 * time.Minute),
				},
				{
					Name:      "build-and-test",
					Tools:     []string{"build_image_atomic"},
					DependsOn: []string{"quick-analysis"},
					Parallel:  false,
					Variables: map[string]interface{}{
						"quick_build":       "true",
						"skip_optimization": "true",
					},
				},
				{
					Name:      "local-deployment",
					Tools:     []string{"generate_manifests_atomic", "deploy_kubernetes_atomic"},
					DependsOn: []string{"build-and-test"},
					Parallel:  false,
					Variables: map[string]interface{}{
						"namespace": "development",
						"replicas":  "1",
					},
				},
			},
			ErrorPolicy: &ErrorPolicy{
				Mode:        "continue",
				MaxFailures: 5,
			},
			Timeout: 15 * time.Minute,
		},
	}
}

// getProductionDeployment returns a production deployment workflow
func getProductionDeployment() *WorkflowSpec {
	return &WorkflowSpec{
		APIVersion: "orchestration/v1",
		Kind:       "Workflow",
		Metadata: WorkflowMetadata{
			Name:        "production-deployment",
			Description: "Production-ready deployment with comprehensive validation",
			Version:     "1.0.0",
			Labels: map[string]string{
				"type":        "deployment",
				"environment": "production",
			},
		},
		Spec: WorkflowDefinition{
			Stages: []WorkflowStage{
				{
					Name:      "pull-image",
					Tools:     []string{"pull_image_atomic"},
					DependsOn: []string{},
					Parallel:  false,
					Conditions: []StageCondition{
						{Key: "image_ref", Operator: "required"},
					},
				},
				{
					Name:      "production-security-scan",
					Tools:     []string{"scan_image_security_atomic"},
					DependsOn: []string{"pull-image"},
					Parallel:  false,
					Variables: map[string]interface{}{
						"scan_mode":    "production",
						"fail_on_high": "true",
					},
					OnFailure: &FailureAction{
						Action: "fail",
					},
				},
				{
					Name:      "production-tag",
					Tools:     []string{"tag_image_atomic"},
					DependsOn: []string{"production-security-scan"},
					Parallel:  false,
					Variables: map[string]interface{}{
						"tag_suffix":    "prod",
						"add_timestamp": "true",
					},
				},
				{
					Name:      "production-push",
					Tools:     []string{"push_image_atomic"},
					DependsOn: []string{"production-tag"},
					Parallel:  false,
				},
				{
					Name:      "production-manifests",
					Tools:     []string{"generate_manifests_atomic"},
					DependsOn: []string{"production-push"},
					Parallel:  false,
					Variables: map[string]interface{}{
						"namespace":       "production",
						"replicas":        "3",
						"resource_limits": "true",
						"gitops_ready":    "true",
					},
				},
				{
					Name:      "production-deployment",
					Tools:     []string{"deploy_kubernetes_atomic"},
					DependsOn: []string{"production-manifests"},
					Parallel:  false,
					Timeout:   timePtr(30 * time.Minute),
					Variables: map[string]interface{}{
						"deployment_strategy": "rolling",
						"max_unavailable":     "25%",
					},
				},
				{
					Name:      "production-validation",
					Tools:     []string{"check_health_atomic"},
					DependsOn: []string{"production-deployment"},
					Parallel:  false,
					Timeout:   timePtr(10 * time.Minute),
					RetryPolicy: &RetryPolicyExecution{
						MaxAttempts:  5,
						BackoffMode:  "linear",
						InitialDelay: 30 * time.Second,
						MaxDelay:     2 * time.Minute,
					},
				},
			},
			ErrorPolicy: &ErrorPolicy{
				Mode:        "fail_fast",
				MaxFailures: 1,
			},
			Timeout: 90 * time.Minute,
		},
	}
}

// getCICDPipeline returns a comprehensive CI/CD pipeline workflow
func getCICDPipeline() *WorkflowSpec {
	return &WorkflowSpec{
		APIVersion: "orchestration/v1",
		Kind:       "Workflow",
		Metadata: WorkflowMetadata{
			Name:        "ci-cd-pipeline",
			Description: "Complete CI/CD pipeline with testing, security, and deployment",
			Version:     "1.0.0",
			Labels: map[string]string{
				"type":     "cicd",
				"category": "complete",
			},
		},
		Spec: WorkflowDefinition{
			Stages: []WorkflowStage{
				{
					Name:      "source-analysis",
					Tools:     []string{"analyze_repository_atomic", "scan_secrets_atomic"},
					DependsOn: []string{},
					Parallel:  true,
				},
				{
					Name:      "dockerfile-validation",
					Tools:     []string{"validate_dockerfile_atomic"},
					DependsOn: []string{"source-analysis"},
					Parallel:  false,
					Conditions: []StageCondition{
						{Key: "dockerfile_exists", Operator: "exists"},
					},
				},
				{
					Name:      "build-stage",
					Tools:     []string{"build_image_atomic"},
					DependsOn: []string{"dockerfile-validation"},
					Parallel:  false,
					RetryPolicy: &RetryPolicyExecution{
						MaxAttempts:  2,
						BackoffMode:  "fixed",
						InitialDelay: 1 * time.Minute,
					},
				},
				{
					Name:      "quality-assurance",
					Tools:     []string{"scan_image_security_atomic"},
					DependsOn: []string{"build-stage"},
					Parallel:  false,
					Variables: map[string]interface{}{
						"qa_mode": "thorough",
					},
				},
				{
					Name:      "staging-deployment",
					Tools:     []string{"tag_image_atomic", "push_image_atomic", "generate_manifests_atomic"},
					DependsOn: []string{"quality-assurance"},
					Parallel:  false,
					Variables: map[string]interface{}{
						"environment": "staging",
						"tag_suffix":  "staging",
					},
				},
				{
					Name:      "staging-deploy",
					Tools:     []string{"deploy_kubernetes_atomic"},
					DependsOn: []string{"staging-deployment"},
					Parallel:  false,
					Variables: map[string]interface{}{
						"namespace": "staging",
					},
				},
				{
					Name:      "staging-tests",
					Tools:     []string{"check_health_atomic"},
					DependsOn: []string{"staging-deploy"},
					Parallel:  false,
					Timeout:   timePtr(15 * time.Minute),
				},
				{
					Name:      "production-promotion",
					Tools:     []string{"tag_image_atomic", "push_image_atomic"},
					DependsOn: []string{"staging-tests"},
					Parallel:  false,
					Conditions: []StageCondition{
						{Key: "approve_production", Operator: "equals", Value: true},
					},
					Variables: map[string]interface{}{
						"tag_suffix": "production",
						"promote":    "true",
					},
				},
			},
			Variables: map[string]interface{}{
				"registry":             "registry.company.com",
				"approve_production":   "false",
				"notification_webhook": "${NOTIFICATION_URL}",
			},
			ErrorPolicy: &ErrorPolicy{
				Mode:        "fail_fast",
				MaxFailures: 2,
				Routing: []ErrorRouting{
					{
						FromTool:   "build_image_atomic",
						ErrorType:  "build_error",
						Action:     "redirect",
						RedirectTo: "dockerfile-validation",
					},
					{
						FromTool:  "scan_image_security_atomic",
						ErrorType: "security_issues",
						Action:    "fail",
					},
				},
			},
			Timeout: 120 * time.Minute,
		},
	}
}

// Helper function to create duration pointers
func durationPtr(d time.Duration) *time.Duration {
	return &d
}

// GetWorkflowByName returns a workflow specification by name
func GetWorkflowByName(name string) (*WorkflowSpec, bool) {
	workflows := GetExampleWorkflows()
	workflow, exists := workflows[name]
	return workflow, exists
}

// ListAvailableWorkflows returns a list of available workflow names and descriptions
func ListAvailableWorkflows() []WorkflowInfo {
	workflows := GetExampleWorkflows()
	var info []WorkflowInfo

	for name, spec := range workflows {
		info = append(info, WorkflowInfo{
			Name:        name,
			DisplayName: spec.Metadata.Name,
			Description: spec.Metadata.Description,
			Version:     spec.Metadata.Version,
			Labels:      spec.Metadata.Labels,
			StageCount:  len(spec.Spec.Stages),
			HasTimeout:  spec.Spec.Timeout > 0,
		})
	}

	return info
}

// WorkflowInfo contains summary information about a workflow
type WorkflowInfo struct {
	Name        string            `json:"name"`
	DisplayName string            `json:"display_name"`
	Description string            `json:"description"`
	Version     string            `json:"version"`
	Labels      map[string]string `json:"labels"`
	StageCount  int               `json:"stage_count"`
	HasTimeout  bool              `json:"has_timeout"`
}
