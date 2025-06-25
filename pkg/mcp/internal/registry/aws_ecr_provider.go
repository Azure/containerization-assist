package registry

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"github.com/Azure/container-copilot/pkg/mcp/internal/registry"
	"github.com/rs/zerolog"
)

// AWSECRProvider handles authentication through AWS CLI for ECR
type AWSECRProvider struct {
	logger  zerolog.Logger
	timeout time.Duration
}

// ECRAuthResponse represents the response from aws ecr get-authorization-token
type ECRAuthResponse struct {
	AuthorizationData []ECRAuthData `json:"authorizationData"`
}

// ECRAuthData contains the authorization data from ECR
type ECRAuthData struct {
	AuthorizationToken string    `json:"authorizationToken"`
	ExpiresAt          time.Time `json:"expiresAt"`
	ProxyEndpoint      string    `json:"proxyEndpoint"`
}

// NewAWSECRProvider creates a new AWS ECR provider
func NewAWSECRProvider(logger zerolog.Logger) *AWSECRProvider {
	return &AWSECRProvider{
		logger:  logger.With().Str("provider", "aws_ecr").Logger(),
		timeout: 60 * time.Second,
	}
}

// GetCredentials retrieves credentials for an AWS ECR registry
func (ecp *AWSECRProvider) GetCredentials(registryURL string) (*registry.RegistryCredentials, error) {
	if !ecp.isECRRegistry(registryURL) {
		return nil, fmt.Errorf("registry %s is not an AWS ECR registry", registryURL)
	}

	ecp.logger.Debug().
		Str("registry", registryURL).
		Msg("Getting AWS ECR credentials")

	// Extract region and account ID from registry URL
	region, accountID, err := ecp.parseECRURL(registryURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse ECR URL: %w", err)
	}

	// Get authorization token
	authData, err := ecp.getAuthorizationToken(region, accountID)
	if err != nil {
		return nil, fmt.Errorf("failed to get ECR authorization token: %w", err)
	}

	// Decode the authorization token
	username, password, err := ecp.decodeAuthToken(authData.AuthorizationToken)
	if err != nil {
		return nil, fmt.Errorf("failed to decode authorization token: %w", err)
	}

	ecp.logger.Info().
		Str("registry", registryURL).
		Str("region", region).
		Str("account_id", accountID).
		Time("expires_at", authData.ExpiresAt).
		Msg("Successfully obtained AWS ECR token")

	return &registry.RegistryCredentials{
		Username:   username,
		Password:   password,
		Registry:   registryURL,
		AuthMethod: "aws_ecr_token",
		ExpiresAt:  &authData.ExpiresAt,
	}, nil
}

// IsAvailable checks if AWS CLI is available and configured
func (ecp *AWSECRProvider) IsAvailable() bool {
	// Check if aws command exists
	if _, err := exec.LookPath("aws"); err != nil {
		ecp.logger.Debug().Err(err).Msg("AWS CLI not found in PATH")
		return false
	}

	// Check if AWS credentials are configured
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "aws", "sts", "get-caller-identity", "--output", "json")
	if err := cmd.Run(); err != nil {
		ecp.logger.Debug().Err(err).Msg("AWS CLI not configured or credentials invalid")
		return false
	}

	return true
}

// GetName returns the provider name
func (ecp *AWSECRProvider) GetName() string {
	return "aws_ecr"
}

// GetPriority returns the provider priority
func (ecp *AWSECRProvider) GetPriority() int {
	return 80 // High priority for ECR registries
}

// Supports checks if this provider supports the given registry
func (ecp *AWSECRProvider) Supports(registryURL string) bool {
	return ecp.isECRRegistry(registryURL)
}

// Private helper methods

func (ecp *AWSECRProvider) isECRRegistry(registryURL string) bool {
	// ECR registry URLs follow pattern: {account-id}.dkr.ecr.{region}.amazonaws.com
	ecrPattern := regexp.MustCompile(`\d+\.dkr\.ecr\.[a-z0-9-]+\.amazonaws\.com`)
	return ecrPattern.MatchString(registryURL)
}

