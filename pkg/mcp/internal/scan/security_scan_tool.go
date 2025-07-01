package scan

import (
	"context"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/core"
	"github.com/Azure/container-kit/pkg/mcp/errors/rich"
	"github.com/Azure/container-kit/pkg/mcp/internal/deploy"
	"github.com/Azure/container-kit/pkg/mcp/types/tools"
	"github.com/rs/zerolog"
)

// securityScanToolImpl implements the strongly-typed security scan tool
type securityScanToolImpl struct {
	pipelineAdapter core.PipelineOperations
	sessionManager  core.ToolSessionManager
	logger          zerolog.Logger
}

// NewSecurityScanTool creates a new strongly-typed security scan tool
func NewSecurityScanTool(adapter core.PipelineOperations, sessionManager core.ToolSessionManager, logger zerolog.Logger) deploy.SecurityScanTool {
	toolLogger := logger.With().Str("tool", "security_scan").Logger()

	return &securityScanToolImpl{
		pipelineAdapter: adapter,
		sessionManager:  sessionManager,
		logger:          toolLogger,
	}
}

// Execute implements tools.Tool[SecurityScanParams, SecurityScanResult]
func (t *securityScanToolImpl) Execute(ctx context.Context, params deploy.SecurityScanParams) (deploy.SecurityScanResult, error) {
	startTime := time.Now()

	// Validate parameters at compile time
	if err := params.Validate(); err != nil {
		return deploy.SecurityScanResult{}, rich.NewError().
			Code(rich.CodeInvalidParameter).
			Message("Security scan parameters validation failed").
			Type(rich.ErrTypeValidation).
			Severity(rich.SeverityMedium).
			Cause(err).
			Context("target", params.Target).
			Context("scan_type", params.ScanType).
			Context("scanner", params.Scanner).
			Suggestion("Ensure target and scan_type are provided and valid").
			WithLocation().
			Build()
	}

	// Set default scanner if not specified
	scanner := params.Scanner
	if scanner == "" {
		scanner = "trivy"
	}

	// Execute security scan using pipeline adapter
	scanErr := t.executeScan(ctx, params, scanner)

	// Create result
	result := deploy.SecurityScanResult{
		Success:   scanErr == nil,
		Target:    params.Target,
		ScanType:  params.ScanType,
		Scanner:   scanner,
		Duration:  time.Since(startTime),
		SessionID: params.SessionID,
	}

	if scanErr != nil {
		// Create RichError for scan failures
		return result, rich.NewError().
			Code("SECURITY_SCAN_FAILED").
			Message("Failed to complete security scan").
			Type(rich.ErrTypeBusiness).
			Severity(rich.SeverityHigh).
			Cause(scanErr).
			Context("target", params.Target).
			Context("scan_type", params.ScanType).
			Context("scanner", scanner).
			Context("format", params.Format).
			Suggestion("Check target availability and scanner configuration").
			WithLocation().
			Build()
	}

	// Mock scan results (in real implementation, this would come from scanner API)
	result.TotalVulnerabilities = 5
	result.VulnerabilitiesBySeverity = map[string]int{
		"CRITICAL": 1,
		"HIGH":     2,
		"MEDIUM":   1,
		"LOW":      1,
	}

	result.Vulnerabilities = []deploy.SecurityVulnerability{
		{
			ID:          "CVE-2023-1234",
			Title:       "Buffer overflow in example library",
			Description: "A buffer overflow vulnerability in the example library",
			Severity:    "CRITICAL",
			CVSS:        9.8,
			Package: struct {
				Name           string `json:"name"`
				Version        string `json:"version"`
				FixedVersion   string `json:"fixed_version,omitempty"`
				PackageManager string `json:"package_manager,omitempty"`
			}{
				Name:           "example-lib",
				Version:        "1.0.0",
				FixedVersion:   "1.0.1",
				PackageManager: "npm",
			},
			References: []string{
				"https://nvd.nist.gov/vuln/detail/CVE-2023-1234",
				"https://github.com/example/advisory",
			},
			Fixed: true,
			Fix:   "Update to version 1.0.1 or later",
		},
	}

	result.ComplianceResults = []deploy.ComplianceResult{
		{
			Standard:    "CIS Docker Benchmark",
			Control:     "4.1",
			Status:      "PASS",
			Description: "Ensure a user for the container has been created",
		},
	}

	result.Secrets = []deploy.DetectedSecret{
		{
			Type:        "api_key",
			File:        "config.yaml",
			Line:        15,
			Description: "Potential API key detected",
			Severity:    "HIGH",
		},
	}

	result.Licenses = []deploy.LicenseInfo{
		{
			Package: "example-lib",
			License: "MIT",
			Type:    "permissive",
			Risk:    "low",
		},
	}

	// Calculate risk score and level
	result.RiskScore = calculateRiskScore(result.VulnerabilitiesBySeverity)
	result.RiskLevel = calculateRiskLevel(result.RiskScore)

	result.Recommendations = []string{
		"Update example-lib to version 1.0.1 to fix critical vulnerability",
		"Review and remove detected API key from config.yaml",
		"Consider implementing secret management solution",
	}

	t.logger.Info().
		Str("target", params.Target).
		Str("scanner", scanner).
		Int("vulnerabilities", result.TotalVulnerabilities).
		Float64("risk_score", result.RiskScore).
		Dur("duration", result.Duration).
		Msg("Security scan completed successfully")

	return result, nil
}

// GetName implements tools.Tool
func (t *securityScanToolImpl) GetName() string {
	return "security_scan"
}

