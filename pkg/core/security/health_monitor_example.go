package security

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/rs/zerolog"
)

// ExampleHealthMonitor demonstrates how to set up and use the health monitoring system
func ExampleHealthMonitor() {
	// Create logger
	logger := zerolog.New(zerolog.NewConsoleWriter()).With().Timestamp().Logger()

	// Create health monitor
	monitor := NewHealthMonitor(logger)
	monitor.SetVersion("1.0.0")
	monitor.SetCheckInterval(30 * time.Second)

	// Create and register health checkers
	trivyChecker := NewTrivyScannerHealthChecker(logger)
	monitor.RegisterChecker(trivyChecker)

	grypeChecker := NewGrypeScannerHealthChecker(logger)
	monitor.RegisterChecker(grypeChecker)

	// Create policy engine and register its health checker
	policyEngine := NewPolicyEngine(logger)
	err := policyEngine.LoadDefaultPolicies()
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to load default policies")
	}

	policyChecker := NewPolicyEngineHealthChecker(logger, policyEngine)
	monitor.RegisterChecker(policyChecker)

	// Create secret discovery and register its health checker
	secretDiscovery := NewSecretDiscovery(logger)
	secretChecker := NewSecretDiscoveryHealthChecker(logger, secretDiscovery)
	monitor.RegisterChecker(secretChecker)

	// Create CVE database and register its health checker
	cveDB := NewCVEDatabase(logger, "") // Empty API key for demo
	cveChecker := NewCVEDatabaseHealthChecker(logger, cveDB)
	monitor.RegisterChecker(cveChecker)

	// Register network connectivity checker
	networkChecker := NewNetworkConnectivityHealthChecker(logger, "")
	monitor.RegisterChecker(networkChecker)

	// Register aggregate framework health checker
	frameworkChecker := NewSecurityFrameworkHealthChecker(logger, monitor)
	monitor.RegisterChecker(frameworkChecker)

	// Start health monitoring
	ctx := context.Background()
	err = monitor.Start(ctx)
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to start health monitor")
	}

	// Create HTTP endpoints
	handler := NewHealthEndpointHandler(monitor, logger)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	// Start HTTP server for health endpoints
	server := &http.Server{
		Addr:              ":8080",
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}

	logger.Info().Msg("Health monitoring started")
	logger.Info().Msg("Health endpoints available at:")
	logger.Info().Msg("  - http://localhost:8080/healthz")
	logger.Info().Msg("  - http://localhost:8080/readyz")
	logger.Info().Msg("  - http://localhost:8080/livez")

	// In a real application, you would run this in a goroutine
	// and handle graceful shutdown
	if err := server.ListenAndServe(); err != nil {
		logger.Fatal().Err(err).Msg("HTTP server failed")
	}
}

// ExampleHealthCheck demonstrates manual health checking
func ExampleHealthCheck() {
	logger := zerolog.New(zerolog.NewConsoleWriter()).With().Timestamp().Logger()

	// Create health monitor
	monitor := NewHealthMonitor(logger)

	// Register some checkers
	trivyChecker := NewTrivyScannerHealthChecker(logger)
	monitor.RegisterChecker(trivyChecker)

	policyEngine := NewPolicyEngine(logger)
	if err := policyEngine.LoadDefaultPolicies(); err != nil {
		logger.Warn().Err(err).Msg("Failed to load default policies")
	}
	policyChecker := NewPolicyEngineHealthChecker(logger, policyEngine)
	monitor.RegisterChecker(policyChecker)

	// Perform manual health check
	ctx := context.Background()
	monitor.checkAllHealth(ctx)

	// Get health status
	health := monitor.GetHealth()

	fmt.Printf("Overall Status: %s\n", health.Status)
	fmt.Printf("Components: %d total, %d healthy, %d degraded, %d unhealthy\n",
		health.Summary.TotalComponents,
		health.Summary.HealthyComponents,
		health.Summary.DegradedComponents,
		health.Summary.UnhealthyComponents)

	// Print component details
	for name, component := range health.Components {
		fmt.Printf("Component %s: %s - %s\n", name, component.Status, component.Message)
	}

	// Check readiness
	readiness := monitor.GetReadiness()
	fmt.Printf("Ready: %t\n", readiness.Ready)
	if !readiness.Ready {
		fmt.Printf("Not ready reasons: %s\n", readiness.Message)
	}
}

