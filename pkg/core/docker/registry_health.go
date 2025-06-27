package docker

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog"
)

// RegistryHealthChecker provides comprehensive registry health checking
type RegistryHealthChecker struct {
	logger     zerolog.Logger
	httpClient *http.Client
	cache      *healthCache
}

// NewRegistryHealthChecker creates a new registry health checker
func NewRegistryHealthChecker(logger zerolog.Logger) *RegistryHealthChecker {
	return &RegistryHealthChecker{
		logger: logger.With().Str("component", "registry_health").Logger(),
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{
					InsecureSkipVerify: false,
					MinVersion:         tls.VersionTLS12,
				},
				DialContext: (&net.Dialer{
					Timeout:   5 * time.Second,
					KeepAlive: 30 * time.Second,
				}).DialContext,
				MaxIdleConns:          100,
				IdleConnTimeout:       90 * time.Second,
				TLSHandshakeTimeout:   5 * time.Second,
				ExpectContinueTimeout: 1 * time.Second,
			},
		},
		cache: newHealthCache(5 * time.Minute),
	}
}

// RegistryHealth represents the health status of a registry
type RegistryHealth struct {
	Registry     string         `json:"registry"`
	Healthy      bool           `json:"healthy"`
	CheckTime    time.Time      `json:"check_time"`
	ResponseTime time.Duration  `json:"response_time_ms"`
	APIVersion   string         `json:"api_version,omitempty"`
	TLSVersion   string         `json:"tls_version,omitempty"`
	Endpoints    EndpointHealth `json:"endpoints"`
	Metrics      HealthMetrics  `json:"metrics"`
	Errors       []string       `json:"errors,omitempty"`
	Capabilities []string       `json:"capabilities,omitempty"`
}

// EndpointHealth tracks health of individual registry endpoints
type EndpointHealth struct {
	Base    EndpointStatus `json:"base"`
	V2API   EndpointStatus `json:"v2_api"`
	Catalog EndpointStatus `json:"catalog,omitempty"`
	Auth    EndpointStatus `json:"auth,omitempty"`
}

// EndpointStatus represents the status of a single endpoint
type EndpointStatus struct {
	URL          string        `json:"url"`
	Reachable    bool          `json:"reachable"`
	StatusCode   int           `json:"status_code,omitempty"`
	ResponseTime time.Duration `json:"response_time_ms"`
	Error        string        `json:"error,omitempty"`
}

// HealthMetrics provides detailed health metrics
type HealthMetrics struct {
	SuccessRate  float64       `json:"success_rate"`
	AvgResponse  time.Duration `json:"avg_response_ms"`
	P95Response  time.Duration `json:"p95_response_ms"`
	P99Response  time.Duration `json:"p99_response_ms"`
	TotalChecks  int64         `json:"total_checks"`
	FailedChecks int64         `json:"failed_checks"`
	LastSuccess  *time.Time    `json:"last_success,omitempty"`
	LastFailure  *time.Time    `json:"last_failure,omitempty"`
}

// CheckRegistryHealth performs a comprehensive health check on a registry
func (rhc *RegistryHealthChecker) CheckRegistryHealth(ctx context.Context, registry string) (*RegistryHealth, error) {
	// Check cache first
	if cached := rhc.cache.get(registry); cached != nil {
		rhc.logger.Debug().Str("registry", registry).Msg("Returning cached health status")
		return cached, nil
	}

	startTime := time.Now()
	health := &RegistryHealth{
		Registry:  registry,
		CheckTime: startTime,
		Endpoints: EndpointHealth{},
		Metrics:   HealthMetrics{},
		Errors:    make([]string, 0),
	}

	rhc.logger.Info().Str("registry", registry).Msg("Starting registry health check")

	// Normalize registry URL
	baseURL := rhc.normalizeRegistryURL(registry)

	// Check base connectivity
	rhc.checkBaseConnectivity(ctx, baseURL, health)

	// Check V2 API
	rhc.checkV2API(ctx, baseURL, health)

	// Check catalog endpoint (if accessible)
	rhc.checkCatalogEndpoint(ctx, baseURL, health)

	// Check auth endpoint
	rhc.checkAuthEndpoint(ctx, baseURL, health)

	// Determine TLS version
	rhc.checkTLSVersion(baseURL, health)

	// Calculate overall health
	health.ResponseTime = time.Since(startTime)
	health.Healthy = rhc.calculateOverallHealth(health)

	// Update metrics
	rhc.updateMetrics(registry, health)

	// Cache result
	rhc.cache.set(registry, health)

	rhc.logger.Info().
		Str("registry", registry).
		Bool("healthy", health.Healthy).
		Dur("response_time", health.ResponseTime).
		Msg("Registry health check completed")

	return health, nil
}

