package observability

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/Azure/container-copilot/pkg/mcp/internal/registry"
	"github.com/Azure/container-copilot/pkg/mcp/internal/registry/credential_providers"
	sessiontypes "github.com/Azure/container-copilot/pkg/mcp/internal/session"
	"github.com/Azure/container-copilot/pkg/mcp/internal/types"
	mcptypes "github.com/Azure/container-copilot/pkg/mcp/types"
	"github.com/rs/zerolog"
)

// DockerConfig represents the structure of Docker's config.json file
type DockerConfig struct {
	Auths map[string]DockerAuth `json:"auths"`
	// CredHelpers and other fields can be added later for extended support
	CredHelpers       map[string]string `json:"credHelpers,omitempty"`
	CredsStore        string            `json:"credsStore,omitempty"`
	CredentialHelpers map[string]string `json:"credentialHelpers,omitempty"`
}

// DockerAuth represents authentication information for a registry
type DockerAuth struct {
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
	Email    string `json:"email,omitempty"`
	Auth     string `json:"auth,omitempty"` // base64 encoded username:password
	// ServerURL is typically the key in the auths map
}

// RegistryAuthInfo contains parsed authentication information for a registry
type RegistryAuthInfo struct {
	Registry string
	Username string
	HasAuth  bool
	AuthType string // "basic", "helper", "store"
	Helper   string // credential helper name if applicable
}

// RegistryAuthSummary contains authentication status for all configured registries
type RegistryAuthSummary struct {
	ConfigPath    string
	Registries    []RegistryAuthInfo
	DefaultHelper string
	HasStore      bool
}

// PreFlightChecker validates system requirements before starting workflow
type PreFlightChecker struct {
	logger            zerolog.Logger
	timeout           time.Duration
	registryMgr       *registry.MultiRegistryManager
	registryValidator *registry.RegistryValidator
}

// PreFlightCheck represents a single validation check
type PreFlightCheck struct {
	Name          string `json:"name"`
	Description   string `json:"description"`
	CheckFunc     func(context.Context) error
	ErrorRecovery string `json:"error_recovery"`
	Optional      bool   `json:"optional"`
	Category      string `json:"category"` // docker, kubernetes, registry, system
}

// PreFlightResult contains the results of all pre-flight checks
type PreFlightResult struct {
	Passed      bool              `json:"passed"`
	Timestamp   time.Time         `json:"timestamp"`
	Duration    time.Duration     `json:"duration"`
	Checks      []CheckResult     `json:"checks"`
	Suggestions map[string]string `json:"suggestions"`
	CanProceed  bool              `json:"can_proceed"`
}

// CheckResult represents the result of a single check
type CheckResult struct {
	Name           string        `json:"name"`
	Category       string        `json:"category"`
	Status         CheckStatus   `json:"status"`
	Message        string        `json:"message"`
	Error          string        `json:"error,omitempty"`
	Duration       time.Duration `json:"duration"`
	RecoveryAction string        `json:"recovery_action,omitempty"`
}

// CheckStatus represents the status of a check
type CheckStatus string

const (
	CheckStatusPass    CheckStatus = "pass"
	CheckStatusFail    CheckStatus = "fail"
	CheckStatusWarning CheckStatus = "warning"
	CheckStatusSkipped CheckStatus = "skipped"
)

// NewPreFlightChecker creates a new pre-flight checker
func NewPreFlightChecker(logger zerolog.Logger) *PreFlightChecker {
	// Create multi-registry configuration with defaults
	config := &registry.MultiRegistryConfig{
		Registries:   make(map[string]registry.RegistryConfig),
		CacheTimeout: 15 * time.Minute,
		MaxRetries:   3,
	}

	// Initialize multi-registry manager
	registryMgr := registry.NewMultiRegistryManager(config, logger)

	// Register credential providers
	registryMgr.RegisterProvider(credential_providers.NewDockerConfigProvider(logger))
	registryMgr.RegisterProvider(credential_providers.NewAzureCLIProvider(logger))
	registryMgr.RegisterProvider(credential_providers.NewAWSECRProvider(logger))

	// Initialize registry validator
	validator := registry.NewRegistryValidator(logger)

	return &PreFlightChecker{
		logger:            logger,
		timeout:           10 * time.Second,
		registryMgr:       registryMgr,
		registryValidator: validator,
	}
}

