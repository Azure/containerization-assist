package credential_providers

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"github.com/Azure/container-copilot/pkg/mcp/internal/registry"
	"github.com/rs/zerolog"
)

// AzureCLIProvider handles authentication through Azure CLI
type AzureCLIProvider struct {
	logger  zerolog.Logger
	timeout time.Duration
}

// AzureTokenResponse represents the response from az acr get-access-token
type AzureTokenResponse struct {
	AccessToken string `json:"accessToken"`
	LoginServer string `json:"loginServer"`
	ExpiresOn   string `json:"expiresOn"`
}

// NewAzureCLIProvider creates a new Azure CLI provider
func NewAzureCLIProvider(logger zerolog.Logger) *AzureCLIProvider {
	return &AzureCLIProvider{
		logger:  logger.With().Str("provider", "azure_cli").Logger(),
		timeout: 60 * time.Second, // Azure CLI can be slow
	}
}

// GetCredentials retrieves credentials for an Azure Container Registry
func (acp *AzureCLIProvider) GetCredentials(registryURL string) (*registry.RegistryCredentials, error) {
	if !acp.isAzureRegistry(registryURL) {
		return nil, fmt.Errorf("registry %s is not an Azure Container Registry", registryURL)
	}

	acp.logger.Debug().
		Str("registry", registryURL).
		Msg("Getting Azure CLI credentials")

	// Extract registry name from URL
	registryName := acp.extractRegistryName(registryURL)

	// Try to get access token
	token, err := acp.getAccessToken(registryName)
	if err != nil {
		return nil, fmt.Errorf("failed to get Azure access token: %w", err)
	}

	// Parse expiration time
	var expiresAt *time.Time
	if token.ExpiresOn != "" {
		if exp, err := time.Parse(time.RFC3339, token.ExpiresOn); err == nil {
			expiresAt = &exp
		}
	}

	acp.logger.Info().
		Str("registry", registryURL).
		Str("registry_name", registryName).
		Msg("Successfully obtained Azure CLI token")

	return &registry.RegistryCredentials{
		Username:   "00000000-0000-0000-0000-000000000000", // Azure uses a fixed GUID for ACR token auth
		Password:   token.AccessToken,
		Token:      token.AccessToken,
		Registry:   registryURL,
		AuthMethod: "azure_token",
		ExpiresAt:  expiresAt,
	}, nil
}

// IsAvailable checks if Azure CLI is available and logged in
func (acp *AzureCLIProvider) IsAvailable() bool {
	// Check if az command exists
	if _, err := exec.LookPath("az"); err != nil {
		acp.logger.Debug().Err(err).Msg("Azure CLI not found in PATH")
		return false
	}

	// Check if user is logged in
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "az", "account", "show", "--output", "json")
	if err := cmd.Run(); err != nil {
		acp.logger.Debug().Err(err).Msg("Azure CLI not logged in")
		return false
	}

	return true
}

// GetName returns the provider name
func (acp *AzureCLIProvider) GetName() string {
	return "azure_cli"
}

// GetPriority returns the provider priority
func (acp *AzureCLIProvider) GetPriority() int {
	return 80 // High priority for Azure registries
}

// Supports checks if this provider supports the given registry
func (acp *AzureCLIProvider) Supports(registryURL string) bool {
	return acp.isAzureRegistry(registryURL)
}

// Private helper methods

func (acp *AzureCLIProvider) isAzureRegistry(registryURL string) bool {
	// Check if URL contains Azure Container Registry patterns
	azurePatterns := []string{
		".azurecr.io",
		".azurecr.cn", // Azure China
		".azurecr.us", // Azure Government
	}

	url := strings.ToLower(registryURL)
	for _, pattern := range azurePatterns {
		if strings.Contains(url, pattern) {
			return true
		}
	}

	return false
}

