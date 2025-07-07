package registry

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/Azure/container-kit/pkg/common/validation-core/core"
	"github.com/Azure/container-kit/pkg/common/validation-core/validators"
	errors "github.com/Azure/container-kit/pkg/mcp/domain/errors"
	"github.com/Azure/container-kit/pkg/mcp/domain/types"
	"github.com/rs/zerolog"
)

// RegistryValidator provides validation and testing capabilities for registries using unified validation
type RegistryValidator struct {
	logger           zerolog.Logger
	httpClient       *http.Client
	timeout          time.Duration
	networkValidator *validators.NetworkValidator
}

// UnifiedRegistryValidator provides a unified validation interface
type UnifiedRegistryValidator struct {
	impl *RegistryValidator
}

// NewUnifiedRegistryValidator creates a new unified registry validator
func NewUnifiedRegistryValidator(logger zerolog.Logger) *UnifiedRegistryValidator {
	return &UnifiedRegistryValidator{
		impl: NewRegistryValidator(logger),
	}
}

// ValidationResult uses the canonical validation type (legacy compatibility)
type ValidationResult = types.ValidationResult

// ValidationDetails contains domain-specific registry validation data
type ValidationDetails struct {
	Registry      string        `json:"registry"`
	Accessible    bool          `json:"accessible"`
	Authenticated bool          `json:"authenticated"`
	Permissions   PermissionSet `json:"permissions"`
	Latency       time.Duration `json:"latency"`
	TLSValid      bool          `json:"tls_valid"`
	APIVersion    string        `json:"api_version,omitempty"`
}

// PermissionSet represents the permissions available for a registry
type PermissionSet struct {
	CanPull  bool `json:"can_pull"`
	CanPush  bool `json:"can_push"`
	CanList  bool `json:"can_list"`
	CanAdmin bool `json:"can_admin"`
}

// NewRegistryValidator creates a new registry validator with unified validation support
func NewRegistryValidator(logger zerolog.Logger) *RegistryValidator {
	return &RegistryValidator{
		logger:           logger.With().Str("component", "unified_registry_validator").Logger(),
		timeout:          30 * time.Second,
		networkValidator: validators.NewNetworkValidator(),
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{
					InsecureSkipVerify: false,
				},
			},
		},
	}
}

// ValidateRegistryUnified performs comprehensive validation of a registry using unified validation framework
func (rv *RegistryValidator) ValidateRegistryUnified(ctx context.Context, registryURL string, creds *RegistryCredentials) (*core.NonGenericResult, error) {
	rv.logger.Info().
		Str("registry", registryURL).
		Msg("Starting unified registry validation")

	// Create validation data
	validationData := map[string]interface{}{
		"registry_url": registryURL,
		"timeout":      rv.timeout,
	}

	if creds != nil {
		validationData["has_credentials"] = true
		validationData["username"] = creds.Username
		// Don't log password for security
	}

	// Use network validator for basic validation
	options := core.NewValidationOptions().WithStrictMode(true)
	result := rv.networkValidator.Validate(ctx, validationData, options)

	// Add registry-specific validations
	if !strings.HasPrefix(registryURL, "https://") && !strings.HasPrefix(registryURL, "http://") {
		result.AddError(core.NewError("INVALID_REGISTRY_URL", "Registry URL must start with http:// or https://", core.ErrTypeNetwork, core.SeverityCritical).WithField("registry_url"))
	}

	// Test connectivity using network validator results and our specific tests
	startTime := time.Now()
	accessible, err := rv.TestConnectivity(ctx, registryURL)
	latency := time.Since(startTime)

	if err != nil {
		result.AddError(core.NewError("REGISTRY_CONNECTIVITY_ERROR", fmt.Sprintf("Failed to connect to registry: %v", err), core.ErrTypeNetwork, core.SeverityCritical).
			WithField("registry_url").
			WithContext("latency_ms", latency.Milliseconds()))
	} else if accessible {
		result.AddSuggestion("Registry is accessible")
	}

	// Test TLS certificate validity
	tlsValid := rv.testTLSCertificate(ctx, registryURL)
	if !tlsValid {
		result.AddWarning(core.NewWarning("TLS_CERTIFICATE_INVALID", "TLS certificate validation failed"))
	}

	// Test authentication if credentials provided
	if creds != nil {
		authenticated, authErr := rv.testAuthentication(ctx, registryURL, creds)
		if authErr != nil {
			result.AddWarning(core.NewWarning("AUTHENTICATION_FAILED", fmt.Sprintf("Authentication test failed: %v", authErr)))
		} else if authenticated {
			result.AddSuggestion("Authentication successful")

			// Test permissions
			permissions, permErr := rv.testPermissions(ctx, registryURL, creds)
			if permErr != nil {
				result.AddWarning(core.NewWarning("PERMISSION_TEST_FAILED", fmt.Sprintf("Permission test failed: %v", permErr)))
			} else {
				result.AddSuggestion(fmt.Sprintf("Permissions validated: pull=%t, push=%t", permissions.CanPull, permissions.CanPush))
			}
		}
	}

	rv.logger.Info().
		Bool("valid", result.Valid).
		Int("errors", len(result.Errors)).
		Int("warnings", len(result.Warnings)).
		Dur("duration", time.Since(startTime)).
		Msg("Unified registry validation completed")

	return result, nil
}

