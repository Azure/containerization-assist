// Package security provides Prometheus metrics for the security scanning framework
package security

import (
	"fmt"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/zerolog"
)

// ScanResultInterface defines the interface for scan results
type ScanResultInterface interface {
	GetSuccess() bool
	GetImageRef() string
	GetScanTime() time.Time
	GetDuration() time.Duration
	GetSummary() VulnerabilitySummaryInterface
	GetVulnerabilities() []VulnerabilityInterface
}

// UnifiedScanResultInterface defines the interface for unified scan results
type UnifiedScanResultInterface interface {
	GetSuccess() bool
	GetImageRef() string
	GetScanTime() time.Time
	GetDuration() time.Duration
	GetCombinedSummary() VulnerabilitySummaryInterface
	GetComparisonMetrics() ComparisonMetricsInterface
	GetTrivyResult() ScanResultInterface
	GetGrypeResult() ScanResultInterface
}

// VulnerabilitySummaryInterface defines the interface for vulnerability summaries
type VulnerabilitySummaryInterface interface {
	GetTotal() int
	GetCritical() int
	GetHigh() int
	GetMedium() int
	GetLow() int
	GetFixable() int
}

// VulnerabilityInterface defines the interface for vulnerabilities
type VulnerabilityInterface interface {
	GetCVSS() CVSSInterface
	GetCVSSV3() CVSSV3Interface
}

// CVSSInterface defines the interface for CVSS information
type CVSSInterface interface {
	GetVersion() string
	GetScore() float64
}

// CVSSV3Interface defines the interface for CVSS v3 information
type CVSSV3Interface interface {
	GetScore() float64
}

// ComparisonMetricsInterface defines the interface for comparison metrics
type ComparisonMetricsInterface interface {
	GetAgreementRate() float64
}

// MetricsCollector collects and exposes security scanning metrics
type MetricsCollector struct {
	logger   zerolog.Logger
	registry *prometheus.Registry

	// Scan metrics
	scanDuration *prometheus.HistogramVec
	scanTotal    *prometheus.CounterVec
	scanErrors   *prometheus.CounterVec
	lastScanTime *prometheus.GaugeVec

	// Vulnerability metrics
	vulnerabilitiesTotal      *prometheus.GaugeVec
	vulnerabilitiesBySeverity *prometheus.GaugeVec
	vulnerabilitiesFixed      *prometheus.CounterVec
	cvssScores                *prometheus.HistogramVec

	// Policy metrics
	policyEvaluations  *prometheus.CounterVec
	policyViolations   *prometheus.CounterVec
	blockedDeployments *prometheus.CounterVec
	policyExecution    *prometheus.HistogramVec

	// Secret detection metrics
	secretsFound       *prometheus.CounterVec
	secretTypes        *prometheus.CounterVec
	falsePositives     *prometheus.CounterVec
	secretScanDuration *prometheus.HistogramVec

	// CVE database metrics
	cveQueries     *prometheus.CounterVec
	cveCacheHits   *prometheus.CounterVec
	cveCacheMisses *prometheus.CounterVec
	cveAPILatency  *prometheus.HistogramVec

	// Health metrics
	componentHealth     *prometheus.GaugeVec
	healthCheckDuration *prometheus.HistogramVec
	healthChecksTotal   *prometheus.CounterVec

	// Scanner-specific metrics
	scannerAvailability *prometheus.GaugeVec
	scannerAgreement    *prometheus.GaugeVec
	scannerPerformance  *prometheus.HistogramVec

	// Registry metrics
	registryHealth  *prometheus.GaugeVec
	registryLatency *prometheus.HistogramVec
	registryErrors  *prometheus.CounterVec
}

// NewMetricsCollector creates a new metrics collector
func NewMetricsCollector(logger zerolog.Logger, namespace string) *MetricsCollector {
	if namespace == "" {
		namespace = "security_scanner"
	}

	mc := &MetricsCollector{
		logger:   logger.With().Str("component", "metrics_collector").Logger(),
		registry: prometheus.NewRegistry(),
	}

	// Initialize metrics
	mc.initScanMetrics(namespace)
	mc.initVulnerabilityMetrics(namespace)
	mc.initPolicyMetrics(namespace)
	mc.initSecretMetrics(namespace)
	mc.initCVEMetrics(namespace)
	mc.initHealthMetrics(namespace)
	mc.initScannerMetrics(namespace)
	mc.initRegistryMetrics(namespace)

	// Register all metrics
	mc.registerMetrics()

	mc.logger.Info().Str("namespace", namespace).Msg("Metrics collector initialized")

	return mc
}

