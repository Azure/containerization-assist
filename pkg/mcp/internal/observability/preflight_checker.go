package observability

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/internal/session"
	"github.com/rs/zerolog"
)

// PreFlightChecker validates system requirements before starting workflow
type PreFlightChecker struct {
	logger            zerolog.Logger
	timeout           time.Duration
	registryValidator *RegistryValidator
	systemValidator   *SystemValidator
	securityValidator *SecurityValidator
}

// NewPreFlightChecker creates a new pre-flight checker
func NewPreFlightChecker(logger zerolog.Logger) *PreFlightChecker {
	return &PreFlightChecker{
		logger:            logger,
		timeout:           10 * time.Second,
		registryValidator: NewRegistryValidator(logger),
		systemValidator:   NewSystemValidator(logger),
		securityValidator: NewSecurityValidator(logger),
	}
}

// RunStageChecks executes pre-flight checks for a specific stage
func (pfc *PreFlightChecker) RunStageChecks(ctx context.Context, stage string, state *session.SessionState) (*PreFlightResult, error) {
	checks := pfc.getChecksForStage(stage, state)
	if len(checks) == 0 {
		return &PreFlightResult{
			Passed:     true,
			Timestamp:  time.Now(),
			CanProceed: true,
		}, nil
	}

	return pfc.runChecks(ctx, checks)
}

// getChecksForStage returns checks specific to a stage
func (pfc *PreFlightChecker) getChecksForStage(stage string, state *session.SessionState) []PreFlightCheck {
	switch stage {
	case "build":
		return pfc.getBuildChecks(state)
	case "push":
		return pfc.getPushChecks(state)
	case "manifests":
		return pfc.getManifestChecks(state)
	case "deploy":
		return pfc.getDeploymentChecks(state)
	default:
		return []PreFlightCheck{}
	}
}

// getBuildChecks returns pre-flight checks for the build stage
func (pfc *PreFlightChecker) getBuildChecks(state *session.SessionState) []PreFlightCheck {
	checks := []PreFlightCheck{
		{
			Name:        "Dockerfile exists",
			Description: "Verify Dockerfile has been generated",
			Category:    "docker",
			CheckFunc: func(ctx context.Context) error {
				if state.Dockerfile.Path == "" {
					return fmt.Errorf("error")
				}
				return nil
			},
			ErrorRecovery: "Generate Dockerfile first using generate_dockerfile",
			Optional:      false,
		},
		{
			Name:          "Docker daemon running",
			Description:   "Check if Docker daemon is accessible",
			Category:      "docker",
			CheckFunc:     pfc.systemValidator.CheckDockerDaemon,
			ErrorRecovery: "Start Docker Desktop or Docker daemon",
			Optional:      false,
		},
		{
			Name:          "Sufficient disk space",
			Description:   "Check if there's enough disk space for build",
			Category:      "system",
			CheckFunc:     pfc.systemValidator.CheckDiskSpace,
			ErrorRecovery: "Free up disk space (need at least 2GB)",
			Optional:      true,
		},
	}

	if state.Dockerfile.Content != "" && state.Dockerfile.Path != "" {
		checks = append(checks, PreFlightCheck{
			Name:        "Dockerfile validation",
			Description: "Ensure Dockerfile exists and is accessible",
			Category:    "docker",
			CheckFunc: func(ctx context.Context) error {
				if state.Dockerfile.Path == "" {
					return fmt.Errorf("error")
				}
				return nil
			},
			ErrorRecovery: "Regenerate Dockerfile before building",
			Optional:      false,
		})
	}

	return checks
}

// getPushChecks returns pre-flight checks for the push stage
func (pfc *PreFlightChecker) getPushChecks(state *session.SessionState) []PreFlightCheck {
	checks := []PreFlightCheck{
		{
			Name:        "Image built",
			Description: "Verify Docker image has been built",
			Category:    "docker",
			CheckFunc: func(ctx context.Context) error {
				if !state.Dockerfile.Built || state.ImageRef.String() == "" {
					return fmt.Errorf("error")
				}
				return nil
			},
			ErrorRecovery: "Build the Docker image first",
			Optional:      false,
		},
		{
			Name:        "Registry connectivity",
			Description: "Check if registry is accessible",
			Category:    "registry",
			CheckFunc: func(ctx context.Context) error {
				registry := extractRegistry(state.ImageRef.String())
				if registry == "" {
					return fmt.Errorf("error")
				}
				return pfc.registryValidator.CheckRegistryConnectivity(ctx, registry)
			},
			ErrorRecovery: "Ensure registry URL is correct and you're logged in",
			Optional:      false,
		},
		{
			Name:          "Registry authentication",
			Description:   "Verify registry authentication",
			Category:      "registry",
			CheckFunc:     pfc.registryValidator.CheckRegistryAuth,
			ErrorRecovery: "Run 'docker login' or configure registry credentials",
			Optional:      false,
		},
	}

	if state.SecurityScan != nil {
		checks = append(checks, pfc.securityValidator.GetSecurityCheck(state))
	}

	return checks
}

