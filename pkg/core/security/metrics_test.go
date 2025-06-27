package security

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	dto "github.com/prometheus/client_model/go"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewMetricsCollector(t *testing.T) {
	logger := zerolog.Nop()
	collector := NewMetricsCollector(logger, "test_security")

	assert.NotNil(t, collector)
	assert.NotNil(t, collector.registry)
	assert.NotNil(t, collector.scanDuration)
	assert.NotNil(t, collector.vulnerabilitiesTotal)
	assert.NotNil(t, collector.policyEvaluations)
	assert.NotNil(t, collector.secretsFound)
	assert.NotNil(t, collector.componentHealth)
}

func TestMetricsCollector_RecordScanMetrics(t *testing.T) {
	logger := zerolog.Nop()
	collector := NewMetricsCollector(logger, "test")

	// Record scan duration
	collector.RecordScanDuration("trivy", "nginx:latest", "HIGH,CRITICAL", 30*time.Second)

	// Record scan total
	collector.RecordScanTotal("trivy", "success")
	collector.RecordScanTotal("grype", "failure")

	// Record scan error
	collector.RecordScanError("trivy", "timeout")

	// Record last scan time
	now := time.Now()
	collector.RecordLastScanTime("trivy", "nginx:latest", now)

	// Verify metrics were recorded
	metrics, err := collector.registry.Gather()
	require.NoError(t, err)

	metricNames := make(map[string]bool)
	for _, metric := range metrics {
		metricNames[*metric.Name] = true
	}

	assert.True(t, metricNames["test_scan_duration_seconds"])
	assert.True(t, metricNames["test_scans_total"])
	assert.True(t, metricNames["test_scan_errors_total"])
	assert.True(t, metricNames["test_last_scan_timestamp"])
}

func TestMetricsCollector_RecordVulnerabilityMetrics(t *testing.T) {
	logger := zerolog.Nop()
	collector := NewMetricsCollector(logger, "test")

	// Record vulnerability metrics
	collector.RecordVulnerabilities("nginx:latest", "trivy", 15)
	collector.RecordVulnerabilitiesBySeverity("nginx:latest", "critical", "trivy", 3)
	collector.RecordVulnerabilitiesBySeverity("nginx:latest", "high", "trivy", 5)
	collector.RecordVulnerabilitiesFixed("nginx:latest", "critical", 2)
	collector.RecordCVSSScore("nginx:latest", "3.0", 8.5)

	// Gather metrics
	metrics, err := collector.registry.Gather()
	require.NoError(t, err)

	// Check that vulnerability metrics exist
	metricNames := make(map[string]bool)
	for _, metric := range metrics {
		metricNames[*metric.Name] = true
	}

	assert.True(t, metricNames["test_vulnerabilities_total"])
	assert.True(t, metricNames["test_vulnerabilities_by_severity"])
	assert.True(t, metricNames["test_vulnerabilities_fixed_total"])
	assert.True(t, metricNames["test_cvss_scores"])
}

func TestMetricsCollector_RecordPolicyMetrics(t *testing.T) {
	logger := zerolog.Nop()
	collector := NewMetricsCollector(logger, "test")

	// Record policy metrics
	collector.RecordPolicyEvaluation("critical-vulns-block", "fail")
	collector.RecordPolicyViolation("critical-vulns-block", "critical", "vulnerability_count")
	collector.RecordBlockedDeployment("critical-vulns-block", "block")
	collector.RecordPolicyExecutionDuration("critical-vulns-block", 100*time.Millisecond)

	// Gather metrics
	metrics, err := collector.registry.Gather()
	require.NoError(t, err)

	// Check that policy metrics exist
	metricNames := make(map[string]bool)
	for _, metric := range metrics {
		metricNames[*metric.Name] = true
	}

	assert.True(t, metricNames["test_policy_evaluations_total"])
	assert.True(t, metricNames["test_policy_violations_total"])
	assert.True(t, metricNames["test_blocked_deployments_total"])
	assert.True(t, metricNames["test_policy_execution_duration_seconds"])
}

