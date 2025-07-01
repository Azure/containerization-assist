package observability

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/internal/registry"
	"github.com/rs/zerolog"
)

// RegistryValidator handles registry authentication and connectivity checks
type RegistryValidator struct {
	logger            zerolog.Logger
	registryValidator *registry.RegistryValidator
}

// NewRegistryValidator creates a new registry validator
func NewRegistryValidator(logger zerolog.Logger) *RegistryValidator {
	return &RegistryValidator{
		logger:            logger,
		registryValidator: registry.NewRegistryValidator(logger),
	}
}

// CheckRegistryAuth validates registry authentication
func (rv *RegistryValidator) CheckRegistryAuth(ctx context.Context) error {
	summary, err := rv.parseRegistryAuth(ctx)
	if err != nil {
		rv.logger.Debug().Err(err).Msg("Legacy registry auth parsing failed, using enhanced system")
	} else {
		rv.logger.Info().
			Str("config_path", summary.ConfigPath).
			Int("registry_count", len(summary.Registries)).
			Bool("has_default_store", summary.HasStore).
			Str("default_helper", summary.DefaultHelper).
			Msg("Legacy registry authentication status")
	}

	return rv.checkEnhancedRegistryAuth(ctx)
}

// CheckRegistryConnectivity tests connectivity to a specific registry
func (rv *RegistryValidator) CheckRegistryConnectivity(ctx context.Context, registryURL string) error {
	if registryURL == "" {
		return fmt.Errorf("no registry specified")
	}

	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	testImage := fmt.Sprintf("%s/library/hello-world:latest", registryURL)
	cmd := exec.CommandContext(ctx, "docker", "manifest", "inspect", testImage)
	if err := cmd.Run(); err != nil {
		testImage = fmt.Sprintf("%s/hello-world:latest", registryURL)
		cmd = exec.CommandContext(ctx, "docker", "manifest", "inspect", testImage)
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("cannot connect to registry %s: %v", registryURL, err)
		}
	}

	return nil
}

// checkEnhancedRegistryAuth validates registry authentication using the new multi-registry system
func (rv *RegistryValidator) checkEnhancedRegistryAuth(ctx context.Context) error {
	rv.logger.Info().Msg("Validating enhanced registry authentication")

	testRegistries := []string{
		"docker.io",
		"index.docker.io",
	}

	hasAnyAuth := false
	authResults := make(map[string]string)

	for _, registryURL := range testRegistries {
		rv.logger.Debug().
			Str("registry", registryURL).
			Msg("Testing registry authentication")

		// Temporarily skip registry validation during interface simplification
		authResults[registryURL] = "Registry validation temporarily disabled during interface simplification"
	}

	for registry, result := range authResults {
		rv.logger.Info().
			Str("registry", registry).
			Str("result", result).
			Msg("Registry authentication test result")
	}

	if !hasAnyAuth {
		rv.logger.Warn().Msg("No registry authentication found - some operations may fail")
		return nil
	}

	rv.logger.Info().Msg("Enhanced registry authentication validation completed")
	return nil
}

// GetRegistryValidator returns the internal registry validator
func (rv *RegistryValidator) GetRegistryValidator() *registry.RegistryValidator {
	return rv.registryValidator
}

// ValidateSpecificRegistry validates authentication and connectivity for a specific registry
func (rv *RegistryValidator) ValidateSpecificRegistry(ctx context.Context, registryURL string) (*registry.ValidationResult, error) {
	rv.logger.Info().
		Str("registry", registryURL).
		Msg("Validating specific registry")

	result, err := rv.registryValidator.ValidateRegistry(ctx, registryURL, nil)
	if err != nil {
		return nil, fmt.Errorf("registry validation failed: %v", err)
	}

	return result, nil
}

// parseRegistryAuth parses the Docker config file and extracts authentication information
func (rv *RegistryValidator) parseRegistryAuth(ctx context.Context) (*RegistryAuthSummary, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("error")
	}

	dockerConfigPath := filepath.Join(homeDir, ".docker", "config.json")
	if _, err := os.Stat(dockerConfigPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("error")
	}

	configData, err := os.ReadFile(dockerConfigPath)
	if err != nil {
		return nil, fmt.Errorf("error")
	}

	var config DockerConfig
	if err := json.Unmarshal(configData, &config); err != nil {
		return nil, fmt.Errorf("error")
	}

	summary := &RegistryAuthSummary{
		ConfigPath:    dockerConfigPath,
		Registries:    []RegistryAuthInfo{},
		DefaultHelper: config.CredsStore,
		HasStore:      config.CredsStore != "",
	}

	for registryURL, authEntry := range config.Auths {
		regInfo := RegistryAuthInfo{
			Registry: registryURL,
			HasAuth:  authEntry.Auth != "",
			AuthType: "basic",
		}

		if authEntry.Auth != "" {
			if decoded, err := base64.StdEncoding.DecodeString(authEntry.Auth); err == nil {
				parts := strings.SplitN(string(decoded), ":", 2)
				if len(parts) > 0 {
					regInfo.Username = parts[0]
				}
			}
		}

		summary.Registries = append(summary.Registries, regInfo)
	}

	for registry, helper := range config.CredHelpers {
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

	if config.CredsStore != "" {
		if err := rv.validateCredentialStore(ctx, config.CredsStore); err != nil {
			rv.logger.Warn().
				Str("credential_store", config.CredsStore).
				Err(err).
				Msg("Credential store validation failed, will fallback to other methods")
		}
	}

	return summary, nil
}

