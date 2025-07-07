package registry

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

	errors "github.com/Azure/container-kit/pkg/mcp/domain/errors"
	"github.com/rs/zerolog"
)

// DockerConfigProvider handles authentication through Docker config and credential helpers
type DockerConfigProvider struct {
	logger     zerolog.Logger
	configPath string
	timeout    time.Duration
}

// DockerConfig represents the structure of Docker's config.json file
type DockerConfig struct {
	Auths             map[string]DockerAuth `json:"auths"`
	CredHelpers       map[string]string     `json:"credHelpers,omitempty"`
	CredsStore        string                `json:"credsStore,omitempty"`
	CredentialHelpers map[string]string     `json:"credentialHelpers,omitempty"`
}

// DockerAuth represents authentication information for a registry
type DockerAuth struct {
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
	Email    string `json:"email,omitempty"`
	Auth     string `json:"auth,omitempty"` // base64 encoded username:password
}

// CredentialHelperResponse represents the response from a credential helper
type CredentialHelperResponse struct {
	Username string `json:"Username"`
	Secret   string `json:"Secret"`
}

// NewDockerConfigProvider creates a new Docker config provider
func NewDockerConfigProvider(logger zerolog.Logger) *DockerConfigProvider {
	homeDir, _ := os.UserHomeDir()
	configPath := filepath.Join(homeDir, ".docker", "config.json")

	return &DockerConfigProvider{
		logger:     logger.With().Str("provider", "docker_config").Logger(),
		configPath: configPath,
		timeout:    30 * time.Second,
	}
}

// GetCredentials retrieves credentials for a registry
func (dcp *DockerConfigProvider) GetCredentials(registryURL string) (*RegistryCredentials, error) {
	// Normalize registry URL for Docker config lookup
	normalizedURL := dcp.normalizeRegistryURL(registryURL)

	dcp.logger.Debug().
		Str("registry", registryURL).
		Str("normalized", normalizedURL).
		Msg("Getting Docker credentials")

	// Load Docker config
	config, err := dcp.loadDockerConfig()
	if err != nil {
		return nil, errors.NewError().Message("failed to load Docker config").Cause(err).WithLocation(

		// Try credential helpers first (higher priority)
		).Build()
	}

	if creds := dcp.tryCredentialHelpers(config, normalizedURL); creds != nil {
		return creds, nil
	}

	// Try direct auth entries
	if creds := dcp.tryDirectAuth(config, normalizedURL); creds != nil {
		return creds, nil
	}

	// Try default credential store
	if config.CredsStore != "" {
		if creds := dcp.tryCredentialStore(config.CredsStore, normalizedURL); creds != nil {
			return creds, nil
		}
	}

	return nil, errors.NewError().Messagef("no Docker credentials found for registry %s", registryURL).WithLocation(

	// IsAvailable checks if Docker config is available
	).Build()
}

func (dcp *DockerConfigProvider) IsAvailable() bool {
	_, err := os.Stat(dcp.configPath)
	return err == nil
}

// GetName returns the provider name
func (dcp *DockerConfigProvider) GetName() string {
	return "docker_config"
}

// GetPriority returns the provider priority (higher = more preferred)
func (dcp *DockerConfigProvider) GetPriority() int {
	return 50 // Medium priority - lower than cloud-specific helpers, higher than basic auth
}

// Supports checks if this provider supports the given registry
func (dcp *DockerConfigProvider) Supports(registryURL string) bool {
	// Docker config provider supports all registries
	return true
}

// Private helper methods

func (dcp *DockerConfigProvider) loadDockerConfig() (*DockerConfig, error) {
	if _, err := os.Stat(dcp.configPath); os.IsNotExist(err) {
		return &DockerConfig{
			Auths:       make(map[string]DockerAuth),
			CredHelpers: make(map[string]string),
		}, nil
	}

	data, err := os.ReadFile(dcp.configPath)
	if err != nil {
		return nil, errors.NewError().Message("failed to read config file").Cause(err).WithLocation().Build()
	}

	var config DockerConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, errors.NewError().Message("failed to parse config JSON").Cause(err).WithLocation(

		// Initialize maps if nil
		).Build()
	}

	if config.Auths == nil {
		config.Auths = make(map[string]DockerAuth)
	}
	if config.CredHelpers == nil {
		config.CredHelpers = make(map[string]string)
	}

	return &config, nil
}

func (dcp *DockerConfigProvider) tryCredentialHelpers(config *DockerConfig, registryURL string) *RegistryCredentials {
	// Check registry-specific credential helpers
	for configRegistry, helper := range config.CredHelpers {
		if dcp.registryMatches(registryURL, configRegistry) {
			dcp.logger.Debug().
				Str("registry", registryURL).
				Str("helper", helper).
				Msg("Trying registry-specific credential helper")

			if creds := dcp.executeCredentialHelper(helper, registryURL); creds != nil {
				return creds
			}
		}
	}

	// Check credential helpers in credentialHelpers field (Docker Desktop format)
	for configRegistry, helper := range config.CredentialHelpers {
		if dcp.registryMatches(registryURL, configRegistry) {
			dcp.logger.Debug().
				Str("registry", registryURL).
				Str("helper", helper).
				Msg("Trying Docker Desktop credential helper")

			if creds := dcp.executeCredentialHelper(helper, registryURL); creds != nil {
				return creds
			}
		}
	}

	return nil
}

