package security

import (
	"context"
	"fmt"
	"net/http"
	"os/exec"
	"time"

	"github.com/rs/zerolog"
)

// TrivyScannerHealthChecker checks the health of Trivy scanner
type TrivyScannerHealthChecker struct {
	logger zerolog.Logger
}

// NewTrivyScannerHealthChecker creates a new Trivy health checker
func NewTrivyScannerHealthChecker(logger zerolog.Logger) *TrivyScannerHealthChecker {
	return &TrivyScannerHealthChecker{
		logger: logger.With().Str("health_checker", "trivy").Logger(),
	}
}

func (t *TrivyScannerHealthChecker) GetName() string {
	return "trivy_scanner"
}

func (t *TrivyScannerHealthChecker) GetDependencies() []string {
	return []string{"filesystem", "network"}
}

func (t *TrivyScannerHealthChecker) CheckHealth(ctx context.Context) ComponentHealth {
	health := ComponentHealth{
		Name:         t.GetName(),
		Status:       HealthStatusHealthy,
		LastChecked:  time.Now(),
		Dependencies: t.GetDependencies(),
	}

	// Check if Trivy is installed
	if _, err := exec.LookPath("trivy"); err != nil {
		health.Status = HealthStatusUnhealthy
		health.Message = "Trivy executable not found in PATH"
		return health
	}

	// Check Trivy version
	cmd := exec.CommandContext(ctx, "trivy", "version")
	output, err := cmd.Output()
	if err != nil {
		health.Status = HealthStatusUnhealthy
		health.Message = fmt.Sprintf("Failed to get Trivy version: %v", err)
		return health
	}

	health.Message = "Trivy scanner is available and responsive"
	if health.Metadata == nil {
		health.Metadata = make(map[string]interface{})
	}
	health.Metadata["version_output"] = string(output)

	return health
}

// GrypeScannerHealthChecker checks the health of Grype scanner
type GrypeScannerHealthChecker struct {
	logger zerolog.Logger
}

// NewGrypeScannerHealthChecker creates a new Grype health checker
func NewGrypeScannerHealthChecker(logger zerolog.Logger) *GrypeScannerHealthChecker {
	return &GrypeScannerHealthChecker{
		logger: logger.With().Str("health_checker", "grype").Logger(),
	}
}

func (g *GrypeScannerHealthChecker) GetName() string {
	return "grype_scanner"
}

func (g *GrypeScannerHealthChecker) GetDependencies() []string {
	return []string{"filesystem", "network"}
}

func (g *GrypeScannerHealthChecker) CheckHealth(ctx context.Context) ComponentHealth {
	health := ComponentHealth{
		Name:         g.GetName(),
		Status:       HealthStatusHealthy,
		LastChecked:  time.Now(),
		Dependencies: g.GetDependencies(),
	}

	// Check if Grype is installed
	if _, err := exec.LookPath("grype"); err != nil {
		health.Status = HealthStatusUnhealthy
		health.Message = "Grype executable not found in PATH"
		return health
	}

	// Check Grype version
	cmd := exec.CommandContext(ctx, "grype", "version")
	output, err := cmd.Output()
	if err != nil {
		health.Status = HealthStatusUnhealthy
		health.Message = fmt.Sprintf("Failed to get Grype version: %v", err)
		return health
	}

	health.Message = "Grype scanner is available and responsive"
	if health.Metadata == nil {
		health.Metadata = make(map[string]interface{})
	}
	health.Metadata["version_output"] = string(output)

	return health
}

// CVEDatabaseHealthChecker checks the health of CVE database integration
type CVEDatabaseHealthChecker struct {
	logger    zerolog.Logger
	cveDB     *CVEDatabase
	testCVEID string
}

// NewCVEDatabaseHealthChecker creates a new CVE database health checker
func NewCVEDatabaseHealthChecker(logger zerolog.Logger, cveDB *CVEDatabase) *CVEDatabaseHealthChecker {
	return &CVEDatabaseHealthChecker{
		logger:    logger.With().Str("health_checker", "cve_database").Logger(),
		cveDB:     cveDB,
		testCVEID: "CVE-2023-1234", // Use a known CVE for testing
	}
}

func (c *CVEDatabaseHealthChecker) GetName() string {
	return "cve_database"
}

func (c *CVEDatabaseHealthChecker) GetDependencies() []string {
	return []string{"network", "nist_nvd_api"}
}