func TestMetricsCollector_RecordSecretMetrics(t *testing.T) {
	logger := zerolog.Nop()
	collector := NewMetricsCollector(logger, "test")

	// Record secret metrics
	collector.RecordSecretFound("aws_access_key", "pattern")
	collector.RecordSecretType("aws_access_key")
	collector.RecordFalsePositive("api_key")
	collector.RecordSecretScanDuration("directory", 5*time.Second)

	// Gather metrics
	metrics, err := collector.registry.Gather()
	require.NoError(t, err)

	// Check that secret metrics exist
	metricNames := make(map[string]bool)
	for _, metric := range metrics {
		metricNames[*metric.Name] = true
	}

	assert.True(t, metricNames["test_secrets_found_total"])
	assert.True(t, metricNames["test_secret_types_total"])
	assert.True(t, metricNames["test_false_positives_total"])
	assert.True(t, metricNames["test_secret_scan_duration_seconds"])
}

func TestMetricsCollector_RecordHealthMetrics(t *testing.T) {
	logger := zerolog.Nop()
	collector := NewMetricsCollector(logger, "test")

	// Record health metrics
	collector.RecordComponentHealth("trivy_scanner", HealthStatusHealthy)
	collector.RecordComponentHealth("policy_engine", HealthStatusDegraded)
	collector.RecordComponentHealth("cve_database", HealthStatusUnhealthy)
	collector.RecordHealthCheckDuration("trivy_scanner", 200*time.Millisecond)
	collector.RecordHealthCheck("trivy_scanner", "success")

	// Gather metrics
	metrics, err := collector.registry.Gather()
	require.NoError(t, err)

	// Find component health metric
	var healthMetric *dto.MetricFamily
	for _, metric := range metrics {
		if *metric.Name == "test_component_health" {
			healthMetric = metric
			break
		}
	}

	require.NotNil(t, healthMetric)
	require.Len(t, healthMetric.Metric, 3)

	// Check health values
	healthValues := make(map[string]float64)
	for _, metric := range healthMetric.Metric {
		component := ""
		for _, label := range metric.Label {
			if *label.Name == "component" {
				component = *label.Value
				break
			}
		}
		healthValues[component] = *metric.Gauge.Value
	}

	assert.Equal(t, 1.0, healthValues["trivy_scanner"])
	assert.Equal(t, 0.5, healthValues["policy_engine"])
	assert.Equal(t, 0.0, healthValues["cve_database"])
}

// MockScanResult implements ScanResultInterface for testing
type MockScanResult struct {
	success         bool
	imageRef        string
	scanTime        time.Time
	duration        time.Duration
	summary         MockVulnerabilitySummary
	vulnerabilities []MockVulnerability
}

func (m *MockScanResult) GetSuccess() bool                          { return m.success }
func (m *MockScanResult) GetImageRef() string                       { return m.imageRef }
func (m *MockScanResult) GetScanTime() time.Time                    { return m.scanTime }
func (m *MockScanResult) GetDuration() time.Duration                { return m.duration }
func (m *MockScanResult) GetSummary() VulnerabilitySummaryInterface { return &m.summary }
func (m *MockScanResult) GetVulnerabilities() []VulnerabilityInterface {
	var vulns []VulnerabilityInterface
	for i := range m.vulnerabilities {
		vulns = append(vulns, &m.vulnerabilities[i])
	}
	return vulns
}

// MockVulnerabilitySummary implements VulnerabilitySummaryInterface
type MockVulnerabilitySummary struct {
	total    int
	critical int
	high     int
	medium   int
	low      int
	fixable  int
}

func (m *MockVulnerabilitySummary) GetTotal() int    { return m.total }
func (m *MockVulnerabilitySummary) GetCritical() int { return m.critical }
func (m *MockVulnerabilitySummary) GetHigh() int     { return m.high }
func (m *MockVulnerabilitySummary) GetMedium() int   { return m.medium }
func (m *MockVulnerabilitySummary) GetLow() int      { return m.low }
func (m *MockVulnerabilitySummary) GetFixable() int  { return m.fixable }