func (dcp *DockerConfigProvider) tryDirectAuth(config *DockerConfig, registryURL string) *RegistryCredentials {
	for configRegistry, auth := range config.Auths {
		if dcp.registryMatches(registryURL, configRegistry) {
			dcp.logger.Debug().
				Str("registry", registryURL).
				Str("config_registry", configRegistry).
				Msg("Found direct auth entry")

			// Try to extract credentials from auth field
			if auth.Auth != "" {
				if username, password := dcp.decodeAuth(auth.Auth); username != "" {
					return &RegistryCredentials{
						Username:   username,
						Password:   password,
						Registry:   registryURL,
						AuthMethod: "basic",
					}
				}
			}

			// Try explicit username/password
			if auth.Username != "" && auth.Password != "" {
				return &RegistryCredentials{
					Username:   auth.Username,
					Password:   auth.Password,
					Registry:   registryURL,
					AuthMethod: "basic",
				}
			}
		}
	}

	return nil
}

func (dcp *DockerConfigProvider) tryCredentialStore(store, registryURL string) *RegistryCredentials {
	dcp.logger.Debug().
		Str("registry", registryURL).
		Str("store", store).
		Msg("Trying default credential store")

	return dcp.executeCredentialHelper(store, registryURL)
}

func (dcp *DockerConfigProvider) executeCredentialHelper(helper, registryURL string) *RegistryCredentials {
	// Construct helper command name
	helperCmd := fmt.Sprintf("docker-credential-%s", helper)

	dcp.logger.Debug().
		Str("helper_cmd", helperCmd).
		Str("registry", registryURL).
		Msg("Executing credential helper")

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), dcp.timeout)
	defer cancel()

	// Execute credential helper
	cmd := exec.CommandContext(ctx, helperCmd, "get")
	cmd.Stdin = strings.NewReader(registryURL)

	output, err := cmd.Output()
	if err != nil {
		dcp.logger.Debug().
			Str("helper_cmd", helperCmd).
			Str("registry", registryURL).
			Err(err).
			Msg("Credential helper failed")
		return nil
	}

	// Parse helper response
	var response CredentialHelperResponse
	if err := json.Unmarshal(output, &response); err != nil {
		dcp.logger.Debug().
			Str("helper_cmd", helperCmd).
			Str("registry", registryURL).
			Err(err).
			Msg("Failed to parse credential helper response")
		return nil
	}

	if response.Username == "" && response.Secret == "" {
		return nil
	}

	dcp.logger.Info().
		Str("helper_cmd", helperCmd).
		Str("registry", registryURL).
		Str("username", response.Username).
		Msg("Successfully retrieved credentials from helper")

	return &RegistryCredentials{
		Username:   response.Username,
		Password:   response.Secret,
		Registry:   registryURL,
		AuthMethod: "helper",
	}
}

func (dcp *DockerConfigProvider) normalizeRegistryURL(url string) string {
	// Remove protocol
	url = strings.TrimPrefix(url, "https://")
	url = strings.TrimPrefix(url, "http://")

	// Handle Docker Hub special cases
	switch url {
	case "docker.io", "index.docker.io", "registry-1.docker.io":
		return "https://index.docker.io/v1/"
	}

	// For other registries, try both with and without https prefix
	return url
}

func (dcp *DockerConfigProvider) registryMatches(targetRegistry, configRegistry string) bool {
	// Normalize both URLs for comparison
	target := dcp.normalizeRegistryURL(targetRegistry)
	config := dcp.normalizeRegistryURL(configRegistry)

	// Direct match
	if target == config {
		return true
	}

	// Handle Docker Hub variations
	dockerHubVariations := []string{
		"docker.io",
		"index.docker.io",
		"registry-1.docker.io",
		"https://index.docker.io/v1/",
	}

	targetIsDockerHub := false
	configIsDockerHub := false

	for _, variation := range dockerHubVariations {
		if strings.Contains(target, variation) || target == variation {
			targetIsDockerHub = true
		}
		if strings.Contains(config, variation) || config == variation {
			configIsDockerHub = true
		}
	}

	if targetIsDockerHub && configIsDockerHub {
		return true
	}

	// Check if one is a subdomain/path of the other
	return strings.Contains(target, config) || strings.Contains(config, target)
}

func (dcp *DockerConfigProvider) decodeAuth(auth string) (username, password string) {
	decoded, err := base64.StdEncoding.DecodeString(auth)
	if err != nil {
		return "", ""
	}

	parts := strings.SplitN(string(decoded), ":", 2)
	if len(parts) != 2 {
		return "", ""
	}

	return parts[0], parts[1]
}
