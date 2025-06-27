// Package server implements MCP server tools for registry health checking
package server

import (
	"context"
	"fmt"
	"strings"
	"time"

	coredocker "github.com/Azure/container-kit/pkg/core/docker"
	"github.com/Azure/container-kit/pkg/mcp/internal/types"
	"github.com/rs/zerolog"
)

// CheckRegistryHealthArgs defines arguments for registry health checking
type CheckRegistryHealthArgs struct {
	types.BaseToolArgs

	// Registries to check
	Registries []string `json:"registries,omitempty" jsonschema:"description=List of registries to check (defaults to common registries)"`

	// Check options
	Detailed       bool `json:"detailed,omitempty" jsonschema:"description=Include detailed endpoint checks"`
	IncludeMetrics bool `json:"include_metrics,omitempty" jsonschema:"description=Include historical metrics"`
	ForceRefresh   bool `json:"force_refresh,omitempty" jsonschema:"description=Bypass cache and force new check"`
	Timeout        int  `json:"timeout,omitempty" jsonschema:"description=Timeout in seconds (default: 30)"`
}

// CheckRegistryHealthResult represents registry health check results
type CheckRegistryHealthResult struct {
	types.BaseToolResponse

	// Summary
	AllHealthy   bool          `json:"all_healthy"`
	TotalChecked int           `json:"total_checked"`
	HealthyCount int           `json:"healthy_count"`
	CheckTime    time.Time     `json:"check_time"`
	Duration     time.Duration `json:"duration"`

	// Registry details
	Registries map[string]*coredocker.RegistryHealth `json:"registries"`

	// Quick summary for common registries
	QuickCheck *coredocker.HealthCheckResult `json:"quick_check,omitempty"`

	// Recommendations
	Recommendations []HealthRecommendation `json:"recommendations,omitempty"`
}