// validateCredentialStore validates that a credential store helper is available and functional
func (rv *RegistryValidator) validateCredentialStore(ctx context.Context, credStore string) error {
	if credStore == "" {
		return fmt.Errorf("no credential store specified")
	}

	helperName := fmt.Sprintf("docker-credential-%s", credStore)

	cmd := exec.CommandContext(ctx, helperName, "version")
	if err := cmd.Run(); err != nil {
		if _, pathErr := exec.LookPath(helperName); pathErr != nil {
			return fmt.Errorf("credential helper error: %s", helperName)
		}

		rv.logger.Debug().
			Str("helper", helperName).
			Msg("Credential store helper exists but version check failed")
	}

	rv.logger.Debug().
		Str("credential_store", credStore).
		Str("helper", helperName).
		Msg("Credential store validation successful")

	return nil
}

// ValidateMultipleRegistries validates authentication and connectivity for multiple registries
func (rv *RegistryValidator) ValidateMultipleRegistries(ctx context.Context, registries []string) (*MultiRegistryValidationResult, error) {
	result := &MultiRegistryValidationResult{
		Timestamp: time.Now(),
		Results:   make(map[string]*RegistryValidationResult),
	}

	config, err := rv.parseDockerConfig(ctx)
	if err != nil {
		rv.logger.Warn().Err(err).Msg("Failed to parse Docker config, will try environment credentials")
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

		authInfo, err := rv.getCredentialWithFallback(ctx, registry, config)
		if err != nil {
			registryResult.AuthenticationStatus = "failed"
			registryResult.AuthenticationError = err.Error()
		} else {
			registryResult.AuthenticationStatus = "success"
			registryResult.AuthenticationType = authInfo.AuthType
			registryResult.Username = authInfo.Username
		}

		if err := rv.testRegistryConnectivity(ctx, registry); err != nil {
			registryResult.ConnectivityStatus = "failed"
			registryResult.ConnectivityError = err.Error()
		} else {
			registryResult.ConnectivityStatus = "success"
		}

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
func (rv *RegistryValidator) parseDockerConfig(ctx context.Context) (*DockerConfig, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	dockerConfigPath := filepath.Join(homeDir, ".docker", "config.json")
	if _, err := os.Stat(dockerConfigPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("failed to read docker config: %s", dockerConfigPath)
	}

	configData, err := os.ReadFile(dockerConfigPath)
	if err != nil {
		return nil, err
	}

	var config DockerConfig
	if err := json.Unmarshal(configData, &config); err != nil {
		return nil, err
	}

	return &config, nil
}

// getCredentialWithFallback attempts to get credentials using multiple fallback methods
func (rv *RegistryValidator) getCredentialWithFallback(ctx context.Context, registry string, config *DockerConfig) (*RegistryAuthInfo, error) {
	authInfo := &RegistryAuthInfo{
		Registry: registry,
		HasAuth:  false,
	}

	if auth, exists := config.Auths[registry]; exists && auth.Auth != "" {
		authInfo.HasAuth = true
		authInfo.AuthType = "basic"

		if decoded, err := base64.StdEncoding.DecodeString(auth.Auth); err == nil {
			parts := strings.SplitN(string(decoded), ":", 2)
			if len(parts) > 0 {
				authInfo.Username = parts[0]
			}
		}
		return authInfo, nil
	}

	if helper, exists := config.CredHelpers[registry]; exists {
		if err := rv.tryCredentialHelper(ctx, registry, helper, authInfo); err == nil {
			return authInfo, nil
		} else {
			rv.logger.Debug().
				Str("registry", registry).
				Str("helper", helper).
				Err(err).
				Msg("Registry-specific credential helper failed")
		}
	}

	if config.CredsStore != "" {
		if err := rv.tryCredentialHelper(ctx, registry, config.CredsStore, authInfo); err == nil {
			return authInfo, nil
		} else {
			rv.logger.Debug().
				Str("registry", registry).
				Str("store", config.CredsStore).
				Err(err).
				Msg("Global credential store failed")
		}
	}

	if err := rv.tryEnvironmentCredentials(registry, authInfo); err == nil {
		return authInfo, nil
	}

	return authInfo, fmt.Errorf("failed to get auth info for registry: %s", registry)
}

// tryCredentialHelper attempts to get credentials using a specific credential helper
func (rv *RegistryValidator) tryCredentialHelper(ctx context.Context, registry, helper string, authInfo *RegistryAuthInfo) error {
	helperName := fmt.Sprintf("docker-credential-%s", helper)

	cmd := exec.CommandContext(ctx, helperName, "get")
	cmd.Stdin = strings.NewReader(registry)

	output, err := cmd.Output()
	if err != nil {
		return err
	}

	var cred struct {
		Username string `json:"Username"`
		Secret   string `json:"Secret"`
	}

	if err := json.Unmarshal(output, &cred); err != nil {
		return err
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
func (rv *RegistryValidator) tryEnvironmentCredentials(registry string, authInfo *RegistryAuthInfo) error {
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

	return fmt.Errorf("failed to get auth info for registry: %s", registry)
}

// testRegistryConnectivity tests connectivity to a registry
func (rv *RegistryValidator) testRegistryConnectivity(ctx context.Context, registry string) error {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	testImages := rv.getTestImagesForRegistry(registry)

	for _, testImage := range testImages {
		cmd := exec.CommandContext(ctx, "docker", "manifest", "inspect", testImage)
		if err := cmd.Run(); err == nil {
			rv.logger.Debug().
				Str("registry", registry).
				Str("test_image", testImage).
				Msg("Registry connectivity test passed")
			return nil
		}
	}

	return fmt.Errorf("registry connectivity test failed: %s", registry)
}

// getTestImagesForRegistry returns appropriate test images for different registries
func (rv *RegistryValidator) getTestImagesForRegistry(registry string) []string {
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
		return []string{
			fmt.Sprintf("%s/hello-world:latest", registry),
			fmt.Sprintf("%s/library/hello-world:latest", registry),
		}
	}
}