// initScanMetrics initializes scan-related metrics
func (mc *MetricsCollector) initScanMetrics(namespace string) {
	mc.scanDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: namespace,
			Name:      "scan_duration_seconds",
			Help:      "Duration of security scans in seconds",
			Buckets:   []float64{1, 5, 10, 30, 60, 120, 300, 600, 1800, 3600},
		},
		[]string{"scanner", "image", "severity_threshold"},
	)

	mc.scanTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "scans_total",
			Help:      "Total number of security scans performed",
		},
		[]string{"scanner", "status"},
	)

	mc.scanErrors = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "scan_errors_total",
			Help:      "Total number of scan errors",
		},
		[]string{"scanner", "error_type"},
	)

	mc.lastScanTime = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "last_scan_timestamp",
			Help:      "Timestamp of the last successful scan",
		},
		[]string{"scanner", "image"},
	)
}

// initVulnerabilityMetrics initializes vulnerability-related metrics
func (mc *MetricsCollector) initVulnerabilityMetrics(namespace string) {
	mc.vulnerabilitiesTotal = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "vulnerabilities_total",
			Help:      "Total number of vulnerabilities found",
		},
		[]string{"image", "scanner"},
	)

	mc.vulnerabilitiesBySeverity = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "vulnerabilities_by_severity",
			Help:      "Number of vulnerabilities by severity level",
		},
		[]string{"image", "severity", "scanner"},
	)

	mc.vulnerabilitiesFixed = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "vulnerabilities_fixed_total",
			Help:      "Total number of vulnerabilities with available fixes",
		},
		[]string{"image", "severity"},
	)

	mc.cvssScores = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: namespace,
			Name:      "cvss_scores",
			Help:      "Distribution of CVSS scores",
			Buckets:   []float64{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10},
		},
		[]string{"image", "cvss_version"},
	)
}

// initPolicyMetrics initializes policy-related metrics
func (mc *MetricsCollector) initPolicyMetrics(namespace string) {
	mc.policyEvaluations = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "policy_evaluations_total",
			Help:      "Total number of policy evaluations",
		},
		[]string{"policy_id", "result"},
	)

	mc.policyViolations = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "policy_violations_total",
			Help:      "Total number of policy violations",
		},
		[]string{"policy_id", "severity", "rule_type"},
	)

	mc.blockedDeployments = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "blocked_deployments_total",
			Help:      "Total number of deployments blocked by policies",
		},
		[]string{"policy_id", "action_type"},
	)

	mc.policyExecution = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: namespace,
			Name:      "policy_execution_duration_seconds",
			Help:      "Duration of policy evaluation in seconds",
			Buckets:   prometheus.DefBuckets,
		},
		[]string{"policy_id"},
	)
}

// initSecretMetrics initializes secret detection metrics
func (mc *MetricsCollector) initSecretMetrics(namespace string) {
	mc.secretsFound = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "secrets_found_total",
			Help:      "Total number of secrets found",
		},
		[]string{"secret_type", "detection_method"},
	)

	mc.secretTypes = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "secret_types_total",
			Help:      "Total number of secrets by type",
		},
		[]string{"secret_type"},
	)

	mc.falsePositives = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "false_positives_total",
			Help:      "Total number of false positive secrets",
		},
		[]string{"secret_type"},
	)

	mc.secretScanDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: namespace,
			Name:      "secret_scan_duration_seconds",
			Help:      "Duration of secret scanning in seconds",
			Buckets:   prometheus.DefBuckets,
		},
		[]string{"scan_type"},
	)
}

// initCVEMetrics initializes CVE database metrics
func (mc *MetricsCollector) initCVEMetrics(namespace string) {
	mc.cveQueries = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "cve_queries_total",
			Help:      "Total number of CVE database queries",
		},
		[]string{"operation", "status"},
	)

	mc.cveCacheHits = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "cve_cache_hits_total",
			Help:      "Total number of CVE cache hits",
		},
		[]string{"operation"},
	)

	mc.cveCacheMisses = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "cve_cache_misses_total",
			Help:      "Total number of CVE cache misses",
		},
		[]string{"operation"},
	)

	mc.cveAPILatency = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: namespace,
			Name:      "cve_api_latency_seconds",
			Help:      "Latency of CVE API requests in seconds",
			Buckets:   []float64{0.1, 0.5, 1, 2, 5, 10, 30},
		},
		[]string{"operation"},
	)
}