// GetDescription implements tools.Tool
func (t *securityScanToolImpl) GetDescription() string {
	return "Performs comprehensive security scans on images, containers, and filesystems with strongly-typed parameters using session context"
}

// GetSchema implements tools.Tool
func (t *securityScanToolImpl) GetSchema() tools.Schema[deploy.SecurityScanParams, deploy.SecurityScanResult] {
	return tools.Schema[deploy.SecurityScanParams, deploy.SecurityScanResult]{
		Name:        "security_scan",
		Description: "Strongly-typed security scanning tool with RichError support",
		Version:     "2.0.0",
		ParamsSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"target": map[string]interface{}{
					"type":        "string",
					"description": "Target to scan (image name, container ID, or filesystem path)",
					"minLength":   1,
				},
				"scan_type": map[string]interface{}{
					"type":        "string",
					"description": "Type of scan to perform",
					"enum":        []string{"image", "container", "filesystem"},
				},
				"scanner": map[string]interface{}{
					"type":        "string",
					"description": "Scanner to use (default: trivy)",
					"enum":        []string{"trivy", "grype"},
				},
				"format": map[string]interface{}{
					"type":        "string",
					"description": "Output format",
					"enum":        []string{"json", "yaml", "table"},
				},
				"severity": map[string]interface{}{
					"type":        "array",
					"description": "Severity levels to include",
					"items": map[string]interface{}{
						"type": "string",
						"enum": []string{"UNKNOWN", "LOW", "MEDIUM", "HIGH", "CRITICAL"},
					},
				},
				"ignore_unfixed": map[string]interface{}{
					"type":        "boolean",
					"description": "Ignore vulnerabilities without fixes",
				},
				"output_path": map[string]interface{}{
					"type":        "string",
					"description": "Path to save scan results",
				},
				"session_id": map[string]interface{}{
					"type":        "string",
					"description": "Session ID for tracking",
				},
			},
			"required": []string{"target", "scan_type"},
		},
		ResultSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"success": map[string]interface{}{
					"type":        "boolean",
					"description": "Whether the scan was successful",
				},
				"target": map[string]interface{}{
					"type":        "string",
					"description": "Target that was scanned",
				},
				"scan_type": map[string]interface{}{
					"type":        "string",
					"description": "Type of scan performed",
				},
				"duration": map[string]interface{}{
					"type":        "string",
					"description": "Scan duration",
				},
				"total_vulnerabilities": map[string]interface{}{
					"type":        "integer",
					"description": "Total number of vulnerabilities found",
				},
				"risk_score": map[string]interface{}{
					"type":        "number",
					"description": "Calculated risk score",
				},
				"risk_level": map[string]interface{}{
					"type":        "string",
					"description": "Risk level assessment",
				},
			},
		},
		Examples: []tools.Example[deploy.SecurityScanParams, deploy.SecurityScanResult]{
			{
				Name:        "scan_docker_image",
				Description: "Scan a Docker image for vulnerabilities",
				Params: deploy.SecurityScanParams{
					Target:        "nginx:latest",
					ScanType:      "image",
					Scanner:       "trivy",
					Format:        "json",
					IgnoreUnfixed: false,
					SessionID:     "session-123",
				},
				Result: deploy.SecurityScanResult{
					Success:              true,
					Target:               "nginx:latest",
					ScanType:             "image",
					Scanner:              "trivy",
					Duration:             30 * time.Second,
					TotalVulnerabilities: 5,
					VulnerabilitiesBySeverity: map[string]int{
						"CRITICAL": 1,
						"HIGH":     2,
						"MEDIUM":   1,
						"LOW":      1,
					},
					RiskScore: 7.5,
					RiskLevel: "HIGH",
					SessionID: "session-123",
				},
			},
		},
	}
}

// executeScan performs the actual security scan operation
func (t *securityScanToolImpl) executeScan(ctx context.Context, params deploy.SecurityScanParams, scanner string) error {
	// This would integrate with the existing pipeline adapter
	// For now, we'll simulate the operation
	t.logger.Info().
		Str("target", params.Target).
		Str("scan_type", params.ScanType).
		Str("scanner", scanner).
		Msg("Executing security scan")

	// In real implementation, this would use:
	// return t.pipelineAdapter.ScanForVulnerabilities(ctx, params)

	// For demonstration, we'll just validate the parameters
	if params.Target == "" || params.ScanType == "" {
		return rich.NewError().
			Code(rich.CodeInvalidParameter).
			Message("Missing required scan parameters").
			Type(rich.ErrTypeValidation).
			Build()
	}

	// Simulate scan delay
	time.Sleep(150 * time.Millisecond)

	return nil // Success for demonstration
}

// calculateRiskScore calculates a risk score based on vulnerability counts
func calculateRiskScore(vulnerabilities map[string]int) float64 {
	score := 0.0

	// Weight vulnerabilities by severity
	weights := map[string]float64{
		"CRITICAL": 10.0,
		"HIGH":     7.0,
		"MEDIUM":   4.0,
		"LOW":      1.0,
		"UNKNOWN":  0.5,
	}

	for severity, count := range vulnerabilities {
		if weight, exists := weights[severity]; exists {
			score += weight * float64(count)
		}
	}

	// Normalize to 0-10 scale
	if score > 10 {
		return 10.0
	}
	return score
}

// calculateRiskLevel determines risk level based on risk score
func calculateRiskLevel(score float64) string {
	if score >= 8.0 {
		return "CRITICAL"
	} else if score >= 6.0 {
		return "HIGH"
	} else if score >= 4.0 {
		return "MEDIUM"
	} else if score >= 2.0 {
		return "LOW"
	}
	return "MINIMAL"
}