func (c *CVEDatabaseHealthChecker) CheckHealth(ctx context.Context) ComponentHealth {
	health := ComponentHealth{
		Name:         c.GetName(),
		Status:       HealthStatusHealthy,
		LastChecked:  time.Now(),
		Dependencies: c.GetDependencies(),
	}

	if c.cveDB == nil {
		health.Status = HealthStatusUnhealthy
		health.Message = "CVE database client not initialized"
		return health
	}

	// Test CVE database connectivity
	startTime := time.Now()
	_, err := c.cveDB.GetCVE(ctx, c.testCVEID)
	responseTime := time.Since(startTime)

	if err != nil {
		// Check if it's a network error or API error
		if responseTime > 5*time.Second {
			health.Status = HealthStatusDegraded
			health.Message = fmt.Sprintf("CVE database slow response (%v): %v", responseTime, err)
		} else {
			health.Status = HealthStatusDegraded
			health.Message = fmt.Sprintf("CVE database error: %v", err)
		}
	} else {
		health.Message = "CVE database is accessible and responsive"
	}

	// Add metadata
	if health.Metadata == nil {
		health.Metadata = make(map[string]interface{})
	}

	cacheStats := c.cveDB.GetCacheStats()
	health.Metadata["cache_stats"] = cacheStats
	health.Metadata["response_time_ms"] = responseTime.Milliseconds()

	return health
}

// PolicyEngineHealthChecker checks the health of policy engine
type PolicyEngineHealthChecker struct {
	logger       zerolog.Logger
	policyEngine *PolicyEngine
}

// NewPolicyEngineHealthChecker creates a new policy engine health checker
func NewPolicyEngineHealthChecker(logger zerolog.Logger, policyEngine *PolicyEngine) *PolicyEngineHealthChecker {
	return &PolicyEngineHealthChecker{
		logger:       logger.With().Str("health_checker", "policy_engine").Logger(),
		policyEngine: policyEngine,
	}
}

func (p *PolicyEngineHealthChecker) GetName() string {
	return "policy_engine"
}

func (p *PolicyEngineHealthChecker) GetDependencies() []string {
	return []string{}
}

func (p *PolicyEngineHealthChecker) CheckHealth(_ context.Context) ComponentHealth {
	health := ComponentHealth{
		Name:         p.GetName(),
		Status:       HealthStatusHealthy,
		LastChecked:  time.Now(),
		Dependencies: p.GetDependencies(),
	}

	if p.policyEngine == nil {
		health.Status = HealthStatusUnhealthy
		health.Message = "Policy engine not initialized"
		return health
	}

	// Check if policies are loaded
	policies := p.policyEngine.GetPolicies()
	if len(policies) == 0 {
		health.Status = HealthStatusDegraded
		health.Message = "No security policies loaded"
	} else {
		enabledCount := 0
		for _, policy := range policies {
			if policy.Enabled {
				enabledCount++
			}
		}

		if enabledCount == 0 {
			health.Status = HealthStatusDegraded
			health.Message = "No enabled security policies"
		} else {
			health.Message = fmt.Sprintf("Policy engine operational with %d enabled policies", enabledCount)
		}
	}

	// Add metadata
	if health.Metadata == nil {
		health.Metadata = make(map[string]interface{})
	}
	health.Metadata["total_policies"] = len(policies)

	enabledPolicies := 0
	for _, policy := range policies {
		if policy.Enabled {
			enabledPolicies++
		}
	}
	health.Metadata["enabled_policies"] = enabledPolicies

	return health
}

// SecretDiscoveryHealthChecker checks the health of secret discovery engine
type SecretDiscoveryHealthChecker struct {
	logger          zerolog.Logger
	secretDiscovery *SecretDiscovery
}

// NewSecretDiscoveryHealthChecker creates a new secret discovery health checker
func NewSecretDiscoveryHealthChecker(logger zerolog.Logger, secretDiscovery *SecretDiscovery) *SecretDiscoveryHealthChecker {
	return &SecretDiscoveryHealthChecker{
		logger:          logger.With().Str("health_checker", "secret_discovery").Logger(),
		secretDiscovery: secretDiscovery,
	}
}

func (s *SecretDiscoveryHealthChecker) GetName() string {
	return "secret_discovery"
}

func (s *SecretDiscoveryHealthChecker) GetDependencies() []string {
	return []string{"filesystem"}
}

func (s *SecretDiscoveryHealthChecker) CheckHealth(_ context.Context) ComponentHealth {
	health := ComponentHealth{
		Name:         s.GetName(),
		Status:       HealthStatusHealthy,
		LastChecked:  time.Now(),
		Dependencies: s.GetDependencies(),
	}

	if s.secretDiscovery == nil {
		health.Status = HealthStatusUnhealthy
		health.Message = "Secret discovery engine not initialized"
		return health
	}

	// Test secret discovery with pattern detector using a more recognizable pattern
	testContent := "export AWS_ACCESS_KEY_ID=AKIAIOSFODNN7EXAMPLE"
	findings := s.secretDiscovery.patternDetector.Scan(testContent, "test.txt", 1)

	// Should find at least one potential secret
	if len(findings) == 0 {
		health.Status = HealthStatusDegraded
		health.Message = "Secret discovery patterns may not be working correctly"
	} else {
		health.Message = "Secret discovery engine is operational"
	}

	// Add metadata
	if health.Metadata == nil {
		health.Metadata = make(map[string]interface{})
	}
	health.Metadata["test_findings"] = len(findings)

	return health
}

// NetworkConnectivityHealthChecker checks general network connectivity
type NetworkConnectivityHealthChecker struct {
	logger  zerolog.Logger
	testURL string
}

