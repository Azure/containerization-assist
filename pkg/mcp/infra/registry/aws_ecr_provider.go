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

	errors "github.com/Azure/container-kit/pkg/mcp/domain/errors"
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
func (ecp *AWSECRProvider) GetCredentials(registryURL string) (*RegistryCredentials, error) {
	if !ecp.isECRRegistry(registryURL) {
		return nil, errors.NewError().Messagef("registry %s is not an AWS ECR registry", registryURL).Build()
	}

	ecp.logger.Debug().
		Str("registry", registryURL).
		Msg("Getting AWS ECR credentials")

	region, accountID, err := ecp.parseECRURL(registryURL)
	if err != nil {
		return nil, errors.NewError().Message("failed to parse ECR URL").Cause(err).WithLocation().Build()
	}

	authData, err := ecp.getAuthorizationToken(region, accountID)
	if err != nil {
		return nil, errors.NewError().Message("failed to get ECR authorization token").Cause(err).WithLocation().Build()
	}

	username, password, err := ecp.decodeAuthToken(authData.AuthorizationToken)
	if err != nil {
		return nil, errors.NewError().Message("failed to decode authorization token").Cause(err).Build()
	}

	ecp.logger.Info().
		Str("registry", registryURL).
		Str("region", region).
		Str("account_id", accountID).
		Time("expires_at", authData.ExpiresAt).
		Msg("Successfully obtained AWS ECR token")

	return &RegistryCredentials{
		Username:   username,
		Password:   password,
		Registry:   registryURL,
		AuthMethod: "aws_ecr_token",
		ExpiresAt:  &authData.ExpiresAt,
	}, nil
}

// IsAvailable checks if AWS CLI is available and configured
func (ecp *AWSECRProvider) IsAvailable() bool {
	if _, err := exec.LookPath("aws"); err != nil {
		ecp.logger.Debug().Err(err).Msg("AWS CLI not found in PATH")
		return false
	}

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

func (ecp *AWSECRProvider) isECRRegistry(registryURL string) bool {
	// ECR registry URLs follow pattern: {account-id}.dkr.ecr.{region}.amazonaws.com
	ecrPattern := regexp.MustCompile(`\d+\.dkr\.ecr\.[a-z0-9-]+\.amazonaws\.com`)
	return ecrPattern.MatchString(registryURL)
}

func (ecp *AWSECRProvider) parseECRURL(registryURL string) (region, accountID string, err error) {
	url := strings.TrimPrefix(registryURL, "https://")
	url = strings.TrimPrefix(url, "http://")

	ecrPattern := regexp.MustCompile(`^(\d+)\.dkr\.ecr\.([a-z0-9-]+)\.amazonaws\.com`)
	matches := ecrPattern.FindStringSubmatch(url)

	if len(matches) != 3 {
		return "", "", errors.NewError().
			Code(errors.CodeValidationFailed).
			Type(errors.ErrTypeValidation).
			Severity(errors.SeverityMedium).
			Messagef("invalid ECR URL format: %s", registryURL).
			Context("url", registryURL).
			Context("expected_pattern", "{account-id}.dkr.ecr.{region}.amazonaws.com").
			Suggestion("Use a valid AWS ECR URL format").
			WithLocation().
			Build()
	}

	return matches[2], matches[1], nil
}

func (ecp *AWSECRProvider) getAuthorizationToken(region, accountID string) (*ECRAuthData, error) {
	ctx, cancel := context.WithTimeout(context.Background(), ecp.timeout)
	defer cancel()

	args := []string{
		"ecr", "get-authorization-token",
		"--region", region,
		"--output", "json",
	}

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
		return nil, errors.NewError().
			Code(errors.CodeInternal).
			Type(errors.ErrTypeSystem).
			Severity(errors.SeverityHigh).
			Message("aws ecr get-authorization-token failed").
			Context("region", region).
			Context("account_id", accountID).
			Cause(err).
			Suggestion("Check AWS credentials and CLI configuration").
			WithLocation().
			Build()
	}

	var response ECRAuthResponse
	if err := json.Unmarshal(output, &response); err != nil {
		return nil, errors.NewError().
			Code(errors.CodeInternalError).
			Type(errors.ErrTypeSystem).
			Severity(errors.SeverityMedium).
			Message("failed to parse AWS ECR response").
			Cause(err).
			Suggestion("Check AWS CLI version and output format").
			WithLocation().
			Build()
	}

	if len(response.AuthorizationData) == 0 {
		return nil, errors.NewError().
			Code(errors.CodeResourceNotFound).
			Type(errors.ErrTypeResource).
			Severity(errors.SeverityMedium).
			Message("no authorization data returned from AWS ECR").
			Context("region", region).
			Context("account_id", accountID).
			Suggestion("Check AWS permissions and ECR service availability").
			WithLocation().
			Build()
	}

	return &response.AuthorizationData[0], nil
}

func (ecp *AWSECRProvider) decodeAuthToken(token string) (username, password string, err error) {
	decoded, err := base64.StdEncoding.DecodeString(token)
	if err != nil {
		return "", "", errors.NewError().
			Code(errors.CodeInvalidParameter).
			Type(errors.ErrTypeValidation).
			Severity(errors.SeverityMedium).
			Message("failed to decode base64 token").
			Context("token_length", fmt.Sprintf("%d", len(token))).
			Cause(err).
			Suggestion("Check ECR authorization token format").
			WithLocation().
			Build()
	}

	parts := strings.SplitN(string(decoded), ":", 2)
	if len(parts) != 2 {
		return "", "", errors.NewError().
			Code(errors.CodeValidationFailed).
			Type(errors.ErrTypeValidation).
			Severity(errors.SeverityMedium).
			Message("invalid token format").
			Context("parts_count", fmt.Sprintf("%d", len(parts))).
			Context("expected_format", "username:password").
			Suggestion("Ensure ECR token contains username and password separated by colon").
			WithLocation().
			Build()
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
		return nil, errors.NewError().Message("failed to get caller identity").Cause(err).Build()
	}

	var identity map[string]string
	if err := json.Unmarshal(output, &identity); err != nil {
		return nil, errors.NewError().Message("failed to parse caller identity").Cause(err).Build()
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
		return nil, errors.NewError().Message("failed to list ECR repositories").Cause(err).Build()
	}

	var repositories []string
	if err := json.Unmarshal(output, &repositories); err != nil {
		return nil, errors.NewError().Message("failed to parse repositories response").Cause(err).Build()
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
		return errors.NewError().Message("failed to validate ECR access").Cause(err).Build()
	}

	return nil
}

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