// ExampleHealthCheckIntegration demonstrates integration with existing applications
func ExampleHealthCheckIntegration() {
	logger := zerolog.New(zerolog.NewConsoleWriter()).With().Timestamp().Logger()

	// Create your security components
	policyEngine := NewPolicyEngine(logger)
	if err := policyEngine.LoadDefaultPolicies(); err != nil {
		logger.Warn().Err(err).Msg("Failed to load default policies")
	}

	secretDiscovery := NewSecretDiscovery(logger)
	cveDB := NewCVEDatabase(logger, "") // Empty API key for demo

	// Create health monitor
	monitor := NewHealthMonitor(logger)
	monitor.SetVersion("myapp-1.2.3")

	// Register health checkers for your components
	monitor.RegisterChecker(NewTrivyScannerHealthChecker(logger))
	monitor.RegisterChecker(NewGrypeScannerHealthChecker(logger))
	monitor.RegisterChecker(NewPolicyEngineHealthChecker(logger, policyEngine))
	monitor.RegisterChecker(NewSecretDiscoveryHealthChecker(logger, secretDiscovery))
	monitor.RegisterChecker(NewCVEDatabaseHealthChecker(logger, cveDB))
	monitor.RegisterChecker(NewNetworkConnectivityHealthChecker(logger, ""))

	// Start monitoring
	ctx := context.Background()
	if err := monitor.Start(ctx); err != nil {
		logger.Warn().Err(err).Msg("Failed to start health monitor")
	}

	// Integrate with your existing HTTP server
	existingMux := http.NewServeMux()

	// Add your existing routes
	existingMux.HandleFunc("/api/scan", func(w http.ResponseWriter, _ *http.Request) {
		// Your scan endpoint logic
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte("Scan endpoint")); err != nil {
			logger.Error().Err(err).Msg("Failed to write response")
		}
	})

	// Add health endpoints
	handler := NewHealthEndpointHandler(monitor, logger)
	handler.RegisterRoutes(existingMux)

	// Your application can also check health status before processing requests
	existingMux.HandleFunc("/api/scan-with-health-check", func(w http.ResponseWriter, _ *http.Request) {
		// Check if system is ready before processing
		readiness := monitor.GetReadiness()
		if !readiness.Ready {
			http.Error(w, "Service not ready: "+readiness.Message, http.StatusServiceUnavailable)
			return
		}

		// Process the request
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte("Scan completed")); err != nil {
			logger.Error().Err(err).Msg("Failed to write response")
		}
	})

	logger.Info().Msg("Application with health monitoring started")
}

// CustomServiceHealthChecker demonstrates creating a custom health checker
type CustomServiceHealthChecker struct {
	logger      zerolog.Logger
	serviceName string
	serviceURL  string
}

// NewCustomServiceHealthChecker creates a new custom service health checker
func NewCustomServiceHealthChecker(logger zerolog.Logger, serviceName, serviceURL string) *CustomServiceHealthChecker {
	return &CustomServiceHealthChecker{
		logger:      logger.With().Str("health_checker", "custom_service").Logger(),
		serviceName: serviceName,
		serviceURL:  serviceURL,
	}
}

// GetName returns the name of the custom service health checker
func (c *CustomServiceHealthChecker) GetName() string {
	return c.serviceName
}

// GetDependencies returns the dependencies for the custom service
func (c *CustomServiceHealthChecker) GetDependencies() []string {
	return []string{"network", "external_service"}
}

// CheckHealth performs a health check for the custom service
func (c *CustomServiceHealthChecker) CheckHealth(ctx context.Context) ComponentHealth {
	health := ComponentHealth{
		Name:         c.GetName(),
		Status:       HealthStatusHealthy,
		LastChecked:  time.Now(),
		Dependencies: c.GetDependencies(),
	}

	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	// Check service health
	req, err := http.NewRequestWithContext(ctx, "GET", c.serviceURL+"/health", nil)
	if err != nil {
		health.Status = HealthStatusUnhealthy
		health.Message = fmt.Sprintf("Failed to create request: %v", err)
		return health
	}

	resp, err := client.Do(req)
	if err != nil {
		health.Status = HealthStatusUnhealthy
		health.Message = fmt.Sprintf("Service unreachable: %v", err)
		return health
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusOK {
		health.Message = "Service is healthy"
	} else {
		health.Status = HealthStatusDegraded
		health.Message = fmt.Sprintf("Service returned status %d", resp.StatusCode)
	}

	// Add metadata
	if health.Metadata == nil {
		health.Metadata = make(map[string]interface{})
	}
	health.Metadata["service_url"] = c.serviceURL
	health.Metadata["response_status"] = resp.StatusCode

	return health
}

// ExampleCustomHealthChecker demonstrates using a custom health checker
func ExampleCustomHealthChecker() {
	logger := zerolog.New(zerolog.NewConsoleWriter()).With().Timestamp().Logger()

	// Create health monitor
	monitor := NewHealthMonitor(logger)

	// Register standard checkers
	monitor.RegisterChecker(NewTrivyScannerHealthChecker(logger))

	// Register custom checker
	customChecker := NewCustomServiceHealthChecker(logger, "my_service", "http://my-service:8080")
	monitor.RegisterChecker(customChecker)

	// Start monitoring
	ctx := context.Background()
	if err := monitor.Start(ctx); err != nil {
		logger.Warn().Err(err).Msg("Failed to start health monitor")
	}

	logger.Info().Msg("Health monitoring with custom checker started")
}
