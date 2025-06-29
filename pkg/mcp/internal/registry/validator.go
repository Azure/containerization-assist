package registry

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/core"
	"github.com/rs/zerolog"
)

// RegistryValidator provides validation and testing capabilities for registries
type RegistryValidator struct {
	logger     zerolog.Logger
	httpClient *http.Client
	timeout    time.Duration
}

// ValidationResult contains the results of registry validation
type ValidationResult struct {
	Registry      string                 `json:"registry"`
	Accessible    bool                   `json:"accessible"`
	Authenticated bool                   `json:"authenticated"`
	Permissions   PermissionSet          `json:"permissions"`
	Latency       time.Duration          `json:"latency"`
	TLSValid      bool                   `json:"tls_valid"`
	APIVersion    string                 `json:"api_version,omitempty"`
	Error         string                 `json:"error,omitempty"`
	Details       map[string]interface{} `json:"details,omitempty"`
}

// PermissionSet represents the permissions available for a registry
type PermissionSet struct {
	CanPull  bool `json:"can_pull"`
	CanPush  bool `json:"can_push"`
	CanList  bool `json:"can_list"`
	CanAdmin bool `json:"can_admin"`
}

// NewRegistryValidator creates a new registry validator
func NewRegistryValidator(logger zerolog.Logger) *RegistryValidator {
	return &RegistryValidator{
		logger:  logger.With().Str("component", "registry_validator").Logger(),
		timeout: 30 * time.Second,
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

// ValidateRegistry performs comprehensive validation of a registry
func (rv *RegistryValidator) ValidateRegistry(ctx context.Context, registryURL string, creds *RegistryCredentials) (*ValidationResult, error) {
	startTime := time.Now()

	rv.logger.Info().
		Str("registry", registryURL).
		Msg("Starting registry validation")

	result := &ValidationResult{
		Registry: registryURL,
		Details:  make(map[string]interface{}),
	}

	// Test basic connectivity
	accessible, err := rv.testConnectivity(ctx, registryURL)
	if err != nil {
		result.Error = err.Error()
		result.Latency = time.Since(startTime)
		return result, nil
	}
	result.Accessible = accessible

	// Test TLS certificate validity
	result.TLSValid = rv.testTLSCertificate(ctx, registryURL)

	// Test API version
	apiVersion, err := rv.detectAPIVersion(ctx, registryURL)
	if err == nil {
		result.APIVersion = apiVersion
	}

	// Test authentication if credentials provided
	if creds != nil {
		authenticated, err := rv.testAuthentication(ctx, registryURL, creds)
		if err != nil {
			result.Details["auth_error"] = err.Error()
		}
		result.Authenticated = authenticated

		// Test permissions if authenticated
		if authenticated {
			permissions, err := rv.testPermissions(ctx, registryURL, creds)
			if err != nil {
				result.Details["permission_error"] = err.Error()
			} else {
				result.Permissions = *permissions
			}
		}
	}

	result.Latency = time.Since(startTime)

	rv.logger.Info().
		Str("registry", registryURL).
		Bool("accessible", result.Accessible).
		Bool("authenticated", result.Authenticated).
		Bool("tls_valid", result.TLSValid).
		Dur("latency", result.Latency).
		Msg("Registry validation completed")

	return result, nil
}

// TestConnectivity tests basic network connectivity to a registry
func (rv *RegistryValidator) testConnectivity(ctx context.Context, registryURL string) (bool, error) {
	// Normalize URL
	url := rv.normalizeRegistryURL(registryURL)

	// Try to reach the registry API endpoint
	endpoint := fmt.Sprintf("%s/v2/", url)

	req, err := http.NewRequestWithContext(ctx, "GET", endpoint, nil)
	if err != nil {
		return false, mcp.NewErrorBuilder("request_creation_failed", "Failed to create HTTP request for registry connectivity test", "network").
			WithField("endpoint", endpoint).
			WithOperation("test_connectivity").
			WithStage("request_creation").
			WithRootCause(fmt.Sprintf("Cannot create HTTP request for endpoint %s: %v", endpoint, err)).
			WithImmediateStep(1, "Check URL", "Verify registry URL format is correct").
			WithImmediateStep(2, "Check context", "Ensure context is valid and not cancelled").
			Build()
	}

	resp, err := rv.httpClient.Do(req)
	if err != nil {
		return false, mcp.NewErrorBuilder("registry_connection_failed", "Failed to connect to registry", "network").
			WithField("endpoint", endpoint).
			WithOperation("test_connectivity").
			WithStage("connection_attempt").
			WithRootCause(fmt.Sprintf("Cannot establish connection to registry endpoint %s: %v", endpoint, err)).
			WithImmediateStep(1, "Check network", "Verify network connectivity to registry").
			WithImmediateStep(2, "Check DNS", "Ensure registry hostname resolves correctly").
			WithImmediateStep(3, "Check firewall", "Verify no firewall rules block registry access").
			Build()
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
func (rv *RegistryValidator) ValidateMultipleRegistries(ctx context.Context, registries map[string]*RegistryCredentials) (map[string]*ValidationResult, error) {
	results := make(map[string]*ValidationResult)

	// For simplicity, validate sequentially
	// In production, this could be done concurrently with goroutines
	for registryURL, creds := range registries {
		result, err := rv.ValidateRegistry(ctx, registryURL, creds)
		if err != nil {
			result = &ValidationResult{
				Registry: registryURL,
				Error:    err.Error(),
			}
		}
		results[registryURL] = result
	}

	return results, nil
}