// CheckMultipleRegistries checks health of multiple registries concurrently
func (rhc *RegistryHealthChecker) CheckMultipleRegistries(ctx context.Context, registries []string) map[string]*RegistryHealth {
	results := make(map[string]*RegistryHealth)
	var mu sync.Mutex
	var wg sync.WaitGroup

	for _, registry := range registries {
		wg.Add(1)
		go func(reg string) {
			defer wg.Done()

			health, err := rhc.CheckRegistryHealth(ctx, reg)
			if err != nil {
				health = &RegistryHealth{
					Registry: reg,
					Healthy:  false,
					Errors:   []string{err.Error()},
				}
			}

			mu.Lock()
			results[reg] = health
			mu.Unlock()
		}(registry)
	}

	wg.Wait()
	return results
}

// normalizeRegistryURL ensures the registry URL is properly formatted
func (rhc *RegistryHealthChecker) normalizeRegistryURL(registry string) string {
	// Handle special cases
	if registry == "docker.io" || registry == "index.docker.io" {
		return "https://registry-1.docker.io"
	}

	// Add https:// if no protocol specified
	if !strings.HasPrefix(registry, "http://") && !strings.HasPrefix(registry, "https://") {
		registry = "https://" + registry
	}

	// Remove trailing slash
	return strings.TrimSuffix(registry, "/")
}

// checkBaseConnectivity checks basic connectivity to the registry
func (rhc *RegistryHealthChecker) checkBaseConnectivity(ctx context.Context, baseURL string, health *RegistryHealth) {
	endpoint := &health.Endpoints.Base
	endpoint.URL = baseURL

	startTime := time.Now()
	req, err := http.NewRequestWithContext(ctx, "GET", baseURL, nil)
	if err != nil {
		endpoint.Error = fmt.Sprintf("failed to create request: %v", err)
		health.Errors = append(health.Errors, endpoint.Error)
		return
	}

	resp, err := rhc.httpClient.Do(req)
	endpoint.ResponseTime = time.Since(startTime)

	if err != nil {
		endpoint.Error = fmt.Sprintf("connection failed: %v", err)
		health.Errors = append(health.Errors, endpoint.Error)
		return
	}
	defer func() { _ = resp.Body.Close() }()

	endpoint.Reachable = true
	endpoint.StatusCode = resp.StatusCode
}

// checkV2API checks the Docker Registry V2 API endpoint
func (rhc *RegistryHealthChecker) checkV2API(ctx context.Context, baseURL string, health *RegistryHealth) {
	endpoint := &health.Endpoints.V2API
	endpoint.URL = baseURL + "/v2/"

	startTime := time.Now()
	req, err := http.NewRequestWithContext(ctx, "GET", endpoint.URL, nil)
	if err != nil {
		endpoint.Error = fmt.Sprintf("failed to create request: %v", err)
		health.Errors = append(health.Errors, endpoint.Error)
		return
	}

	resp, err := rhc.httpClient.Do(req)
	endpoint.ResponseTime = time.Since(startTime)

	if err != nil {
		endpoint.Error = fmt.Sprintf("V2 API check failed: %v", err)
		health.Errors = append(health.Errors, endpoint.Error)
		return
	}
	defer func() { _ = resp.Body.Close() }()

	endpoint.Reachable = true
	endpoint.StatusCode = resp.StatusCode

	// Check for API version header
	if apiVersion := resp.Header.Get("Docker-Distribution-Api-Version"); apiVersion != "" {
		health.APIVersion = apiVersion
	}

	// Registry is considered to support V2 if we get 200 or 401 (auth required)
	if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusUnauthorized {
		health.Capabilities = append(health.Capabilities, "v2-api")
	}
}