// RunStageChecks executes pre-flight checks for a specific stage
func (pfc *PreFlightChecker) RunStageChecks(ctx context.Context, stage types.ConversationStage, state *sessiontypes.SessionState) (*PreFlightResult, error) {
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
func (pfc *PreFlightChecker) getChecksForStage(stage types.ConversationStage, state *sessiontypes.SessionState) []PreFlightCheck {
	switch stage {
	case types.StageBuild:
		return pfc.getBuildChecks(state)
	case types.StagePush:
		return pfc.getPushChecks(state)
	case types.StageManifests:
		return pfc.getManifestChecks(state)
	case types.StageDeployment:
		return pfc.getDeploymentChecks(state)
	default:
		return []PreFlightCheck{}
	}
}

// getBuildChecks returns pre-flight checks for the build stage
func (pfc *PreFlightChecker) getBuildChecks(state *sessiontypes.SessionState) []PreFlightCheck {
	checks := []PreFlightCheck{
		{
			Name:        "Dockerfile exists",
			Description: "Verify Dockerfile has been generated",
			Category:    "docker",
			CheckFunc: func(ctx context.Context) error {
				if state.Dockerfile.Content == "" {
					return mcptypes.NewRichError("DOCKERFILE_NOT_GENERATED", "Dockerfile not generated yet", "validation_error")
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
			CheckFunc:     pfc.checkDockerDaemon,
			ErrorRecovery: "Start Docker Desktop or Docker daemon",
			Optional:      false,
		},
		{
			Name:          "Sufficient disk space",
			Description:   "Check if there's enough disk space for build",
			Category:      "system",
			CheckFunc:     pfc.checkDiskSpace,
			ErrorRecovery: "Free up disk space (need at least 2GB)",
			Optional:      true,
		},
	}

	// Add Dockerfile validation check if we have validation results
	if state.Dockerfile.ValidationResult != nil {
		checks = append(checks, PreFlightCheck{
			Name:        "Dockerfile validation",
			Description: "Ensure Dockerfile has no critical errors",
			Category:    "docker",
			CheckFunc: func(ctx context.Context) error {
				if !state.Dockerfile.ValidationResult.Valid && state.Dockerfile.ValidationResult.ErrorCount > 0 {
					return mcptypes.NewRichError("DOCKERFILE_VALIDATION_FAILED", fmt.Sprintf("Dockerfile has %d critical validation errors", state.Dockerfile.ValidationResult.ErrorCount), "validation_error")
				}
				return nil
			},
			ErrorRecovery: "Fix critical Dockerfile errors before building",
			Optional:      false,
		})
	}

	return checks
}

// getPushChecks returns pre-flight checks for the push stage
func (pfc *PreFlightChecker) getPushChecks(state *sessiontypes.SessionState) []PreFlightCheck {
	checks := []PreFlightCheck{
		{
			Name:        "Image built",
			Description: "Verify Docker image has been built",
			Category:    "docker",
			CheckFunc: func(ctx context.Context) error {
				if !state.Dockerfile.Built || state.Dockerfile.ImageID == "" {
					return mcptypes.NewRichError("IMAGE_NOT_BUILT", "Docker image not built yet", "validation_error")
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
				// Check if we have registry credentials
				if state.ImageRef.Registry == "" {
					return mcptypes.NewRichError("NO_REGISTRY_SPECIFIED", "no registry specified", "configuration_error")
				}

				// Try to ping the registry using docker
				ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
				defer cancel()

				// Use docker manifest inspect to check connectivity
				testImage := fmt.Sprintf("%s/library/hello-world:latest", state.ImageRef.Registry)
				cmd := exec.CommandContext(ctx, "docker", "manifest", "inspect", testImage)
				if err := cmd.Run(); err != nil {
					// Try without library prefix
					testImage = fmt.Sprintf("%s/hello-world:latest", state.ImageRef.Registry)
					cmd = exec.CommandContext(ctx, "docker", "manifest", "inspect", testImage)
					if err := cmd.Run(); err != nil {
						return mcptypes.WrapRichError(err, "REGISTRY_CONNECTION_FAILED", fmt.Sprintf("cannot connect to registry %s", state.ImageRef.Registry), "network_error")
					}
				}

				return nil
			},
			ErrorRecovery: "Ensure registry URL is correct and you're logged in",
			Optional:      false,
		},
		{
			Name:          "Registry authentication",
			Description:   "Verify registry authentication",
			Category:      "registry",
			CheckFunc:     pfc.checkRegistryAuth,
			ErrorRecovery: "Run 'docker login' or configure registry credentials",
			Optional:      false,
		},
	}

	// Add security scan check if scan results are available
	if state.SecurityScan != nil {
		checks = append(checks, PreFlightCheck{
			Name:        "Security vulnerabilities",
			Description: "Ensure image has no critical vulnerabilities",
			Category:    "security",
			CheckFunc: func(ctx context.Context) error {
				if state.SecurityScan.Summary.Critical > 0 {
					return mcptypes.NewRichError("CRITICAL_VULNERABILITIES", fmt.Sprintf("image has %d CRITICAL vulnerabilities", state.SecurityScan.Summary.Critical), "security_error")
				}
				if state.SecurityScan.Summary.High > 3 {
					return mcptypes.NewRichError("HIGH_VULNERABILITIES", fmt.Sprintf("image has %d HIGH vulnerabilities (threshold: 3)", state.SecurityScan.Summary.High), "security_error")
				}
				return nil
			},
			ErrorRecovery: "Fix critical vulnerabilities before pushing to registry",
			Optional:      false,
		})
	}

	return checks
}

// getManifestChecks returns pre-flight checks for manifest generation
func (pfc *PreFlightChecker) getManifestChecks(state *sessiontypes.SessionState) []PreFlightCheck {
	return []PreFlightCheck{
		{
			Name:        "Image reference available",
			Description: "Verify image has been built or pushed",
			Category:    "docker",
			CheckFunc: func(ctx context.Context) error {
				if state.ImageRef.Repository == "" {
					return mcptypes.NewRichError("NO_IMAGE_REFERENCE", "no image reference available", "validation_error")
				}
				return nil
			},
			ErrorRecovery: "Build and optionally push Docker image first",
			Optional:      false,
		},
	}
}

// getDeploymentChecks returns pre-flight checks for deployment
func (pfc *PreFlightChecker) getDeploymentChecks(state *sessiontypes.SessionState) []PreFlightCheck {
	return []PreFlightCheck{
		{
			Name:          "Kubernetes connectivity",
			Description:   "Check if kubectl can connect to cluster",
			Category:      "kubernetes",
			CheckFunc:     pfc.checkKubernetesConnectivity,
			ErrorRecovery: "Configure kubectl to connect to your cluster",
			Optional:      false,
		},
		{
			Name:        "Manifests generated",
			Description: "Verify Kubernetes manifests exist",
			Category:    "kubernetes",
			CheckFunc: func(ctx context.Context) error {
				if len(state.K8sManifests) == 0 {
					return mcptypes.NewRichError("NO_K8S_MANIFESTS", "no Kubernetes manifests generated", "validation_error")
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

		// Create context with timeout for individual check
		checkCtx, cancel := context.WithTimeout(ctx, pfc.timeout)
		defer cancel()

		result := CheckResult{
			Name:     check.Name,
			Category: check.Category,
			Status:   CheckStatusPass,
		}

		// Run the check
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
			CheckFunc:     pfc.checkDockerDaemon,
			ErrorRecovery: "Please start Docker Desktop or run: sudo systemctl start docker",
			Optional:      false,
		},
		{
			Name:          "docker_disk_space",
			Description:   "Check available disk space for Docker",
			Category:      "system",
			CheckFunc:     pfc.checkDockerDiskSpace,
			ErrorRecovery: "Please free up at least 5GB of disk space for container builds",
			Optional:      false,
		},
		{
			Name:          "kubernetes_context",
			Description:   "Check if kubectl is configured with a valid context",
			Category:      "kubernetes",
			CheckFunc:     pfc.checkKubernetesContext,
			ErrorRecovery: "Please configure kubectl with: kubectl config use-context <context-name>",
			Optional:      true, // Can skip if only building, not deploying
		},
		{
			Name:          "kubernetes_connectivity",
			Description:   "Check connectivity to Kubernetes cluster",
			Category:      "kubernetes",
			CheckFunc:     pfc.checkKubernetesConnectivity,
			ErrorRecovery: "Please ensure your Kubernetes cluster is accessible",
			Optional:      true,
		},
		{
			Name:          "registry_auth",
			Description:   "Check Docker registry authentication",
			Category:      "registry",
			CheckFunc:     pfc.checkRegistryAuth,
			ErrorRecovery: "Please authenticate with: docker login <registry>",
			Optional:      true, // Can use local images only
		},
		{
			Name:          "required_tools",
			Description:   "Check for required CLI tools",
			Category:      "system",
			CheckFunc:     pfc.checkRequiredTools,
			ErrorRecovery: "Please install missing tools",
			Optional:      false,
		},
		{
			Name:          "git_installed",
			Description:   "Check if git is installed for repository operations",
			Category:      "system",
			CheckFunc:     pfc.checkGitInstalled,
			ErrorRecovery: "Please install git: https://git-scm.com/downloads",
			Optional:      true,
		},
	}
}

// Check implementations

func (pfc *PreFlightChecker) checkDockerDaemon(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, "docker", "version", "--format", "{{.Server.Version}}")
	output, err := cmd.Output()
	if err != nil {
		return mcptypes.WrapRichError(err, "DOCKER_DAEMON_NOT_ACCESSIBLE", "Docker daemon not accessible", "system_error")
	}

	version := strings.TrimSpace(string(output))
	if version == "" {
		return mcptypes.NewRichError("DOCKER_DAEMON_NOT_RUNNING", "Docker daemon not running", "system_error")
	}

	pfc.logger.Debug().Str("docker_version", version).Msg("Docker daemon check passed")
	return nil
}

func (pfc *PreFlightChecker) checkDockerDiskSpace(ctx context.Context) error {
	// Get Docker root directory
	cmd := exec.CommandContext(ctx, "docker", "info", "--format", "{{.DockerRootDir}}")
	output, err := cmd.Output()
	if err != nil {
		return mcptypes.WrapRichError(err, "DOCKER_ROOT_DIR_FAILED", "failed to get Docker root directory", "system_error")
	}

	dockerRoot := strings.TrimSpace(string(output))
	if dockerRoot == "" {
		dockerRoot = "/var/lib/docker" // Default location
	}

	// Check disk space using df
	cmd = exec.CommandContext(ctx, "df", "-BG", dockerRoot)
	output, err = cmd.Output()
	if err != nil {
		// Fallback to checking root filesystem
		cmd = exec.CommandContext(ctx, "df", "-BG", "/")
		output, err = cmd.Output()
		if err != nil {
			return fmt.Errorf("failed to check disk space: %w", err)
		}
	}

	// Parse df output
	lines := strings.Split(string(output), "\n")
	if len(lines) < 2 {
		return fmt.Errorf("unexpected df output format")
	}

	// Parse available space from second line
	fields := strings.Fields(lines[1])
	if len(fields) < 4 {
		return fmt.Errorf("unexpected df output format")
	}

	// Extract number from "123G" format
	availStr := strings.TrimSuffix(fields[3], "G")
	availGB, err := strconv.Atoi(availStr)
	if err != nil {
		return fmt.Errorf("failed to parse available space: %w", err)
	}

	const minSpaceGB = 5
	if availGB < minSpaceGB {
		return fmt.Errorf("insufficient disk space: %dGB available, need at least %dGB", availGB, minSpaceGB)
	}

	pfc.logger.Debug().Int("available_gb", availGB).Msg("Docker disk space check passed")
	return nil
}

func (pfc *PreFlightChecker) checkRegistryAuth(ctx context.Context) error {
	// Use both legacy and new registry authentication systems
	summary, err := pfc.parseRegistryAuth(ctx)
	if err != nil {
		pfc.logger.Debug().Err(err).Msg("Legacy registry auth parsing failed, using enhanced system")
	} else {
		// Log legacy registry authentication information
		pfc.logger.Info().
			Str("config_path", summary.ConfigPath).
			Int("registry_count", len(summary.Registries)).
			Bool("has_default_store", summary.HasStore).
			Str("default_helper", summary.DefaultHelper).
			Msg("Legacy registry authentication status")
	}

	// Test enhanced registry authentication system
	return pfc.checkEnhancedRegistryAuth(ctx)
}

// checkEnhancedRegistryAuth validates registry authentication using the new multi-registry system
func (pfc *PreFlightChecker) checkEnhancedRegistryAuth(ctx context.Context) error {
	pfc.logger.Info().Msg("Validating enhanced registry authentication")

	// Test common registries
	testRegistries := []string{
		"docker.io",
		"index.docker.io",
	}

	hasAnyAuth := false
	authResults := make(map[string]string)

	for _, registryURL := range testRegistries {
		pfc.logger.Debug().
			Str("registry", registryURL).
			Msg("Testing registry authentication")

		// Try to get credentials
		creds, err := pfc.registryMgr.GetCredentials(ctx, registryURL)
		if err != nil {
			authResults[registryURL] = fmt.Sprintf("No credentials: %v", err)
			continue
		}

		if creds != nil {
			hasAnyAuth = true
			authResults[registryURL] = fmt.Sprintf("Authenticated via %s (%s)", creds.Source, creds.AuthMethod)

			// Validate registry access
			if err := pfc.registryMgr.ValidateRegistryAccess(ctx, registryURL); err != nil {
				authResults[registryURL] += fmt.Sprintf(" - Validation failed: %v", err)
			} else {
				authResults[registryURL] += " - Access validated"
			}
		} else {
			authResults[registryURL] = "No credentials available"
		}
	}

	// Log results
	for registry, result := range authResults {
		pfc.logger.Info().
			Str("registry", registry).
			Str("result", result).
			Msg("Registry authentication test result")
	}

	// Check if we have at least some authentication capability
	if !hasAnyAuth {
		// Don't fail completely - warn but allow proceeding
		pfc.logger.Warn().Msg("No registry authentication found - some operations may fail")
		return nil
	}

	pfc.logger.Info().Msg("Enhanced registry authentication validation completed")
	return nil
}

// GetRegistryManager returns the multi-registry manager
func (pfc *PreFlightChecker) GetRegistryManager() *registry.MultiRegistryManager {
	return pfc.registryMgr
}

// GetRegistryValidator returns the registry validator
func (pfc *PreFlightChecker) GetRegistryValidator() *registry.RegistryValidator {
	return pfc.registryValidator
}

// ValidateSpecificRegistry validates authentication and connectivity for a specific registry
func (pfc *PreFlightChecker) ValidateSpecificRegistry(ctx context.Context, registryURL string) (*registry.ValidationResult, error) {
	pfc.logger.Info().
		Str("registry", registryURL).
		Msg("Validating specific registry")

	// Get credentials for the registry
	creds, err := pfc.registryMgr.GetCredentials(ctx, registryURL)
	if err != nil {
		pfc.logger.Debug().
			Str("registry", registryURL).
			Err(err).
			Msg("No credentials available for registry")
		// Continue validation without credentials
		creds = nil
	}

	// Validate the registry
	result, err := pfc.registryValidator.ValidateRegistry(ctx, registryURL, creds)
	if err != nil {
		return nil, fmt.Errorf("registry validation failed: %w", err)
	}

	return result, nil
}

// parseRegistryAuth parses the Docker config file and extracts authentication information
func (pfc *PreFlightChecker) parseRegistryAuth(ctx context.Context) (*RegistryAuthSummary, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home directory: %w", err)
	}

	dockerConfigPath := filepath.Join(homeDir, ".docker", "config.json")
	if _, err := os.Stat(dockerConfigPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("Docker config not found at %s - run 'docker login' first", dockerConfigPath)
	}

	// Parse Docker config to check authentication details
	configData, err := os.ReadFile(dockerConfigPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read Docker config: %w", err)
	}

	var config DockerConfig
	if err := json.Unmarshal(configData, &config); err != nil {
		return nil, fmt.Errorf("failed to parse Docker config JSON: %w", err)
	}

	// Build RegistryAuthSummary
	summary := &RegistryAuthSummary{
		ConfigPath:    dockerConfigPath,
		Registries:    []RegistryAuthInfo{},
		DefaultHelper: config.CredsStore,
		HasStore:      config.CredsStore != "",
	}

	// Process registry authentication entries
	for registryURL, authEntry := range config.Auths {
		regInfo := RegistryAuthInfo{
			Registry: registryURL,
			HasAuth:  authEntry.Auth != "",
			AuthType: "basic",
		}

		if authEntry.Auth != "" {
			// Extract username from auth string (basic auth is base64 encoded username:password)
			if decoded, err := base64.StdEncoding.DecodeString(authEntry.Auth); err == nil {
				parts := strings.SplitN(string(decoded), ":", 2)
				if len(parts) > 0 {
					regInfo.Username = parts[0]
				}
			}
		}

		summary.Registries = append(summary.Registries, regInfo)
	}

	// Process credential helpers
	for registry, helper := range config.CredHelpers {
		// Check if this registry already exists in our list
		found := false
		for i, reg := range summary.Registries {
			if reg.Registry == registry {
				summary.Registries[i].AuthType = "helper"
				summary.Registries[i].Helper = helper
				summary.Registries[i].HasAuth = true
				found = true
				break
			}
		}

		if !found {
			regInfo := RegistryAuthInfo{
				Registry: registry,
				HasAuth:  true,
				AuthType: "helper",
				Helper:   helper,
			}
			summary.Registries = append(summary.Registries, regInfo)
		}
	}

	// Process credential store fallback
	if config.CredsStore != "" {
		// Add global credential store support
		if err := pfc.validateCredentialStore(ctx, config.CredsStore); err != nil {
			pfc.logger.Warn().
				Str("credential_store", config.CredsStore).
				Err(err).
				Msg("Credential store validation failed, will fallback to other methods")
		}
	}

	return summary, nil
}

func (pfc *PreFlightChecker) checkDiskSpace(ctx context.Context) error {
	// Check available disk space
	cmd := exec.CommandContext(ctx, "df", "-h", "/var/lib/docker")
	output, err := cmd.Output()
	if err != nil {
		// Try alternative location
		cmd = exec.CommandContext(ctx, "df", "-h", "/")
		output, err = cmd.Output()
		if err != nil {
			return fmt.Errorf("failed to check disk space: %w", err)
		}
	}

	// Parse output to check available space
	lines := strings.Split(string(output), "\n")
	if len(lines) < 2 {
		return fmt.Errorf("unexpected df output format")
	}

	// Basic check - just ensure we're not critically low
	// In production, would parse the actual values
	outputStr := string(output)
	if strings.Contains(outputStr, "100%") || strings.Contains(outputStr, "99%") || strings.Contains(outputStr, "98%") {
		return fmt.Errorf("disk space critically low")
	}

	return nil
}

func (pfc *PreFlightChecker) checkKubernetesContext(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, "kubectl", "config", "current-context")
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("no Kubernetes context configured: %w", err)
	}

	context := strings.TrimSpace(string(output))
	if context == "" {
		return fmt.Errorf("no current Kubernetes context set")
	}

	pfc.logger.Debug().Str("context", context).Msg("Kubernetes context check passed")
	return nil
}

func (pfc *PreFlightChecker) checkKubernetesConnectivity(ctx context.Context) error {
	// Try to get server version
	cmd := exec.CommandContext(ctx, "kubectl", "version", "--short", "--output=json")
	output, err := cmd.Output()
	if err != nil {
		// Check if it's just a warning about version skew
		if exitErr, ok := err.(*exec.ExitError); ok {
			stderr := string(exitErr.Stderr)
			if strings.Contains(stderr, "connection refused") || strings.Contains(stderr, "no such host") {
				return fmt.Errorf("cannot connect to Kubernetes cluster: %s", stderr)
			}
		}
		// Might be version skew warning, try simpler check
		cmd = exec.CommandContext(ctx, "kubectl", "get", "nodes", "--no-headers")
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("cannot connect to Kubernetes cluster: %w", err)
		}
	}

	pfc.logger.Debug().Str("output", string(output)).Msg("Kubernetes connectivity check passed")
	return nil
}

func (pfc *PreFlightChecker) checkRequiredTools(ctx context.Context) error {
	requiredTools := []string{"docker", "kubectl"}
	missingTools := []string{}

	for _, tool := range requiredTools {
		cmd := exec.CommandContext(ctx, "which", tool)
		if err := cmd.Run(); err != nil {
			missingTools = append(missingTools, tool)
		}
	}

	if len(missingTools) > 0 {
		return fmt.Errorf("required tools not found: %s", strings.Join(missingTools, ", "))
	}

	pfc.logger.Debug().Msg("Required tools check passed")
	return nil
}

func (pfc *PreFlightChecker) checkGitInstalled(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, "git", "--version")
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("git not installed: %w", err)
	}

	version := strings.TrimSpace(string(output))
	pfc.logger.Debug().Str("git_version", version).Msg("Git check passed")
	return nil
}