// HealthRecommendation provides actionable guidance
type HealthRecommendation struct {
	Registry    string `json:"registry"`
	Priority    int    `json:"priority"`
	Type        string `json:"type"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Action      string `json:"action,omitempty"`
}

// CheckRegistryHealthTool implements registry health checking
type CheckRegistryHealthTool struct {
	logger        zerolog.Logger
	healthChecker *coredocker.RegistryHealthChecker
}

// NewCheckRegistryHealthTool creates a new registry health check tool
func NewCheckRegistryHealthTool(logger zerolog.Logger) *CheckRegistryHealthTool {
	return &CheckRegistryHealthTool{
		logger:        logger.With().Str("tool", "check_registry_health").Logger(),
		healthChecker: coredocker.NewRegistryHealthChecker(logger),
	}
}

// ExecuteTyped performs registry health checks with typed arguments
func (t *CheckRegistryHealthTool) ExecuteTyped(ctx context.Context, args CheckRegistryHealthArgs) (*CheckRegistryHealthResult, error) {
	t.logger.Info().
		Str("session_id", args.SessionID).
		Strs("registries", args.Registries).
		Bool("detailed", args.Detailed).
		Msg("Starting registry health check")

	startTime := time.Now()

	// Create base result
	result := &CheckRegistryHealthResult{
		BaseToolResponse: types.NewBaseResponse("check_registry_health", args.SessionID, args.DryRun),
		CheckTime:        startTime,
		Registries:       make(map[string]*coredocker.RegistryHealth),
		Recommendations:  make([]HealthRecommendation, 0),
	}

	// Handle dry-run
	if args.DryRun {
		result.Duration = time.Since(startTime)
		result.Recommendations = append(result.Recommendations, HealthRecommendation{
			Priority:    1,
			Type:        "dry_run",
			Title:       "Dry Run - Registry Health Check",
			Description: "Would check health of specified registries",
			Action:      "Run without dry_run flag to perform actual health checks",
		})
		return result, nil
	}

	// Set timeout
	timeout := 30 * time.Second
	if args.Timeout > 0 {
		timeout = time.Duration(args.Timeout) * time.Second
	}

	checkCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Default registries if none specified
	registries := args.Registries
	if len(registries) == 0 {
		registries = []string{
			"docker.io",
			"gcr.io",
			"quay.io",
			"ghcr.io",
		}
		t.logger.Info().Strs("registries", registries).Msg("Using default registry list")
	}

	// Force refresh cache if requested
	if args.ForceRefresh {
		// Clear cache by creating new health checker
		t.healthChecker = coredocker.NewRegistryHealthChecker(t.logger)
	}

	// Perform health checks
	if args.Detailed || args.IncludeMetrics {
		// Detailed check for each registry
		healthResults := t.healthChecker.CheckMultipleRegistries(checkCtx, registries)

		for registry, health := range healthResults {
			result.Registries[registry] = health
			result.TotalChecked++
			if health.Healthy {
				result.HealthyCount++
			}

			// Generate recommendations for unhealthy registries
			if !health.Healthy {
				t.generateRecommendations(registry, health, result)
			}
		}

		result.AllHealthy = result.HealthyCount == result.TotalChecked
	} else {
		// Quick check for common registries
		quickResult := t.healthChecker.QuickHealthCheck(checkCtx)
		result.QuickCheck = quickResult
		result.AllHealthy = quickResult.Healthy
		result.TotalChecked = len(quickResult.Registries)

		for _, healthy := range quickResult.Registries {
			if healthy {
				result.HealthyCount++
			}
		}
	}

	// Add general recommendations
	t.addGeneralRecommendations(result)

	result.Duration = time.Since(startTime)

	t.logger.Info().
		Bool("all_healthy", result.AllHealthy).
		Int("total_checked", result.TotalChecked).
		Int("healthy_count", result.HealthyCount).
		Dur("duration", result.Duration).
		Msg("Registry health check completed")

	return result, nil
}

// generateRecommendations creates recommendations for unhealthy registries
func (t *CheckRegistryHealthTool) generateRecommendations(registry string, health *coredocker.RegistryHealth, result *CheckRegistryHealthResult) {
	priority := 1

	// Check connectivity issues
	if !health.Endpoints.Base.Reachable {
		result.Recommendations = append(result.Recommendations, HealthRecommendation{
			Registry:    registry,
			Priority:    priority,
			Type:        "connectivity",
			Title:       fmt.Sprintf("%s is unreachable", registry),
			Description: fmt.Sprintf("Cannot connect to registry: %s", health.Endpoints.Base.Error),
			Action:      "Check network connectivity, firewall rules, and registry URL",
		})
		priority++
	}

	// Check V2 API issues
	if health.Endpoints.Base.Reachable && !health.Endpoints.V2API.Reachable {
		result.Recommendations = append(result.Recommendations, HealthRecommendation{
			Registry:    registry,
			Priority:    priority,
			Type:        "api",
			Title:       fmt.Sprintf("%s V2 API not available", registry),
			Description: "Registry does not support Docker Registry V2 API",
			Action:      "Ensure registry supports Docker Registry V2 API specification",
		})
		priority++
	}

	// Check authentication issues
	if health.Endpoints.V2API.StatusCode == 401 && !health.Endpoints.Auth.Reachable {
		result.Recommendations = append(result.Recommendations, HealthRecommendation{
			Registry:    registry,
			Priority:    priority,
			Type:        "auth",
			Title:       fmt.Sprintf("%s requires authentication", registry),
			Description: "Registry requires authentication but no auth endpoint found",
			Action:      "Configure docker credentials using 'docker login'",
		})
		priority++
	}

	// Check TLS issues
	if health.TLSVersion != "" && strings.Contains(health.TLSVersion, "1.0") {
		result.Recommendations = append(result.Recommendations, HealthRecommendation{
			Registry:    registry,
			Priority:    priority,
			Type:        "security",
			Title:       fmt.Sprintf("%s using outdated TLS version", registry),
			Description: fmt.Sprintf("Registry is using %s which is considered insecure", health.TLSVersion),
			Action:      "Contact registry administrator to upgrade TLS version",
		})
		priority++
	}

	// Check response time issues
	if health.ResponseTime > 5*time.Second {
		result.Recommendations = append(result.Recommendations, HealthRecommendation{
			Registry:    registry,
			Priority:    priority,
			Type:        "performance",
			Title:       fmt.Sprintf("%s has slow response time", registry),
			Description: fmt.Sprintf("Registry responded in %v which may indicate performance issues", health.ResponseTime),
			Action:      "Consider using a registry mirror or CDN for better performance",
		})
	}
}

// addGeneralRecommendations adds general health recommendations
func (t *CheckRegistryHealthTool) addGeneralRecommendations(result *CheckRegistryHealthResult) {
	if result.AllHealthy {
		result.Recommendations = append(result.Recommendations, HealthRecommendation{
			Priority:    1,
			Type:        "success",
			Title:       "All registries healthy",
			Description: fmt.Sprintf("All %d checked registries are healthy and responding", result.TotalChecked),
		})
		return
	}

	// Recommend fallback registries
	if result.HealthyCount > 0 && result.HealthyCount < result.TotalChecked {
		healthyRegistries := []string{}
		if result.QuickCheck != nil {
			for reg, healthy := range result.QuickCheck.Registries {
				if healthy {
					healthyRegistries = append(healthyRegistries, reg)
				}
			}
		} else {
			for reg, health := range result.Registries {
				if health.Healthy {
					healthyRegistries = append(healthyRegistries, reg)
				}
			}
		}

		result.Recommendations = append(result.Recommendations, HealthRecommendation{
			Priority: 1,
			Type:     "fallback",
			Title:    "Use healthy registries as fallback",
			Description: fmt.Sprintf("%d of %d registries are unhealthy. Consider using healthy registries: %s",
				result.TotalChecked-result.HealthyCount, result.TotalChecked, strings.Join(healthyRegistries, ", ")),
		})
	}

	// Recommend monitoring
	if result.TotalChecked > 1 {
		result.Recommendations = append(result.Recommendations, HealthRecommendation{
			Priority:    2,
			Type:        "monitoring",
			Title:       "Set up registry monitoring",
			Description: "Configure monitoring and alerting for registry health to detect issues early",
			Action:      "Use Prometheus metrics endpoint or scheduled health checks",
		})
	}
}