// getManifestChecks returns pre-flight checks for manifest generation
func (pfc *PreFlightChecker) getManifestChecks(state *session.SessionState) []PreFlightCheck {
	return []PreFlightCheck{
		{
			Name:        "Image reference available",
			Description: "Verify image has been built or pushed",
			Category:    "docker",
			CheckFunc: func(ctx context.Context) error {
				if state.ImageRef.String() == "" {
					return fmt.Errorf("error")
				}
				return nil
			},
			ErrorRecovery: "Build and optionally push Docker image first",
			Optional:      false,
		},
	}
}

// getDeploymentChecks returns pre-flight checks for deployment
func (pfc *PreFlightChecker) getDeploymentChecks(state *session.SessionState) []PreFlightCheck {
	return []PreFlightCheck{
		{
			Name:          "Kubernetes connectivity",
			Description:   "Check if kubectl can connect to cluster",
			Category:      "kubernetes",
			CheckFunc:     pfc.systemValidator.CheckKubernetesConnectivity,
			ErrorRecovery: "Configure kubectl to connect to your cluster",
			Optional:      false,
		},
		{
			Name:        "Manifests generated",
			Description: "Verify Kubernetes manifests exist",
			Category:    "kubernetes",
			CheckFunc: func(ctx context.Context) error {
				if len(state.K8sManifests) == 0 {
					return fmt.Errorf("error")
				}
				return nil
			},
			ErrorRecovery: "Generate Kubernetes manifests first",
			Optional:      false,
		},
	}
}

// RunChecks executes all pre-flight checks
func (pfc *PreFlightChecker) RunChecks(ctx context.Context) (*PreFlightResult, error) {
	checks := pfc.getChecks()
	return pfc.runChecks(ctx, checks)
}

// runChecks executes a list of checks and returns results
func (pfc *PreFlightChecker) runChecks(ctx context.Context, checks []PreFlightCheck) (*PreFlightResult, error) {
	start := time.Now()
	results := make([]CheckResult, 0, len(checks))
	suggestions := make(map[string]string)

	allPassed := true
	canProceed := true

	for _, check := range checks {
		checkStart := time.Now()

		checkCtx, cancel := context.WithTimeout(ctx, pfc.timeout)
		defer cancel()

		result := CheckResult{
			Name:     check.Name,
			Category: check.Category,
			Status:   CheckStatusPass,
		}

		err := check.CheckFunc(checkCtx)
		result.Duration = time.Since(checkStart)

		if err != nil {
			if check.Optional {
				result.Status = CheckStatusWarning
				result.Message = fmt.Sprintf("Optional check failed: %v", err)
			} else {
				result.Status = CheckStatusFail
				result.Message = fmt.Sprintf("Check failed: %v", err)
				result.Error = err.Error()
				allPassed = false
				canProceed = false
			}
			result.RecoveryAction = check.ErrorRecovery
			suggestions[check.Name] = check.ErrorRecovery
		} else {
			result.Message = "Check passed"
		}

		results = append(results, result)

		pfc.logger.Info().
			Str("check", check.Name).
			Str("status", string(result.Status)).
			Dur("duration", result.Duration).
			Msg("Pre-flight check completed")
	}

	return &PreFlightResult{
		Passed:      allPassed,
		Timestamp:   start,
		Duration:    time.Since(start),
		Checks:      results,
		Suggestions: suggestions,
		CanProceed:  canProceed,
	}, nil
}

// getChecks returns all pre-flight checks
func (pfc *PreFlightChecker) getChecks() []PreFlightCheck {
	return []PreFlightCheck{
		{
			Name:          "docker_daemon",
			Description:   "Check if Docker daemon is running",
			Category:      "docker",
			CheckFunc:     pfc.systemValidator.CheckDockerDaemon,
			ErrorRecovery: "Please start Docker Desktop or run: sudo systemctl start docker",
			Optional:      false,
		},
		{
			Name:          "docker_disk_space",
			Description:   "Check available disk space for Docker",
			Category:      "system",
			CheckFunc:     pfc.systemValidator.CheckDockerDiskSpace,
			ErrorRecovery: "Please free up at least 5GB of disk space for container builds",
			Optional:      false,
		},
		{
			Name:          "kubernetes_context",
			Description:   "Check if kubectl is configured with a valid context",
			Category:      "kubernetes",
			CheckFunc:     pfc.systemValidator.CheckKubernetesContext,
			ErrorRecovery: "Please configure kubectl with: kubectl config use-context <context-name>",
			Optional:      true,
		},
		{
			Name:          "kubernetes_connectivity",
			Description:   "Check connectivity to Kubernetes cluster",
			Category:      "kubernetes",
			CheckFunc:     pfc.systemValidator.CheckKubernetesConnectivity,
			ErrorRecovery: "Please ensure your Kubernetes cluster is accessible",
			Optional:      true,
		},
		{
			Name:          "registry_auth",
			Description:   "Check Docker registry authentication",
			Category:      "registry",
			CheckFunc:     pfc.registryValidator.CheckRegistryAuth,
			ErrorRecovery: "Please authenticate with: docker login <registry>",
			Optional:      true,
		},
		{
			Name:          "required_tools",
			Description:   "Check for required CLI tools",
			Category:      "system",
			CheckFunc:     pfc.systemValidator.CheckRequiredTools,
			ErrorRecovery: "Please install missing tools",
			Optional:      false,
		},
		{
			Name:          "git_installed",
			Description:   "Check if git is installed for repository operations",
			Category:      "system",
			CheckFunc:     pfc.systemValidator.CheckGitInstalled,
			ErrorRecovery: "Please install git: https://git-scm.com/downloads",
			Optional:      true,
		},
	}
}