// GetCheckByName returns a specific check by name
func (pfc *PreFlightChecker) GetCheckByName(name string) (*PreFlightCheck, error) {
	for _, check := range pfc.getChecks() {
		if check.Name == name {
			return &check, nil
		}
	}
	return nil, fmt.Errorf("check not found: %s", name)
}

// RunSingleCheck runs a specific check
func (pfc *PreFlightChecker) RunSingleCheck(ctx context.Context, checkName string) (*CheckResult, error) {
	check, err := pfc.GetCheckByName(checkName)
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
func (pfc *PreFlightChecker) FormatResults(results *PreFlightResult) string {
	var sb strings.Builder

	sb.WriteString("Pre-flight Check Results:\n")
	sb.WriteString(fmt.Sprintf("Overall Status: %s\n", pfc.getOverallStatus(results)))
	sb.WriteString(fmt.Sprintf("Duration: %v\n\n", results.Duration.Round(time.Millisecond)))

	// Group by category
	byCategory := make(map[string][]CheckResult)
	for _, check := range results.Checks {
		byCategory[check.Category] = append(byCategory[check.Category], check)
	}

	// Display results by category
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

// validateCredentialStore validates that a credential store helper is available and functional
func (pfc *PreFlightChecker) validateCredentialStore(ctx context.Context, credStore string) error {
	if credStore == "" {
		return fmt.Errorf("no credential store specified")
	}

	// Try to execute the credential helper to see if it's available
	helperName := fmt.Sprintf("docker-credential-%s", credStore)

	cmd := exec.CommandContext(ctx, helperName, "version")
	if err := cmd.Run(); err != nil {
		// If version command fails, try to check if the helper exists in PATH
		if _, pathErr := exec.LookPath(helperName); pathErr != nil {
			return fmt.Errorf("credential store helper '%s' not found in PATH", helperName)
		}

		// If helper exists but version fails, it might still work for get/store operations
		pfc.logger.Debug().
			Str("helper", helperName).
			Msg("Credential store helper exists but version check failed")
	}

	pfc.logger.Debug().
		Str("credential_store", credStore).
		Str("helper", helperName).
		Msg("Credential store validation successful")

	return nil
}

// getCredentialWithFallback attempts to get credentials using multiple fallback methods
func (pfc *PreFlightChecker) getCredentialWithFallback(ctx context.Context, registry string, config *DockerConfig) (*RegistryAuthInfo, error) {
	authInfo := &RegistryAuthInfo{
		Registry: registry,
		HasAuth:  false,
	}

	// 1. Try direct auth from config
	if auth, exists := config.Auths[registry]; exists && auth.Auth != "" {
		authInfo.HasAuth = true
		authInfo.AuthType = "basic"

		// Extract username from auth string
		if decoded, err := base64.StdEncoding.DecodeString(auth.Auth); err == nil {
			parts := strings.SplitN(string(decoded), ":", 2)
			if len(parts) > 0 {
				authInfo.Username = parts[0]
			}
		}
		return authInfo, nil
	}

	// 2. Try registry-specific credential helper
	if helper, exists := config.CredHelpers[registry]; exists {
		if err := pfc.tryCredentialHelper(ctx, registry, helper, authInfo); err == nil {
			return authInfo, nil
		} else {
			pfc.logger.Debug().
				Str("registry", registry).
				Str("helper", helper).
				Err(err).
				Msg("Registry-specific credential helper failed")
		}
	}

	// 3. Try global credential store
	if config.CredsStore != "" {
		if err := pfc.tryCredentialHelper(ctx, registry, config.CredsStore, authInfo); err == nil {
			return authInfo, nil
		} else {
			pfc.logger.Debug().
				Str("registry", registry).
				Str("store", config.CredsStore).
				Err(err).
				Msg("Global credential store failed")
		}
	}

	// 4. Try environment variables for common registries
	if err := pfc.tryEnvironmentCredentials(registry, authInfo); err == nil {
		return authInfo, nil
	}

	return authInfo, fmt.Errorf("no credentials found for registry %s", registry)
}

// tryCredentialHelper attempts to get credentials using a specific credential helper
func (pfc *PreFlightChecker) tryCredentialHelper(ctx context.Context, registry, helper string, authInfo *RegistryAuthInfo) error {
	helperName := fmt.Sprintf("docker-credential-%s", helper)

	cmd := exec.CommandContext(ctx, helperName, "get")
	cmd.Stdin = strings.NewReader(registry)

	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("credential helper failed: %w", err)
	}

	// Parse credential helper response
	var cred struct {
		Username string `json:"Username"`
		Secret   string `json:"Secret"`
	}

	if err := json.Unmarshal(output, &cred); err != nil {
		return fmt.Errorf("failed to parse credential helper response: %w", err)
	}

	if cred.Username != "" && cred.Secret != "" {
		authInfo.HasAuth = true
		authInfo.AuthType = "helper"
		authInfo.Helper = helper
		authInfo.Username = cred.Username
		return nil
	}

	return fmt.Errorf("credential helper returned empty credentials")
}

// tryEnvironmentCredentials attempts to get credentials from environment variables
func (pfc *PreFlightChecker) tryEnvironmentCredentials(registry string, authInfo *RegistryAuthInfo) error {
	// Check for common registry environment variable patterns
	var userEnv, passEnv string

	switch {
	case strings.Contains(registry, "docker.io") || strings.Contains(registry, "index.docker.io"):
		userEnv = "DOCKER_USERNAME"
		passEnv = "DOCKER_PASSWORD"
	case strings.Contains(registry, "ghcr.io"):
		userEnv = "GITHUB_USERNAME"
		passEnv = "GITHUB_TOKEN"
	case strings.Contains(registry, "quay.io"):
		userEnv = "QUAY_USERNAME"
		passEnv = "QUAY_PASSWORD"
	case strings.Contains(registry, "gcr.io"):
		userEnv = "GCR_USERNAME"
		passEnv = "GCR_PASSWORD"
	default:
		// Try generic patterns
		registryName := strings.Split(registry, ".")[0]
		registryName = strings.ToUpper(strings.ReplaceAll(registryName, "-", "_"))
		userEnv = fmt.Sprintf("%s_USERNAME", registryName)
		passEnv = fmt.Sprintf("%s_PASSWORD", registryName)
	}

	username := os.Getenv(userEnv)
	password := os.Getenv(passEnv)

	if username != "" && password != "" {
		authInfo.HasAuth = true
		authInfo.AuthType = "environment"
		authInfo.Username = username
		return nil
	}

	return fmt.Errorf("no environment credentials found for registry %s", registry)
}

// ValidateMultipleRegistries validates authentication and connectivity for multiple registries
func (pfc *PreFlightChecker) ValidateMultipleRegistries(ctx context.Context, registries []string) (*MultiRegistryValidationResult, error) {
	result := &MultiRegistryValidationResult{
		Timestamp: time.Now(),
		Results:   make(map[string]*RegistryValidationResult),
	}

	// Parse Docker config once
	config, err := pfc.parseDockerConfig()
	if err != nil {
		pfc.logger.Warn().Err(err).Msg("Failed to parse Docker config, will try environment credentials")
		// Continue with empty config to try environment variables
		config = &DockerConfig{
			Auths:       make(map[string]DockerAuth),
			CredHelpers: make(map[string]string),
		}
	}

	for _, registry := range registries {
		registryResult := &RegistryValidationResult{
			Registry:  registry,
			Timestamp: time.Now(),
		}

		// Test authentication
		authInfo, err := pfc.getCredentialWithFallback(ctx, registry, config)
		if err != nil {
			registryResult.AuthenticationStatus = "failed"
			registryResult.AuthenticationError = err.Error()
		} else {
			registryResult.AuthenticationStatus = "success"
			registryResult.AuthenticationType = authInfo.AuthType
			registryResult.Username = authInfo.Username
		}

		// Test connectivity
		if err := pfc.testRegistryConnectivity(ctx, registry); err != nil {
			registryResult.ConnectivityStatus = "failed"
			registryResult.ConnectivityError = err.Error()
		} else {
			registryResult.ConnectivityStatus = "success"
		}

		// Overall status
		registryResult.OverallStatus = "success"
		if registryResult.AuthenticationStatus == "failed" || registryResult.ConnectivityStatus == "failed" {
			registryResult.OverallStatus = "failed"
			result.HasFailures = true
		}

		result.Results[registry] = registryResult
	}

	result.Duration = time.Since(result.Timestamp)
	return result, nil
}

// parseDockerConfig parses Docker configuration and returns it
func (pfc *PreFlightChecker) parseDockerConfig() (*DockerConfig, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home directory: %w", err)
	}

	dockerConfigPath := filepath.Join(homeDir, ".docker", "config.json")
	if _, err := os.Stat(dockerConfigPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("Docker config not found at %s", dockerConfigPath)
	}

	configData, err := os.ReadFile(dockerConfigPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read Docker config: %w", err)
	}

	var config DockerConfig
	if err := json.Unmarshal(configData, &config); err != nil {
		return nil, fmt.Errorf("failed to parse Docker config JSON: %w", err)
	}

	return &config, nil
}