// MockVulnerability implements VulnerabilityInterface
type MockVulnerability struct {
	cvss   MockCVSS
	cvssv3 MockCVSSV3
}

func (m *MockVulnerability) GetCVSS() CVSSInterface     { return &m.cvss }
func (m *MockVulnerability) GetCVSSV3() CVSSV3Interface { return &m.cvssv3 }

// MockCVSS implements CVSSInterface
type MockCVSS struct {
	version string
	score   float64
}

func (m *MockCVSS) GetVersion() string { return m.version }
func (m *MockCVSS) GetScore() float64  { return m.score }

// MockCVSSV3 implements CVSSV3Interface
type MockCVSSV3 struct {
	score float64
}

func (m *MockCVSSV3) GetScore() float64 { return m.score }

func TestMetricsCollector_UpdateFromScanResult(t *testing.T) {
	logger := zerolog.Nop()
	collector := NewMetricsCollector(logger, "test")

	// Create a sample scan result
	scanResult := &MockScanResult{
		success:  true,
		imageRef: "nginx:latest",
		scanTime: time.Now(),
		duration: 30 * time.Second,
		summary: MockVulnerabilitySummary{
			total:    10,
			critical: 2,
			high:     3,
			medium:   3,
			low:      2,
			fixable:  8,
		},
		vulnerabilities: []MockVulnerability{
			{
				cvss: MockCVSS{
					version: "2.0",
					score:   7.5,
				},
				cvssv3: MockCVSSV3{
					score: 8.5,
				},
			},
		},
	}

	// Update metrics from scan result
	collector.UpdateFromScanResult("trivy", scanResult)

	// Gather metrics
	metrics, err := collector.registry.Gather()
	require.NoError(t, err)

	// Verify scan metrics were recorded
	found := false
	for _, metricFamily := range metrics {
		if *metricFamily.Name == "test_scans_total" {
			for _, metric := range metricFamily.Metric {
				scannerLabel := ""
				statusLabel := ""
				for _, label := range metric.Label {
					if *label.Name == "scanner" {
						scannerLabel = *label.Value
					}
					if *label.Name == "status" {
						statusLabel = *label.Value
					}
				}
				if scannerLabel == "trivy" && statusLabel == "success" {
					assert.Equal(t, 1.0, *metric.Counter.Value)
					found = true
				}
			}
		}
	}
	assert.True(t, found, "Expected to find trivy success scan metric")
}

// MockUnifiedScanResult implements UnifiedScanResultInterface for testing
type MockUnifiedScanResult struct {
	success           bool
	imageRef          string
	scanTime          time.Time
	duration          time.Duration
	combinedSummary   MockVulnerabilitySummary
	comparisonMetrics MockComparisonMetrics
	trivyResult       *MockScanResult
	grypeResult       *MockScanResult
}

func (m *MockUnifiedScanResult) GetSuccess() bool           { return m.success }
func (m *MockUnifiedScanResult) GetImageRef() string        { return m.imageRef }
func (m *MockUnifiedScanResult) GetScanTime() time.Time     { return m.scanTime }
func (m *MockUnifiedScanResult) GetDuration() time.Duration { return m.duration }
func (m *MockUnifiedScanResult) GetCombinedSummary() VulnerabilitySummaryInterface {
	return &m.combinedSummary
}
func (m *MockUnifiedScanResult) GetComparisonMetrics() ComparisonMetricsInterface {
	return &m.comparisonMetrics
}
func (m *MockUnifiedScanResult) GetTrivyResult() ScanResultInterface { return m.trivyResult }
func (m *MockUnifiedScanResult) GetGrypeResult() ScanResultInterface { return m.grypeResult }