// NewNetworkConnectivityHealthChecker creates a new network connectivity health checker
func NewNetworkConnectivityHealthChecker(logger zerolog.Logger, testURL string) *NetworkConnectivityHealthChecker {
	if testURL == "" {
		testURL = "https://services.nvd.nist.gov/rest/json/cves/2.0"
	}

	return &NetworkConnectivityHealthChecker{
		logger:  logger.With().Str("health_checker", "network_connectivity").Logger(),
		testURL: testURL,
	}
}

func (n *NetworkConnectivityHealthChecker) GetName() string {
	return "network_connectivity"
}

func (n *NetworkConnectivityHealthChecker) GetDependencies() []string {
	return []string{"network", "dns"}
}

func (n *NetworkConnectivityHealthChecker) CheckHealth(ctx context.Context) ComponentHealth {
	health := ComponentHealth{
		Name:         n.GetName(),
		Status:       HealthStatusHealthy,
		LastChecked:  time.Now(),
		Dependencies: n.GetDependencies(),
	}

	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	// Test network connectivity
	req, err := http.NewRequestWithContext(ctx, "HEAD", n.testURL, nil)
	if err != nil {
		health.Status = HealthStatusUnhealthy
		health.Message = fmt.Sprintf("Failed to create request: %v", err)
		return health
	}

	startTime := time.Now()
	resp, err := client.Do(req)
	responseTime := time.Since(startTime)

	if err != nil {
		health.Status = HealthStatusUnhealthy
		health.Message = fmt.Sprintf("Network connectivity test failed: %v", err)
	} else {
		_ = resp.Body.Close()
		if resp.StatusCode >= 200 && resp.StatusCode < 500 {
			health.Message = "Network connectivity is operational"
		} else {
			health.Status = HealthStatusDegraded
			health.Message = fmt.Sprintf("Network test returned status %d", resp.StatusCode)
		}
	}

	// Add metadata
	if health.Metadata == nil {
		health.Metadata = make(map[string]interface{})
	}
	health.Metadata["test_url"] = n.testURL
	health.Metadata["response_time_ms"] = responseTime.Milliseconds()

	return health
}

// SecurityFrameworkHealthChecker provides an aggregate health checker for the entire security framework
type SecurityFrameworkHealthChecker struct {
	logger         zerolog.Logger
	healthMonitor  *HealthMonitor
	requiredChecks []string
}

// NewSecurityFrameworkHealthChecker creates a comprehensive health checker for the security framework
func NewSecurityFrameworkHealthChecker(logger zerolog.Logger, healthMonitor *HealthMonitor) *SecurityFrameworkHealthChecker {
	return &SecurityFrameworkHealthChecker{
		logger:        logger.With().Str("health_checker", "security_framework").Logger(),
		healthMonitor: healthMonitor,
		requiredChecks: []string{
			"trivy_scanner",
			"policy_engine",
			"secret_discovery",
		},
	}
}

func (s *SecurityFrameworkHealthChecker) GetName() string {
	return "security_framework"
}

func (s *SecurityFrameworkHealthChecker) GetDependencies() []string {
	return s.requiredChecks
}

func (s *SecurityFrameworkHealthChecker) CheckHealth(_ context.Context) ComponentHealth {
	health := ComponentHealth{
		Name:         s.GetName(),
		Status:       HealthStatusHealthy,
		LastChecked:  time.Now(),
		Dependencies: s.GetDependencies(),
	}

	// Get current health status
	overallHealth := s.healthMonitor.GetHealth()

	// Check required components
	var unhealthyComponents []string
	var degradedComponents []string

	for _, requiredCheck := range s.requiredChecks {
		if component, exists := overallHealth.Components[requiredCheck]; exists {
			switch component.Status {
			case HealthStatusUnhealthy:
				unhealthyComponents = append(unhealthyComponents, requiredCheck)
			case HealthStatusDegraded:
				degradedComponents = append(degradedComponents, requiredCheck)
			}
		} else {
			unhealthyComponents = append(unhealthyComponents, requiredCheck+" (missing)")
		}
	}

	// Determine overall status
	if len(unhealthyComponents) > 0 {
		health.Status = HealthStatusUnhealthy
		health.Message = fmt.Sprintf("Unhealthy components: %v", unhealthyComponents)
	} else if len(degradedComponents) > 0 {
		health.Status = HealthStatusDegraded
		health.Message = fmt.Sprintf("Degraded components: %v", degradedComponents)
	} else {
		health.Message = "All core security components are healthy"
	}

	// Add metadata
	if health.Metadata == nil {
		health.Metadata = make(map[string]interface{})
	}
	health.Metadata["total_components"] = overallHealth.Summary.TotalComponents
	health.Metadata["healthy_components"] = overallHealth.Summary.HealthyComponents
	health.Metadata["degraded_components"] = overallHealth.Summary.DegradedComponents
	health.Metadata["unhealthy_components"] = overallHealth.Summary.UnhealthyComponents
	health.Metadata["framework_uptime"] = overallHealth.Uptime.String()

	return health
}