// initHealthMetrics initializes health check metrics
func (mc *MetricsCollector) initHealthMetrics(namespace string) {
	mc.componentHealth = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "component_health",
			Help:      "Health status of components (1=healthy, 0.5=degraded, 0=unhealthy)",
		},
		[]string{"component"},
	)

	mc.healthCheckDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: namespace,
			Name:      "health_check_duration_seconds",
			Help:      "Duration of health checks in seconds",
			Buckets:   prometheus.DefBuckets,
		},
		[]string{"component"},
	)

	mc.healthChecksTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "health_checks_total",
			Help:      "Total number of health checks performed",
		},
		[]string{"component", "status"},
	)
}

// initScannerMetrics initializes scanner-specific metrics
func (mc *MetricsCollector) initScannerMetrics(namespace string) {
	mc.scannerAvailability = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "scanner_availability",
			Help:      "Scanner availability (1=available, 0=unavailable)",
		},
		[]string{"scanner"},
	)

	mc.scannerAgreement = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "scanner_agreement_rate",
			Help:      "Agreement rate between scanners (percentage)",
		},
		[]string{"scanner_pair"},
	)

	mc.scannerPerformance = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: namespace,
			Name:      "scanner_performance_seconds",
			Help:      "Scanner performance metrics in seconds",
			Buckets:   []float64{1, 5, 10, 30, 60, 120, 300, 600},
		},
		[]string{"scanner", "metric_type"},
	)
}

// initRegistryMetrics initializes registry health metrics
func (mc *MetricsCollector) initRegistryMetrics(namespace string) {
	mc.registryHealth = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "registry_health",
			Help:      "Registry health status (1=healthy, 0=unhealthy)",
		},
		[]string{"registry_url"},
	)

	mc.registryLatency = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: namespace,
			Name:      "registry_latency_seconds",
			Help:      "Registry response latency in seconds",
			Buckets:   prometheus.DefBuckets,
		},
		[]string{"registry_url", "operation"},
	)

	mc.registryErrors = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "registry_errors_total",
			Help:      "Total number of registry errors",
		},
		[]string{"registry_url", "error_type"},
	)
}

// registerMetrics registers all metrics with the registry
func (mc *MetricsCollector) registerMetrics() {
	metrics := []prometheus.Collector{
		// Scan metrics
		mc.scanDuration,
		mc.scanTotal,
		mc.scanErrors,
		mc.lastScanTime,

		// Vulnerability metrics
		mc.vulnerabilitiesTotal,
		mc.vulnerabilitiesBySeverity,
		mc.vulnerabilitiesFixed,
		mc.cvssScores,

		// Policy metrics
		mc.policyEvaluations,
		mc.policyViolations,
		mc.blockedDeployments,
		mc.policyExecution,

		// Secret metrics
		mc.secretsFound,
		mc.secretTypes,
		mc.falsePositives,
		mc.secretScanDuration,

		// CVE metrics
		mc.cveQueries,
		mc.cveCacheHits,
		mc.cveCacheMisses,
		mc.cveAPILatency,

		// Health metrics
		mc.componentHealth,
		mc.healthCheckDuration,
		mc.healthChecksTotal,

		// Scanner metrics
		mc.scannerAvailability,
		mc.scannerAgreement,
		mc.scannerPerformance,

		// Registry metrics
		mc.registryHealth,
		mc.registryLatency,
		mc.registryErrors,
	}

	for _, metric := range metrics {
		mc.registry.MustRegister(metric)
	}
}

// GetRegistry returns the Prometheus registry
func (mc *MetricsCollector) GetRegistry() *prometheus.Registry {
	return mc.registry
}

// GetHandler returns an HTTP handler for the /metrics endpoint
func (mc *MetricsCollector) GetHandler() http.Handler {
	return promhttp.HandlerFor(mc.registry, promhttp.HandlerOpts{
		EnableOpenMetrics: true,
	})
}

// RecordScanDuration records the duration of a security scan
func (mc *MetricsCollector) RecordScanDuration(scanner, image, severityThreshold string, duration time.Duration) {
	mc.scanDuration.WithLabelValues(scanner, image, severityThreshold).Observe(duration.Seconds())
}