// checkCatalogEndpoint checks if the catalog endpoint is accessible
func (rhc *RegistryHealthChecker) checkCatalogEndpoint(ctx context.Context, baseURL string, health *RegistryHealth) {
	endpoint := &health.Endpoints.Catalog
	endpoint.URL = baseURL + "/v2/_catalog?n=1"

	startTime := time.Now()
	req, err := http.NewRequestWithContext(ctx, "GET", endpoint.URL, nil)
	if err != nil {
		endpoint.Error = fmt.Sprintf("failed to create request: %v", err)
		return
	}

	resp, err := rhc.httpClient.Do(req)
	endpoint.ResponseTime = time.Since(startTime)

	if err != nil {
		endpoint.Error = fmt.Sprintf("catalog check failed: %v", err)
		return
	}
	defer func() { _ = resp.Body.Close() }()

	endpoint.Reachable = true
	endpoint.StatusCode = resp.StatusCode

	// Catalog is accessible if we get 200 (some registries don't support catalog)
	if resp.StatusCode == http.StatusOK {
		health.Capabilities = append(health.Capabilities, "catalog")
	}
}

// checkAuthEndpoint checks authentication mechanisms
func (rhc *RegistryHealthChecker) checkAuthEndpoint(ctx context.Context, baseURL string, health *RegistryHealth) {
	// First, try to access a protected resource to get auth challenge
	testURL := baseURL + "/v2/"
	req, err := http.NewRequestWithContext(ctx, "GET", testURL, nil)
	if err != nil {
		return
	}

	resp, err := rhc.httpClient.Do(req)
	if err != nil {
		return
	}
	defer func() { _ = resp.Body.Close() }()

	// Check for WWW-Authenticate header
	if authHeader := resp.Header.Get("WWW-Authenticate"); authHeader != "" {
		health.Endpoints.Auth.Reachable = true

		// Parse auth type
		if strings.HasPrefix(authHeader, "Bearer") {
			health.Capabilities = append(health.Capabilities, "bearer-auth")

			// Extract realm from Bearer challenge
			if realm := extractRealm(authHeader); realm != "" {
				health.Endpoints.Auth.URL = realm
			}
		} else if strings.HasPrefix(authHeader, "Basic") {
			health.Capabilities = append(health.Capabilities, "basic-auth")
		}
	}
}

// checkTLSVersion determines the TLS version used by the registry
func (rhc *RegistryHealthChecker) checkTLSVersion(baseURL string, health *RegistryHealth) {
	u, err := url.Parse(baseURL)
	if err != nil || u.Scheme != "https" {
		return
	}

	// nolint:gosec // InsecureSkipVerify is intentional for registry health checks to test connectivity
	conn, err := tls.Dial("tcp", u.Host+":443", &tls.Config{
		InsecureSkipVerify: true, // This is for health check connectivity testing only
		MinVersion:         tls.VersionTLS12,
	})
	if err != nil {
		return
	}
	defer func() { _ = conn.Close() }()

	state := conn.ConnectionState()
	switch state.Version {
	case tls.VersionTLS10:
		health.TLSVersion = "TLS 1.0"
	case tls.VersionTLS11:
		health.TLSVersion = "TLS 1.1"
	case tls.VersionTLS12:
		health.TLSVersion = "TLS 1.2"
	case tls.VersionTLS13:
		health.TLSVersion = "TLS 1.3"
	}
}

// calculateOverallHealth determines if the registry is healthy
func (rhc *RegistryHealthChecker) calculateOverallHealth(health *RegistryHealth) bool {
	// Registry is healthy if base is reachable and V2 API responds
	return health.Endpoints.Base.Reachable &&
		health.Endpoints.V2API.Reachable &&
		(health.Endpoints.V2API.StatusCode == http.StatusOK ||
			health.Endpoints.V2API.StatusCode == http.StatusUnauthorized)
}