func (ecp *AWSECRProvider) parseECRURL(registryURL string) (region, accountID string, err error) {
	// Remove protocol
	url := strings.TrimPrefix(registryURL, "https://")
	url = strings.TrimPrefix(url, "http://")

	// Parse ECR URL: {account-id}.dkr.ecr.{region}.amazonaws.com
	ecrPattern := regexp.MustCompile(`^(\d+)\.dkr\.ecr\.([a-z0-9-]+)\.amazonaws\.com`)
	matches := ecrPattern.FindStringSubmatch(url)

	if len(matches) != 3 {
		return "", "", fmt.Errorf("invalid ECR URL format: %s", registryURL)
	}

	return matches[2], matches[1], nil // region, accountID
}

func (ecp *AWSECRProvider) getAuthorizationToken(region, accountID string) (*ECRAuthData, error) {
	ctx, cancel := context.WithTimeout(context.Background(), ecp.timeout)
	defer cancel()

	// Construct AWS CLI command
	args := []string{
		"ecr", "get-authorization-token",
		"--region", region,
		"--output", "json",
	}

	// Add registry IDs if account ID is available
	if accountID != "" {
		args = append(args, "--registry-ids", accountID)
	}

	cmd := exec.CommandContext(ctx, "aws", args...)

	output, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			ecp.logger.Debug().
				Str("stderr", string(exitErr.Stderr)).
				Str("region", region).
				Str("account_id", accountID).
				Msg("AWS ECR get-authorization-token failed")
		}
		return nil, fmt.Errorf("aws ecr get-authorization-token failed: %w", err)
	}

	var response ECRAuthResponse
	if err := json.Unmarshal(output, &response); err != nil {
		return nil, fmt.Errorf("failed to parse AWS ECR response: %w", err)
	}

	if len(response.AuthorizationData) == 0 {
		return nil, fmt.Errorf("no authorization data returned from AWS ECR")
	}

	return &response.AuthorizationData[0], nil
}

func (ecp *AWSECRProvider) decodeAuthToken(token string) (username, password string, err error) {
	decoded, err := base64.StdEncoding.DecodeString(token)
	if err != nil {
		return "", "", fmt.Errorf("failed to decode base64 token: %w", err)
	}

	parts := strings.SplitN(string(decoded), ":", 2)
	if len(parts) != 2 {
		return "", "", fmt.Errorf("invalid token format")
	}

	return parts[0], parts[1], nil
}

// GetCallerIdentity returns information about the AWS caller
func (ecp *AWSECRProvider) GetCallerIdentity() (map[string]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "aws", "sts", "get-caller-identity", "--output", "json")

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get caller identity: %w", err)
	}

	var identity map[string]string
	if err := json.Unmarshal(output, &identity); err != nil {
		return nil, fmt.Errorf("failed to parse caller identity: %w", err)
	}

	return identity, nil
}

// GetECRRepositories lists repositories in the ECR registry
func (ecp *AWSECRProvider) GetECRRepositories(region string) ([]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "aws", "ecr", "describe-repositories",
		"--region", region,
		"--query", "repositories[].repositoryName",
		"--output", "json")

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to list ECR repositories: %w", err)
	}

	var repositories []string
	if err := json.Unmarshal(output, &repositories); err != nil {
		return nil, fmt.Errorf("failed to parse repositories response: %w", err)
	}

	return repositories, nil
}

// ValidateAccess tests if the provider can access the ECR registry
func (ecp *AWSECRProvider) ValidateAccess(region, accountID string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Try to describe repositories to validate access
	args := []string{
		"ecr", "describe-repositories",
		"--region", region,
		"--max-items", "1",
		"--output", "json",
	}

	if accountID != "" {
		args = append(args, "--registry-id", accountID)
	}

	cmd := exec.CommandContext(ctx, args[0], args[1:]...)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to validate ECR access: %w", err)
	}

	return nil
}

// GetRegionFromRegistry extracts the AWS region from an ECR registry URL
func (ecp *AWSECRProvider) GetRegionFromRegistry(registryURL string) string {
	region, _, err := ecp.parseECRURL(registryURL)
	if err != nil {
		return ""
	}
	return region
}

// GetAccountIDFromRegistry extracts the AWS account ID from an ECR registry URL
func (ecp *AWSECRProvider) GetAccountIDFromRegistry(registryURL string) string {
	_, accountID, err := ecp.parseECRURL(registryURL)
	if err != nil {
		return ""
	}
	return accountID
}