// testRegistryConnectivity tests connectivity to a registry
func (pfc *PreFlightChecker) testRegistryConnectivity(ctx context.Context, registry string) error {
	// Use docker manifest inspect to test connectivity with a well-known image
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	// Try common test images based on registry
	testImages := pfc.getTestImagesForRegistry(registry)

	for _, testImage := range testImages {
		cmd := exec.CommandContext(ctx, "docker", "manifest", "inspect", testImage)
		if err := cmd.Run(); err == nil {
			pfc.logger.Debug().
				Str("registry", registry).
				Str("test_image", testImage).
				Msg("Registry connectivity test passed")
			return nil
		}
	}

	return fmt.Errorf("failed to connect to registry %s with any test image", registry)
}

// getTestImagesForRegistry returns appropriate test images for different registries
func (pfc *PreFlightChecker) getTestImagesForRegistry(registry string) []string {
	switch {
	case strings.Contains(registry, "docker.io") || strings.Contains(registry, "index.docker.io"):
		return []string{"docker.io/library/hello-world:latest", "hello-world:latest"}
	case strings.Contains(registry, "ghcr.io"):
		return []string{"ghcr.io/containerbase/base:latest"}
	case strings.Contains(registry, "quay.io"):
		return []string{"quay.io/prometheus/busybox:latest"}
	case strings.Contains(registry, "gcr.io"):
		return []string{"gcr.io/google-containers/pause:latest"}
	case strings.Contains(registry, "mcr.microsoft.com"):
		return []string{"mcr.microsoft.com/hello-world:latest"}
	default:
		// For unknown registries, try a generic approach
		return []string{
			fmt.Sprintf("%s/hello-world:latest", registry),
			fmt.Sprintf("%s/library/hello-world:latest", registry),
		}
	}
}

// MultiRegistryValidationResult represents validation results for multiple registries
type MultiRegistryValidationResult struct {
	Timestamp   time.Time                            `json:"timestamp"`
	Duration    time.Duration                        `json:"duration"`
	Results     map[string]*RegistryValidationResult `json:"results"`
	HasFailures bool                                 `json:"has_failures"`
}

// RegistryValidationResult represents validation result for a single registry
type RegistryValidationResult struct {
	Registry             string    `json:"registry"`
	Timestamp            time.Time `json:"timestamp"`
	OverallStatus        string    `json:"overall_status"`
	AuthenticationStatus string    `json:"authentication_status"`
	AuthenticationError  string    `json:"authentication_error,omitempty"`
	AuthenticationType   string    `json:"authentication_type,omitempty"`
	Username             string    `json:"username,omitempty"`
	ConnectivityStatus   string    `json:"connectivity_status"`
	ConnectivityError    string    `json:"connectivity_error,omitempty"`
}

func (pfc *PreFlightChecker) getOverallStatus(results *PreFlightResult) string {
	if results.Passed {
		return "✅ All checks passed"
	}
	if results.CanProceed {
		return "⚠️  Some optional checks failed"
	}
	return "❌ Required checks failed"
}

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