// RecordScanTotal records the total number of scans performed
func (mc *MetricsCollector) RecordScanTotal(scanner, status string) {
	mc.scanTotal.WithLabelValues(scanner, status).Inc()
}

// RecordScanError records scan errors
func (mc *MetricsCollector) RecordScanError(scanner, errorType string) {
	mc.scanErrors.WithLabelValues(scanner, errorType).Inc()
}

// RecordLastScanTime records the timestamp of the last scan
func (mc *MetricsCollector) RecordLastScanTime(scanner, image string, timestamp time.Time) {
	mc.lastScanTime.WithLabelValues(scanner, image).Set(float64(timestamp.Unix()))
}

// RecordVulnerabilities records vulnerability counts by severity
func (mc *MetricsCollector) RecordVulnerabilities(image, scanner string, total int) {
	mc.vulnerabilitiesTotal.WithLabelValues(image, scanner).Set(float64(total))
}

// RecordVulnerabilitiesBySeverity records vulnerabilities grouped by severity level
func (mc *MetricsCollector) RecordVulnerabilitiesBySeverity(image, severity, scanner string, count int) {
	mc.vulnerabilitiesBySeverity.WithLabelValues(image, severity, scanner).Set(float64(count))
}

// RecordVulnerabilitiesFixed records the number of fixed vulnerabilities
func (mc *MetricsCollector) RecordVulnerabilitiesFixed(image, severity string, count int) {
	mc.vulnerabilitiesFixed.WithLabelValues(image, severity).Add(float64(count))
}

// RecordCVSSScore records CVSS scores for vulnerabilities
func (mc *MetricsCollector) RecordCVSSScore(image, cvssVersion string, score float64) {
	mc.cvssScores.WithLabelValues(image, cvssVersion).Observe(score)
}

// RecordPolicyEvaluation records the results of policy evaluations
func (mc *MetricsCollector) RecordPolicyEvaluation(policyID, result string) {
	mc.policyEvaluations.WithLabelValues(policyID, result).Inc()
}

// RecordPolicyViolation records policy violations by type and severity
func (mc *MetricsCollector) RecordPolicyViolation(policyID, severity, ruleType string) {
	mc.policyViolations.WithLabelValues(policyID, severity, ruleType).Inc()
}

// RecordBlockedDeployment records deployments blocked by security policies
func (mc *MetricsCollector) RecordBlockedDeployment(policyID, actionType string) {
	mc.blockedDeployments.WithLabelValues(policyID, actionType).Inc()
}

// RecordPolicyExecutionDuration records how long policy evaluations take
func (mc *MetricsCollector) RecordPolicyExecutionDuration(policyID string, duration time.Duration) {
	mc.policyExecution.WithLabelValues(policyID).Observe(duration.Seconds())
}

// RecordSecretFound records discovered secrets by type and detection method
func (mc *MetricsCollector) RecordSecretFound(secretType, detectionMethod string) {
	mc.secretsFound.WithLabelValues(secretType, detectionMethod).Inc()
}

func (mc *MetricsCollector) RecordSecretType(secretType string) {
	mc.secretTypes.WithLabelValues(secretType).Inc()
}

func (mc *MetricsCollector) RecordFalsePositive(secretType string) {
	mc.falsePositives.WithLabelValues(secretType).Inc()
}

func (mc *MetricsCollector) RecordSecretScanDuration(scanType string, duration time.Duration) {
	mc.secretScanDuration.WithLabelValues(scanType).Observe(duration.Seconds())
}

// Recording methods for CVE metrics
func (mc *MetricsCollector) RecordCVEQuery(operation, status string) {
	mc.cveQueries.WithLabelValues(operation, status).Inc()
}

func (mc *MetricsCollector) RecordCVECacheHit(operation string) {
	mc.cveCacheHits.WithLabelValues(operation).Inc()
}

func (mc *MetricsCollector) RecordCVECacheMiss(operation string) {
	mc.cveCacheMisses.WithLabelValues(operation).Inc()
}

func (mc *MetricsCollector) RecordCVEAPILatency(operation string, duration time.Duration) {
	mc.cveAPILatency.WithLabelValues(operation).Observe(duration.Seconds())
}