func (acp *AzureCLIProvider) extractRegistryName(registryURL string) string {
	// Remove protocol
	url := strings.TrimPrefix(registryURL, "https://")
	url = strings.TrimPrefix(url, "http://")

	// Extract registry name (part before .azurecr.io)
	re := regexp.MustCompile(`^([^.]+)\.azurecr\.(io|cn|us)`)
	matches := re.FindStringSubmatch(url)
	if len(matches) > 1 {
		return matches[1]
	}

	// Fallback: use the full URL
	return url
}

func (acp *AzureCLIProvider) getAccessToken(registryName string) (*AzureTokenResponse, error) {
	ctx, cancel := context.WithTimeout(context.Background(), acp.timeout)
	defer cancel()

	// Execute az acr get-access-token command
	cmd := exec.CommandContext(ctx, "az", "acr", "get-access-token",
		"--name", registryName,
		"--output", "json")

	output, err := cmd.Output()
	if err != nil {
		// Try alternative command for older Azure CLI versions
		if exitErr, ok := err.(*exec.ExitError); ok {
			acp.logger.Debug().
				Str("stderr", string(exitErr.Stderr)).
				Msg("az acr get-access-token failed, trying alternative")
		}

		return acp.tryAlternativeLogin(registryName)
	}

	var tokenResponse AzureTokenResponse
	if err := json.Unmarshal(output, &tokenResponse); err != nil {
		return nil, fmt.Errorf("failed to parse Azure CLI response: %w", err)
	}

	return &tokenResponse, nil
}

func (acp *AzureCLIProvider) tryAlternativeLogin(registryName string) (*AzureTokenResponse, error) {
	acp.logger.Debug().
		Str("registry_name", registryName).
		Msg("Trying alternative Azure login method")

	ctx, cancel := context.WithTimeout(context.Background(), acp.timeout)
	defer cancel()

	// Try az acr login which might work for older CLI versions
	loginCmd := exec.CommandContext(ctx, "az", "acr", "login",
		"--name", registryName,
		"--expose-token",
		"--output", "json")

	output, err := loginCmd.Output()
	if err != nil {
		return nil, fmt.Errorf("alternative Azure login failed: %w", err)
	}

	// Try to parse as token response
	var tokenResponse AzureTokenResponse
	if err := json.Unmarshal(output, &tokenResponse); err != nil {
		// If parsing fails, try to extract token from different format
		return acp.parseAlternativeResponse(output)
	}

	return &tokenResponse, nil
}

func (acp *AzureCLIProvider) parseAlternativeResponse(output []byte) (*AzureTokenResponse, error) {
	// Parse alternative response formats that might be returned by older Azure CLI
	var response map[string]interface{}
	if err := json.Unmarshal(output, &response); err != nil {
		return nil, fmt.Errorf("failed to parse alternative response: %w", err)
	}

	token := ""
	if accessToken, ok := response["accessToken"].(string); ok {
		token = accessToken
	} else if tokenVal, ok := response["token"].(string); ok {
		token = tokenVal
	}

	if token == "" {
		return nil, fmt.Errorf("no access token found in response")
	}

	return &AzureTokenResponse{
		AccessToken: token,
		ExpiresOn:   "", // May not be available in alternative format
	}, nil
}

// GetResourceGroupName attempts to get the resource group for a registry
func (acp *AzureCLIProvider) GetResourceGroupName(registryName string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "az", "acr", "show",
		"--name", registryName,
		"--query", "resourceGroup",
		"--output", "tsv")

	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get resource group: %w", err)
	}

	return strings.TrimSpace(string(output)), nil
}

// ValidateAccess tests if the provider can access the registry
func (acp *AzureCLIProvider) ValidateAccess(registryName string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Try to list repositories to validate access
	cmd := exec.CommandContext(ctx, "az", "acr", "repository", "list",
		"--name", registryName,
		"--output", "json")

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to validate Azure registry access: %w", err)
	}

	return nil
}