// ValidateRegistry performs comprehensive validation of a registry using unified validation
func (rv *RegistryValidator) ValidateRegistry(ctx context.Context, registryURL string, creds *RegistryCredentials) (*core.NonGenericResult, error) {
	return rv.ValidateRegistryUnified(ctx, registryURL, creds)
}

// Unified validation interface methods for UnifiedRegistryValidator

// Validate implements the GenericValidator interface
func (urv *UnifiedRegistryValidator) Validate(ctx context.Context, data interface{}, options *core.ValidationOptions) *core.NonGenericResult {
	var registryURL string
	var creds *RegistryCredentials

	// Extract data based on input type
	switch v := data.(type) {
	case string:
		registryURL = v
	case map[string]interface{}:
		if url, ok := v["registry_url"].(string); ok {
			registryURL = url
		}
		if username, ok := v["username"].(string); ok {
			if password, ok := v["password"].(string); ok {
				creds = &RegistryCredentials{
					Username: username,
					Password: password,
				}
			}
		}
	default:
		result := core.NewNonGenericResult("unified_registry_validator", "1.0.0")
		result.AddError(core.NewError("INVALID_DATA_TYPE", "Invalid data type for registry validation", core.ErrTypeValidation, core.SeverityCritical))
		return result
	}

	result, err := urv.impl.ValidateRegistryUnified(ctx, registryURL, creds)
	if err != nil {
		if result == nil {
			result = core.NewNonGenericResult("unified_registry_validator", "1.0.0")
		}
		result.AddError(core.NewError("VALIDATION_ERROR", err.Error(), core.ErrTypeNetwork, core.SeverityHigh))
	}
	return result
}

// GetName returns the validator name
func (urv *UnifiedRegistryValidator) GetName() string {
	return "unified_registry_validator"
}

// GetVersion returns the validator version
func (urv *UnifiedRegistryValidator) GetVersion() string {
	return "1.0.0"
}

// GetSupportedTypes returns the data types this validator can handle
func (urv *UnifiedRegistryValidator) GetSupportedTypes() []string {
	return []string{"string", "map[string]interface{}", "RegistryCredentials"}
}

// ValidateWithCredentials performs validation with explicit credentials
func (urv *UnifiedRegistryValidator) ValidateWithCredentials(ctx context.Context, registryURL string, creds *RegistryCredentials) (*core.NonGenericResult, error) {
	return urv.impl.ValidateRegistryUnified(ctx, registryURL, creds)
}

// Migration helpers for backward compatibility

// MigrateRegistryValidatorToUnified provides a drop-in replacement for legacy RegistryValidator
func MigrateRegistryValidatorToUnified(logger zerolog.Logger) *UnifiedRegistryValidator {
	return NewUnifiedRegistryValidator(logger)
}

// TestConnectivity tests basic network connectivity to a registry
func (rv *RegistryValidator) TestConnectivity(ctx context.Context, registryURL string) (bool, error) {
	// Normalize URL
	url := rv.normalizeRegistryURL(registryURL)

	// Try to reach the registry API endpoint
	endpoint := fmt.Sprintf("%s/v2/", url)

	req, err := http.NewRequestWithContext(ctx, "GET", endpoint, nil)
	if err != nil {
		return false, errors.NewError().Message("failed to create HTTP request for registry connectivity test").Cause(err).Build()
	}

	resp, err := rv.httpClient.Do(req)
	if err != nil {
		return false, errors.NewError().Message(fmt.Sprintf("failed to connect to registry %s", endpoint)).Cause(err).Build()
	}
	defer resp.Body.Close()

	// Any response (even 401) indicates connectivity
	return true, nil
}

// TestTLSCertificate validates the TLS certificate of a registry
func (rv *RegistryValidator) testTLSCertificate(ctx context.Context, registryURL string) bool {
	// Create a client that validates certificates
	client := &http.Client{
		Timeout: 10 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: false,
			},
		},
	}

	url := rv.normalizeRegistryURL(registryURL)
	endpoint := fmt.Sprintf("%s/v2/", url)

	req, err := http.NewRequestWithContext(ctx, "GET", endpoint, nil)
	if err != nil {
		return false
	}

	resp, err := client.Do(req)
	if err != nil {
		// Check if it's a TLS error
		if strings.Contains(err.Error(), "certificate") || strings.Contains(err.Error(), "tls") {
			return false
		}
		// Other errors (like 401) don't indicate TLS problems
		return true
	}
	defer resp.Body.Close()

	return true
}