// updateMetrics updates health metrics for the registry
func (rhc *RegistryHealthChecker) updateMetrics(registry string, health *RegistryHealth) {
	metrics := rhc.cache.getMetrics(registry)
	if metrics == nil {
		metrics = &HealthMetrics{}
	}

	metrics.TotalChecks++
	if health.Healthy {
		now := time.Now()
		metrics.LastSuccess = &now
	} else {
		metrics.FailedChecks++
		now := time.Now()
		metrics.LastFailure = &now
	}

	// Calculate success rate
	if metrics.TotalChecks > 0 {
		metrics.SuccessRate = float64(metrics.TotalChecks-metrics.FailedChecks) / float64(metrics.TotalChecks) * 100
	}

	health.Metrics = *metrics
	rhc.cache.setMetrics(registry, metrics)
}

// GetHealthSummary returns a summary of all cached registry health statuses
func (rhc *RegistryHealthChecker) GetHealthSummary() map[string]*RegistryHealth {
	return rhc.cache.getAll()
}

// extractRealm extracts the realm URL from a Bearer auth challenge
func extractRealm(authHeader string) string {
	parts := strings.Split(authHeader, ",")
	for _, part := range parts {
		if strings.Contains(part, "realm=") {
			realm := strings.Split(part, "realm=")[1]
			return strings.Trim(realm, `"`)
		}
	}
	return ""
}

// healthCache provides caching for health check results
type healthCache struct {
	mu      sync.RWMutex
	cache   map[string]*cacheEntry
	metrics map[string]*HealthMetrics
	ttl     time.Duration
}

type cacheEntry struct {
	health    *RegistryHealth
	expiresAt time.Time
}

func newHealthCache(ttl time.Duration) *healthCache {
	hc := &healthCache{
		cache:   make(map[string]*cacheEntry),
		metrics: make(map[string]*HealthMetrics),
		ttl:     ttl,
	}

	// Start cleanup goroutine
	go hc.cleanup()

	return hc
}

func (hc *healthCache) get(registry string) *RegistryHealth {
	hc.mu.RLock()
	defer hc.mu.RUnlock()

	entry, ok := hc.cache[registry]
	if !ok || time.Now().After(entry.expiresAt) {
		return nil
	}

	return entry.health
}

func (hc *healthCache) set(registry string, health *RegistryHealth) {
	hc.mu.Lock()
	defer hc.mu.Unlock()

	hc.cache[registry] = &cacheEntry{
		health:    health,
		expiresAt: time.Now().Add(hc.ttl),
	}
}

func (hc *healthCache) getAll() map[string]*RegistryHealth {
	hc.mu.RLock()
	defer hc.mu.RUnlock()

	results := make(map[string]*RegistryHealth)
	now := time.Now()

	for registry, entry := range hc.cache {
		if now.Before(entry.expiresAt) {
			results[registry] = entry.health
		}
	}

	return results
}

func (hc *healthCache) getMetrics(registry string) *HealthMetrics {
	hc.mu.RLock()
	defer hc.mu.RUnlock()

	return hc.metrics[registry]
}

func (hc *healthCache) setMetrics(registry string, metrics *HealthMetrics) {
	hc.mu.Lock()
	defer hc.mu.Unlock()

	hc.metrics[registry] = metrics
}

func (hc *healthCache) cleanup() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		hc.mu.Lock()
		now := time.Now()

		for registry, entry := range hc.cache {
			if now.After(entry.expiresAt) {
				delete(hc.cache, registry)
			}
		}

		hc.mu.Unlock()
	}
}

// HealthCheckResult provides a simplified health check result for API responses
type HealthCheckResult struct {
	Healthy      bool            `json:"healthy"`
	Registries   map[string]bool `json:"registries"`
	ResponseTime time.Duration   `json:"response_time_ms"`
	CheckedAt    time.Time       `json:"checked_at"`
}

// QuickHealthCheck performs a quick health check on common registries
func (rhc *RegistryHealthChecker) QuickHealthCheck(ctx context.Context) *HealthCheckResult {
	commonRegistries := []string{
		"docker.io",
		"gcr.io",
		"quay.io",
	}

	startTime := time.Now()
	results := rhc.CheckMultipleRegistries(ctx, commonRegistries)

	healthMap := make(map[string]bool)
	allHealthy := true

	for registry, health := range results {
		healthMap[registry] = health.Healthy
		if !health.Healthy {
			allHealthy = false
		}
	}

	return &HealthCheckResult{
		Healthy:      allHealthy,
		Registries:   healthMap,
		ResponseTime: time.Since(startTime),
		CheckedAt:    startTime,
	}
}