// GetCheckByName returns a specific check by name
func (pfc *PreFlightChecker) GetCheckByName(ctx context.Context, name string) (*PreFlightCheck, error) {
	for _, check := range pfc.getChecks() {
		if check.Name == name {
			return &check, nil
		}
	}
	return nil, fmt.Errorf("failed to get helper: %s", name)
}

// RunSingleCheck runs a specific check
func (pfc *PreFlightChecker) RunSingleCheck(ctx context.Context, checkName string) (*CheckResult, error) {
	check, err := pfc.GetCheckByName(ctx, checkName)
	if err != nil {
		return nil, err
	}

	start := time.Now()
	checkCtx, cancel := context.WithTimeout(ctx, pfc.timeout)
	defer cancel()

	result := &CheckResult{
		Name:     check.Name,
		Category: check.Category,
		Status:   CheckStatusPass,
	}

	err = check.CheckFunc(checkCtx)
	result.Duration = time.Since(start)

	if err != nil {
		result.Status = CheckStatusFail
		result.Message = fmt.Sprintf("Check failed: %v", err)
		result.Error = err.Error()
		result.RecoveryAction = check.ErrorRecovery
	} else {
		result.Message = "Check passed"
	}

	return result, nil
}

// FormatResults formats the pre-flight results for display
func (pfc *PreFlightChecker) FormatResults(ctx context.Context, results *PreFlightResult) string {
	var sb strings.Builder

	sb.WriteString("Pre-flight Check Results:\n")
	sb.WriteString(fmt.Sprintf("Overall Status: %s\n", pfc.getOverallStatus(results)))
	sb.WriteString(fmt.Sprintf("Duration: %v\n\n", results.Duration.Round(time.Millisecond)))

	byCategory := make(map[string][]CheckResult)
	for _, check := range results.Checks {
		byCategory[check.Category] = append(byCategory[check.Category], check)
	}

	for category, checks := range byCategory {
		sb.WriteString(fmt.Sprintf("%s Checks:\n", strings.Title(category)))
		for _, check := range checks {
			icon := pfc.getStatusIcon(check.Status)
			sb.WriteString(fmt.Sprintf("  %s %s: %s\n", icon, check.Name, check.Message))
			if check.RecoveryAction != "" && check.Status == CheckStatusFail {
				sb.WriteString(fmt.Sprintf("     → %s\n", check.RecoveryAction))
			}
		}
		sb.WriteString("\n")
	}

	if !results.CanProceed {
		sb.WriteString("⚠️  Cannot proceed until required checks pass.\n")
	}

	return sb.String()
}

// ValidateMultipleRegistries validates authentication and connectivity for multiple registries
func (pfc *PreFlightChecker) ValidateMultipleRegistries(ctx context.Context, registries []string) (*MultiRegistryValidationResult, error) {
	return pfc.registryValidator.ValidateMultipleRegistries(ctx, registries)
}

// getOverallStatus returns the overall status message
func (pfc *PreFlightChecker) getOverallStatus(results *PreFlightResult) string {
	if results.Passed {
		return "✅ All checks passed"
	}
	if results.CanProceed {
		return "⚠️  Some optional checks failed"
	}
	return "❌ Required checks failed"
}

// getStatusIcon returns the icon for a check status
func (pfc *PreFlightChecker) getStatusIcon(status CheckStatus) string {
	switch status {
	case CheckStatusPass:
		return "✅"
	case CheckStatusFail:
		return "❌"
	case CheckStatusWarning:
		return "⚠️"
	case CheckStatusSkipped:
		return "⏭️"
	default:
		return "?"
	}
}

// extractRegistry extracts the registry hostname from an image reference
func extractRegistry(imageRef string) string {
	if imageRef == "" {
		return ""
	}

	if !strings.Contains(imageRef, "/") || (!strings.Contains(imageRef, ".") && !strings.Contains(imageRef, ":")) {
		return "docker.io"
	}

	parts := strings.Split(imageRef, "/")
	if len(parts) > 0 {
		firstPart := parts[0]
		if strings.Contains(firstPart, ".") || strings.Contains(firstPart, ":") {
			return firstPart
		}
	}

	return "docker.io"
}