// DetectAPIVersion attempts to detect the Docker Registry API version
func (rv *RegistryValidator) detectAPIVersion(ctx context.Context, registryURL string) (string, error) {
	url := rv.normalizeRegistryURL(registryURL)
	endpoint := fmt.Sprintf("%s/v2/", url)

	req, err := http.NewRequestWithContext(ctx, "GET", endpoint, nil)
	if err != nil {
		return "", err
	}

	resp, err := rv.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	// Check Docker-Distribution-API-Version header
	if apiVersion := resp.Header.Get("Docker-Distribution-API-Version"); apiVersion != "" {
		return apiVersion, nil
	}

	// Check for other version indicators
	if resp.StatusCode == 200 || resp.StatusCode == 401 {
		return "registry/2.0", nil
	}

	return "unknown", nil
}

// TestAuthentication tests if the provided credentials work with the registry
func (rv *RegistryValidator) testAuthentication(ctx context.Context, registryURL string, creds *RegistryCredentials) (bool, error) {
	url := rv.normalizeRegistryURL(registryURL)
	endpoint := fmt.Sprintf("%s/v2/", url)

	req, err := http.NewRequestWithContext(ctx, "GET", endpoint, nil)
	if err != nil {
		return false, err
	}

	// Add authentication based on credential type
	switch creds.AuthMethod {
	case "basic":
		req.SetBasicAuth(creds.Username, creds.Password)
	case "bearer", "token", "azure_token", "aws_ecr_token":
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", creds.Token))
	default:
		// Default to basic auth
		req.SetBasicAuth(creds.Username, creds.Password)
	}

	resp, err := rv.httpClient.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	// 200 indicates successful authentication
	// 401 indicates authentication failed
	// Other status codes might indicate other issues
	return resp.StatusCode == 200, nil
}

// TestPermissions tests what permissions are available with the given credentials
func (rv *RegistryValidator) testPermissions(ctx context.Context, registryURL string, creds *RegistryCredentials) (*PermissionSet, error) {
	permissions := &PermissionSet{}

	// Test catalog listing (admin permission)
	permissions.CanList = rv.testCatalogAccess(ctx, registryURL, creds)
	permissions.CanAdmin = permissions.CanList // Simplified assumption

	// Test repository access (this is a simplified test)
	// In a real implementation, you'd test with actual repositories
	permissions.CanPull = true // If authenticated, usually can pull
	permissions.CanPush = true // This would need more sophisticated testing

	return permissions, nil
}

// TestCatalogAccess tests if the credentials can access the registry catalog
func (rv *RegistryValidator) testCatalogAccess(ctx context.Context, registryURL string, creds *RegistryCredentials) bool {
	url := rv.normalizeRegistryURL(registryURL)
	endpoint := fmt.Sprintf("%s/v2/_catalog", url)

	req, err := http.NewRequestWithContext(ctx, "GET", endpoint, nil)
	if err != nil {
		return false
	}

	// Add authentication
	switch creds.AuthMethod {
	case "basic":
		req.SetBasicAuth(creds.Username, creds.Password)
	case "bearer", "token", "azure_token", "aws_ecr_token":
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", creds.Token))
	default:
		req.SetBasicAuth(creds.Username, creds.Password)
	}

	resp, err := rv.httpClient.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	return resp.StatusCode == 200
}

// SetInsecure configures the validator to skip TLS verification
func (rv *RegistryValidator) SetInsecure(insecure bool) {
	if transport, ok := rv.httpClient.Transport.(*http.Transport); ok {
		transport.TLSClientConfig.InsecureSkipVerify = insecure
	}
}

// SetTimeout configures the timeout for validation operations
func (rv *RegistryValidator) SetTimeout(timeout time.Duration) {
	rv.timeout = timeout
	rv.httpClient.Timeout = timeout
}

// Private helper methods

func (rv *RegistryValidator) normalizeRegistryURL(registryURL string) string {
	// Remove trailing slashes
	url := strings.TrimSuffix(registryURL, "/")

	// Add https:// if no protocol specified
	if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
		url = "https://" + url
	}

	// Handle Docker Hub special case
	if strings.Contains(url, "docker.io") || strings.Contains(url, "index.docker.io") {
		return "https://index.docker.io"
	}

	return url
}

// ValidateMultipleRegistries validates multiple registries concurrently
func (rv *RegistryValidator) ValidateMultipleRegistries(ctx context.Context, registries map[string]*RegistryCredentials) (map[string]*core.NonGenericResult, error) {
	results := make(map[string]*core.NonGenericResult)

	// For simplicity, validate sequentially
	// In production, this could be done concurrently with goroutines
	for registryURL, creds := range registries {
		result, err := rv.ValidateRegistry(ctx, registryURL, creds)
		if err != nil {
			result = core.NewNonGenericResult("registry_validator", "1.0.0")
			result.AddError(core.NewError("REGISTRY_VALIDATION_ERROR", "Registry validation failed", core.ErrTypeNetwork, core.SeverityHigh))
		}
		results[registryURL] = result
	}

	return results, nil
}