// MockComparisonMetrics implements ComparisonMetricsInterface
type MockComparisonMetrics struct {
	agreementRate float64
}

func (m *MockComparisonMetrics) GetAgreementRate() float64 { return m.agreementRate }

func TestMetricsCollector_UpdateFromUnifiedScanResult(t *testing.T) {
	logger := zerolog.Nop()
	collector := NewMetricsCollector(logger, "test")

	// Create a sample unified scan result
	unifiedResult := &MockUnifiedScanResult{
		success:  true,
		imageRef: "nginx:latest",
		scanTime: time.Now(),
		duration: 45 * time.Second,
		combinedSummary: MockVulnerabilitySummary{
			total:    15,
			critical: 3,
			high:     5,
			medium:   4,
			low:      3,
			fixable:  12,
		},
		comparisonMetrics: MockComparisonMetrics{
			agreementRate: 85.5,
		},
		// Provide mock results to avoid nil pointer dereference
		trivyResult: &MockScanResult{
			success:  true,
			imageRef: "nginx:latest",
			scanTime: time.Now(),
			duration: 25 * time.Second,
			summary: MockVulnerabilitySummary{
				total:    8,
				critical: 1,
				high:     2,
				medium:   3,
				low:      2,
				fixable:  6,
			},
		},
		grypeResult: &MockScanResult{
			success:  true,
			imageRef: "nginx:latest",
			scanTime: time.Now(),
			duration: 20 * time.Second,
			summary: MockVulnerabilitySummary{
				total:    7,
				critical: 2,
				high:     3,
				medium:   1,
				low:      1,
				fixable:  6,
			},
		},
	}

	// Update metrics from unified scan result
	collector.UpdateFromScanResult("unified", unifiedResult)

	// Gather metrics
	metrics, err := collector.registry.Gather()
	require.NoError(t, err)

	// Verify unified scan metrics were recorded
	found := false
	for _, metricFamily := range metrics {
		if *metricFamily.Name == "test_scanner_agreement_rate" {
			for _, metric := range metricFamily.Metric {
				scannerPairLabel := ""
				for _, label := range metric.Label {
					if *label.Name == "scanner_pair" {
						scannerPairLabel = *label.Value
					}
				}
				if scannerPairLabel == "trivy-grype" {
					assert.Equal(t, 85.5, *metric.Gauge.Value)
					found = true
				}
			}
		}
	}
	assert.True(t, found, "Expected to find scanner agreement metric")
}

func TestMetricsCollector_UpdateFromPolicyResults(t *testing.T) {
	logger := zerolog.Nop()
	collector := NewMetricsCollector(logger, "test")

	// Create sample policy results
	policyResults := []PolicyEvaluationResult{
		{
			PolicyID: "critical-vulns-block",
			Passed:   false,
			Violations: []PolicyViolation{
				{
					RuleID:   "critical-count",
					Severity: PolicySeverityCritical,
				},
			},
			Actions: []PolicyAction{
				{Type: ActionTypeBlock},
			},
		},
		{
			PolicyID: "high-vulns-warn",
			Passed:   true,
		},
	}

	// Update metrics from policy results
	collector.UpdateFromPolicyResults(policyResults)

	// Gather metrics
	metrics, err := collector.registry.Gather()
	require.NoError(t, err)

	// Verify policy metrics were recorded
	foundEvaluation := false
	foundViolation := false
	foundBlock := false

	for _, metricFamily := range metrics {
		switch *metricFamily.Name {
		case "test_policy_evaluations_total":
			for _, metric := range metricFamily.Metric {
				policyID := ""
				result := ""
				for _, label := range metric.Label {
					if *label.Name == "policy_id" {
						policyID = *label.Value
					}
					if *label.Name == "result" {
						result = *label.Value
					}
				}
				if policyID == "critical-vulns-block" && result == "fail" {
					foundEvaluation = true
				}
			}
		case "test_policy_violations_total":
			foundViolation = len(metricFamily.Metric) > 0
		case "test_blocked_deployments_total":
			foundBlock = len(metricFamily.Metric) > 0
		}
	}

	assert.True(t, foundEvaluation, "Expected to find policy evaluation metric")
	assert.True(t, foundViolation, "Expected to find policy violation metric")
	assert.True(t, foundBlock, "Expected to find blocked deployment metric")
}