// Recording methods for health metrics
func (mc *MetricsCollector) RecordComponentHealth(component string, status HealthStatus) {
	var value float64
	switch status {
	case HealthStatusHealthy:
		value = 1.0
	case HealthStatusDegraded:
		value = 0.5
	case HealthStatusUnhealthy:
		value = 0.0
	}
	mc.componentHealth.WithLabelValues(component).Set(value)
}

func (mc *MetricsCollector) RecordHealthCheckDuration(component string, duration time.Duration) {
	mc.healthCheckDuration.WithLabelValues(component).Observe(duration.Seconds())
}

func (mc *MetricsCollector) RecordHealthCheck(component, status string) {
	mc.healthChecksTotal.WithLabelValues(component, status).Inc()
}

// Recording methods for scanner metrics
func (mc *MetricsCollector) RecordScannerAvailability(scanner string, available bool) {
	var value float64
	if available {
		value = 1.0
	}
	mc.scannerAvailability.WithLabelValues(scanner).Set(value)
}

func (mc *MetricsCollector) RecordScannerAgreement(scannerPair string, agreementRate float64) {
	mc.scannerAgreement.WithLabelValues(scannerPair).Set(agreementRate)
}

func (mc *MetricsCollector) RecordScannerPerformance(scanner, metricType string, duration time.Duration) {
	mc.scannerPerformance.WithLabelValues(scanner, metricType).Observe(duration.Seconds())
}

// Recording methods for registry metrics
func (mc *MetricsCollector) RecordRegistryHealth(registryURL string, healthy bool) {
	var value float64
	if healthy {
		value = 1.0
	}
	mc.registryHealth.WithLabelValues(registryURL).Set(value)
}

func (mc *MetricsCollector) RecordRegistryLatency(registryURL, operation string, duration time.Duration) {
	mc.registryLatency.WithLabelValues(registryURL, operation).Observe(duration.Seconds())
}

func (mc *MetricsCollector) RecordRegistryError(registryURL, errorType string) {
	mc.registryErrors.WithLabelValues(registryURL, errorType).Inc()
}

// UpdateFromScanResult updates metrics from a scan result
func (mc *MetricsCollector) UpdateFromScanResult(scanner string, result interface{}) {
	switch r := result.(type) {
	case ScanResultInterface:
		mc.updateFromBasicScanResult(scanner, r)
	case UnifiedScanResultInterface:
		mc.updateFromUnifiedScanResult(r)
	default:
		mc.logger.Warn().Str("result_type", fmt.Sprintf("%T", result)).Msg("Unknown scan result type")
	}
}

// updateFromBasicScanResult updates metrics from a basic scan result
func (mc *MetricsCollector) updateFromBasicScanResult(scanner string, result ScanResultInterface) {
	// Record scan metrics
	status := "success"
	if !result.GetSuccess() {
		status = "failure"
	}
	mc.RecordScanTotal(scanner, status)
	mc.RecordScanDuration(scanner, result.GetImageRef(), "", result.GetDuration())
	mc.RecordLastScanTime(scanner, result.GetImageRef(), result.GetScanTime())

	// Record vulnerability metrics
	summary := result.GetSummary()
	mc.RecordVulnerabilities(result.GetImageRef(), scanner, summary.GetTotal())
	mc.RecordVulnerabilitiesBySeverity(result.GetImageRef(), "critical", scanner, summary.GetCritical())
	mc.RecordVulnerabilitiesBySeverity(result.GetImageRef(), "high", scanner, summary.GetHigh())
	mc.RecordVulnerabilitiesBySeverity(result.GetImageRef(), "medium", scanner, summary.GetMedium())
	mc.RecordVulnerabilitiesBySeverity(result.GetImageRef(), "low", scanner, summary.GetLow())
	mc.RecordVulnerabilitiesFixed(result.GetImageRef(), "all", summary.GetFixable())

	// Record CVSS scores
	for _, vuln := range result.GetVulnerabilities() {
		cvss := vuln.GetCVSS()
		if cvss != nil && cvss.GetScore() > 0 {
			mc.RecordCVSSScore(result.GetImageRef(), cvss.GetVersion(), cvss.GetScore())
		}
		cvssv3 := vuln.GetCVSSV3()
		if cvssv3 != nil && cvssv3.GetScore() > 0 {
			mc.RecordCVSSScore(result.GetImageRef(), "3.0", cvssv3.GetScore())
		}
	}
}

