package registry

import (
	"context"
	"encoding/json"
	"os/exec"
	"regexp"
	"strings"
	"time"

	errors "github.com/Azure/container-kit/pkg/mcp/domain/errors"
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
func (acp *AzureCLIProvider) GetCredentials(registryURL string) (*RegistryCredentials, error) {
	if !acp.isAzureRegistry(registryURL) {
		return nil, errors.NewError().Messagef("registry %s is not an Azure Container Registry", registryURL).Build()
	}

	acp.logger.Debug().
		Str("registry", registryURL).
		Msg("Getting Azure CLI credentials")

	registryName := acp.extractRegistryName(registryURL)

	token, err := acp.getAccessToken(registryName)
	if err != nil {
		return nil, errors.NewError().Message("failed to get Azure access token").Cause(err).WithLocation().Build()
	}

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

	return &RegistryCredentials{
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
	if _, err := exec.LookPath("az"); err != nil {
		acp.logger.Debug().Err(err).Msg("Azure CLI not found in PATH")
		return false
	}

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

func (acp *AzureCLIProvider) isAzureRegistry(registryURL string) bool {
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
	url := strings.TrimPrefix(registryURL, "https://")
	url = strings.TrimPrefix(url, "http://")

	re := regexp.MustCompile(`^([^.]+)\.azurecr\.(io|cn|us)`)
	matches := re.FindStringSubmatch(url)
	if len(matches) > 1 {
		return matches[1]
	}

	return url
}

func (acp *AzureCLIProvider) getAccessToken(registryName string) (*AzureTokenResponse, error) {
	ctx, cancel := context.WithTimeout(context.Background(), acp.timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "az", "acr", "get-access-token",
		"--name", registryName,
		"--output", "json")

	output, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			acp.logger.Debug().
				Str("stderr", string(exitErr.Stderr)).
				Msg("az acr get-access-token failed, trying alternative")
		}

		return acp.tryAlternativeLogin(registryName)
	}

	var tokenResponse AzureTokenResponse
	if err := json.Unmarshal(output, &tokenResponse); err != nil {
		return nil, errors.NewError().Message("failed to parse Azure CLI response").Cause(err).WithLocation().Build()
	}

	return &tokenResponse, nil
}

func (acp *AzureCLIProvider) tryAlternativeLogin(registryName string) (*AzureTokenResponse, error) {
	acp.logger.Debug().
		Str("registry_name", registryName).
		Msg("Trying alternative Azure login method")

	ctx, cancel := context.WithTimeout(context.Background(), acp.timeout)
	defer cancel()

	loginCmd := exec.CommandContext(ctx, "az", "acr", "login",
		"--name", registryName,
		"--expose-token",
		"--output", "json")

	output, err := loginCmd.Output()
	if err != nil {
		return nil, errors.NewError().Message("alternative Azure login failed").Cause(err).WithLocation().Build()
	}

	var tokenResponse AzureTokenResponse
	if err := json.Unmarshal(output, &tokenResponse); err != nil {
		return acp.parseAlternativeResponse(output)
	}

	return &tokenResponse, nil
}

func (acp *AzureCLIProvider) parseAlternativeResponse(output []byte) (*AzureTokenResponse, error) {
	var response map[string]interface{}
	if err := json.Unmarshal(output, &response); err != nil {
		return nil, errors.NewError().Message("failed to parse alternative response").Cause(err).WithLocation().Build()
	}

	token := ""
	if accessToken, ok := response["accessToken"].(string); ok {
		token = accessToken
	} else if tokenVal, ok := response["token"].(string); ok {
		token = tokenVal
	}

	if token == "" {
		return nil, errors.NewError().Messagef("no access token found in response").WithLocation().Build()
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
		return "", errors.NewError().Message("failed to get resource group").Cause(err).Build()
	}

	return strings.TrimSpace(string(output)), nil
}

// ValidateAccess tests if the provider can access the registry
func (acp *AzureCLIProvider) ValidateAccess(registryName string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "az", "acr", "repository", "list",
		"--name", registryName,
		"--output", "json")

	if err := cmd.Run(); err != nil {
		return errors.NewError().Message("failed to validate Azure registry access").Cause(err).Build()
	}

	return nil
}