func TestMetricsEndpointHandler_MetricsHandler(t *testing.T) {
	logger := zerolog.Nop()
	collector := NewMetricsCollector(logger, "test")
	handler := NewMetricsEndpointHandler(collector, logger)

	// Record some test metrics
	collector.RecordScanTotal("trivy", "success")
	collector.RecordVulnerabilities("nginx:latest", "trivy", 5)

	// Create HTTP request
	req := httptest.NewRequest("GET", "/metrics", nil)
	w := httptest.NewRecorder()

	// Handle request
	handler.MetricsHandler(w, req)

	// Check response
	assert.Equal(t, http.StatusOK, w.Code)

	body := w.Body.String()
	assert.Contains(t, body, "test_scans_total")
	assert.Contains(t, body, "test_vulnerabilities_total")
	assert.Contains(t, body, "trivy")
	assert.Contains(t, body, "nginx:latest")
}

func TestMetricsEndpointHandler_RegisterRoutes(t *testing.T) {
	logger := zerolog.Nop()
	collector := NewMetricsCollector(logger, "test")
	handler := NewMetricsEndpointHandler(collector, logger)

	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	// Test that the route is registered
	req := httptest.NewRequest("GET", "/metrics", nil)
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	// Should not return 404
	assert.NotEqual(t, http.StatusNotFound, w.Code)
}

func TestMetricsCollector_GetHandler(t *testing.T) {
	logger := zerolog.Nop()
	collector := NewMetricsCollector(logger, "test")

	handler := collector.GetHandler()
	assert.NotNil(t, handler)

	// Test the handler
	req := httptest.NewRequest("GET", "/metrics", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Header().Get("Content-Type"), "text/plain")
}

func TestMetricsCollector_EmptyNamespace(t *testing.T) {
	logger := zerolog.Nop()
	collector := NewMetricsCollector(logger, "")

	// Should use default namespace
	assert.NotNil(t, collector)

	// Record a metric and check it has the default namespace
	collector.RecordScanTotal("trivy", "success")

	metrics, err := collector.registry.Gather()
	require.NoError(t, err)

	found := false
	for _, metric := range metrics {
		if strings.HasPrefix(*metric.Name, "security_scanner_") {
			found = true
			break
		}
	}
	assert.True(t, found, "Expected to find metric with default namespace 'security_scanner'")
}

func TestMetricsCollector_ScannerAvailability(t *testing.T) {
	logger := zerolog.Nop()
	collector := NewMetricsCollector(logger, "test")

	// Record scanner availability
	collector.RecordScannerAvailability("trivy", true)
	collector.RecordScannerAvailability("grype", false)

	// Update from map
	scanners := map[string]bool{
		"trivy": true,
		"grype": false,
	}
	collector.UpdateScannerAvailability(scanners)

	// Gather metrics
	metrics, err := collector.registry.Gather()
	require.NoError(t, err)

	// Find scanner availability metric
	var availabilityMetric *dto.MetricFamily
	for _, metric := range metrics {
		if *metric.Name == "test_scanner_availability" {
			availabilityMetric = metric
			break
		}
	}

	require.NotNil(t, availabilityMetric)

	// Check availability values
	availabilityValues := make(map[string]float64)
	for _, metric := range availabilityMetric.Metric {
		scanner := ""
		for _, label := range metric.Label {
			if *label.Name == "scanner" {
				scanner = *label.Value
				break
			}
		}
		availabilityValues[scanner] = *metric.Gauge.Value
	}

	assert.Equal(t, 1.0, availabilityValues["trivy"])
	assert.Equal(t, 0.0, availabilityValues["grype"])
}