// updateFromUnifiedScanResult updates metrics from a unified scan result
func (mc *MetricsCollector) updateFromUnifiedScanResult(result UnifiedScanResultInterface) {
	// Record unified scan metrics
	status := "success"
	if !result.GetSuccess() {
		status = "failure"
	}
	mc.RecordScanTotal("unified", status)
	mc.RecordScanDuration("unified", result.GetImageRef(), "", result.GetDuration())
	mc.RecordLastScanTime("unified", result.GetImageRef(), result.GetScanTime())

	// Record vulnerability metrics
	combinedSummary := result.GetCombinedSummary()
	mc.RecordVulnerabilities(result.GetImageRef(), "unified", combinedSummary.GetTotal())
	mc.RecordVulnerabilitiesBySeverity(result.GetImageRef(), "critical", "unified", combinedSummary.GetCritical())
	mc.RecordVulnerabilitiesBySeverity(result.GetImageRef(), "high", "unified", combinedSummary.GetHigh())
	mc.RecordVulnerabilitiesBySeverity(result.GetImageRef(), "medium", "unified", combinedSummary.GetMedium())
	mc.RecordVulnerabilitiesBySeverity(result.GetImageRef(), "low", "unified", combinedSummary.GetLow())
	mc.RecordVulnerabilitiesFixed(result.GetImageRef(), "all", combinedSummary.GetFixable())

	// Record scanner agreement
	comparisonMetrics := result.GetComparisonMetrics()
	if comparisonMetrics != nil {
		mc.RecordScannerAgreement("trivy-grype", comparisonMetrics.GetAgreementRate())
	}

	// Record individual scanner results if available
	trivyResult := result.GetTrivyResult()
	if trivyResult != nil {
		mc.updateFromBasicScanResult("trivy", trivyResult)
	}
	grypeResult := result.GetGrypeResult()
	if grypeResult != nil {
		mc.updateFromBasicScanResult("grype", grypeResult)
	}
}

// UpdateFromPolicyResults updates metrics from policy evaluation results
func (mc *MetricsCollector) UpdateFromPolicyResults(results []PolicyEvaluationResult) {
	for _, result := range results {
		status := "pass"
		if !result.Passed {
			status = "fail"
		}
		mc.RecordPolicyEvaluation(result.PolicyID, status)

		// Record violations
		for _, violation := range result.Violations {
			mc.RecordPolicyViolation(result.PolicyID, string(violation.Severity), violation.RuleID)
		}

		// Record blocking actions
		for _, action := range result.Actions {
			if action.Type == ActionTypeBlock {
				mc.RecordBlockedDeployment(result.PolicyID, string(action.Type))
			}
		}
	}
}

// UpdateFromHealthCheck updates metrics from health check results
func (mc *MetricsCollector) UpdateFromHealthCheck(health OverallHealth) {
	for name, component := range health.Components {
		mc.RecordComponentHealth(name, component.Status)
		mc.RecordHealthCheckDuration(name, component.ResponseTime)

		status := "success"
		if component.Status != HealthStatusHealthy {
			status = "failure"
		}
		mc.RecordHealthCheck(name, status)
	}
}

// UpdateScannerAvailability updates scanner availability metrics
func (mc *MetricsCollector) UpdateScannerAvailability(scanners map[string]bool) {
	for scanner, available := range scanners {
		mc.RecordScannerAvailability(scanner, available)
	}
}

// MetricsEndpointHandler provides HTTP handlers for metrics endpoints
type MetricsEndpointHandler struct {
	collector *MetricsCollector
	logger    zerolog.Logger
}

// NewMetricsEndpointHandler creates a new metrics endpoint handler
func NewMetricsEndpointHandler(collector *MetricsCollector, logger zerolog.Logger) *MetricsEndpointHandler {
	return &MetricsEndpointHandler{
		collector: collector,
		logger:    logger.With().Str("component", "metrics_endpoints").Logger(),
	}
}

// MetricsHandler handles /metrics endpoint requests
func (m *MetricsEndpointHandler) MetricsHandler(w http.ResponseWriter, r *http.Request) {
	handler := m.collector.GetHandler()
	handler.ServeHTTP(w, r)

	m.logger.Debug().Msg("Metrics endpoint accessed")
}

// RegisterRoutes registers metrics routes with a HTTP mux
func (m *MetricsEndpointHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/metrics", m.MetricsHandler)

	m.logger.Info().Msg("Metrics endpoint registered: /metrics")
}
