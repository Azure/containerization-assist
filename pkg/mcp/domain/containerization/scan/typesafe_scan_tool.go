package scan

import (
	"context"
	"fmt"
	"strings"
	"time"

	"log/slog"

	"github.com/Azure/container-kit/pkg/mcp/application/api"
	"github.com/Azure/container-kit/pkg/mcp/application/core"
	"github.com/Azure/container-kit/pkg/mcp/domain/errors"
	"github.com/Azure/container-kit/pkg/mcp/domain/session"
	toolstypes "github.com/Azure/container-kit/pkg/mcp/domain/types/tools"
	"github.com/Azure/container-kit/pkg/mcp/services"
)

// TypeSafeSecurityScanTool implements the new type-safe api.TypedScanTool interface
type TypeSafeSecurityScanTool struct {
	pipelineAdapter core.TypedPipelineOperations
	sessionManager  session.UnifiedSessionManager // Legacy field for backward compatibility
	sessionStore    services.SessionStore         // Modern service interface
	sessionState    services.SessionState         // Modern service interface
	logger          *slog.Logger
	timeout         time.Duration
	atomicTool      *AtomicScanImageSecurityTool // If available
}

// NewTypeSafeSecurityScanTool creates a new type-safe security scan tool (legacy constructor)
func NewTypeSafeSecurityScanTool(
	adapter core.TypedPipelineOperations,
	sessionManager session.UnifiedSessionManager,
	logger *slog.Logger,
) api.Tool {
	return &TypeSafeSecurityScanTool{
		pipelineAdapter: adapter,
		sessionManager:  sessionManager,
		logger:          logger.With("tool", "typesafe_security_scan"),
		timeout:         15 * time.Minute, // Default scan timeout
	}
}

// NewTypeSafeSecurityScanToolWithServices creates a new type-safe security scan tool using service interfaces
func NewTypeSafeSecurityScanToolWithServices(
	adapter core.TypedPipelineOperations,
	serviceContainer services.ServiceContainer,
	logger *slog.Logger,
) api.Tool {
	toolLogger := logger.With("tool", "typesafe_security_scan")

	return &TypeSafeSecurityScanTool{
		pipelineAdapter: adapter,
		sessionStore:    serviceContainer.SessionStore(),
		sessionState:    serviceContainer.SessionState(),
		logger:          toolLogger,
		timeout:         15 * time.Minute, // Default scan timeout
	}
}

// Name implements api.TypedTool
func (t *TypeSafeSecurityScanTool) Name() string {
	return "security_scan"
}

// Description implements api.TypedTool
func (t *TypeSafeSecurityScanTool) Description() string {
	return "Scans images, filesystems, or repositories for security vulnerabilities"
}

// Execute implements api.Tool interface for compatibility
func (t *TypeSafeSecurityScanTool) Execute(ctx context.Context, input api.ToolInput) (api.ToolOutput, error) {
	// Convert api.ToolInput to TypedToolInput
	var typedInput api.TypedScanInput
	if rawParams, ok := input.Data["params"]; ok {
		if params, ok := rawParams.(api.TypedScanInput); ok {
			typedInput = params
		} else {
			return api.ToolOutput{
				Success: false,
				Error:   "Invalid input type for typed scan tool",
			}, errors.NewError().Messagef("invalid input type for typed scan tool").WithLocation().Build()
		}
	} else {
		return api.ToolOutput{
				Success: false,
				Error:   "No params provided",
			}, errors.NewError().Messagef("no params provided").WithLocation(

			// Use session ID from input if available
			).Build()
	}

	if input.SessionID != "" {
		typedInput.SessionID = input.SessionID
	}

	// Execute the typed version using atomic tool if available
	if t.atomicTool != nil {
		// Delegate to atomic tool (simplified approach)
		return api.ToolOutput{
			Success: false,
			Data:    map[string]interface{}{"message": "Scan execution not yet implemented"},
		}, nil
	}

	// Fallback implementation
	return api.ToolOutput{
		Success: false,
		Data:    map[string]interface{}{"message": "No atomic tool available for scan execution"},
		Error:   "atomic scan tool not available",
	}, errors.NewError().Messagef("atomic scan tool not available").Build()
}

func (t *TypeSafeSecurityScanTool) executeWithTelemetry(
	ctx context.Context,
	input api.TypedToolInput[api.TypedScanInput, api.ScanContext],
) (api.TypedToolOutput[api.TypedScanOutput, api.ScanDetails], error) {
	// Telemetry execution removed - call executeInternal directly
	return t.executeInternal(ctx, input)
}

// executeInternal contains the core execution logic
func (t *TypeSafeSecurityScanTool) executeInternal(
	ctx context.Context,
	input api.TypedToolInput[api.TypedScanInput, api.ScanContext],
) (api.TypedToolOutput[api.TypedScanOutput, api.ScanDetails], error) {
	startTime := time.Now()

	// Validate input
	if err := t.validateInput(input); err != nil {
		return api.TypedToolOutput[api.TypedScanOutput, api.ScanDetails]{
			Success: false,
			Error:   err.Error(),
		}, err
	}

	t.logger.Info("Starting security scan",
		"session_id", input.SessionID,
		"target", input.Data.Target,
		"scan_type", string(input.Data.ScanType),
		"severity", input.Data.Severity)

	// Create or get session using appropriate service interface
	sess, err := t.getOrCreateSession(ctx, input.SessionID)
	if err != nil {
		return t.errorOutput(input.SessionID, "Failed to get or create session", err), err
	}

	// Update session state
	sess.AddLabel("scanning")
	sess.UpdateLastAccessed()

	// Execute scan
	scanResult, err := t.executeScan(ctx, input)
	if err != nil {
		return t.errorOutput(input.SessionID, "Scan failed", err), err
	}

	// Store scan results in session
	sess.RemoveLabel("scanning")
	sess.AddLabel("scan_completed")

	// Add execution record
	endTime := time.Now()
	sess.AddToolExecution(session.ToolExecution{
		Tool:      "security_scan",
		StartTime: time.Now().Add(-time.Minute), // Approximate start time
		EndTime:   &endTime,
		Success:   err == nil,
	})

	// Filter vulnerabilities by severity if requested
	vulnerabilities := t.filterVulnerabilities(scanResult.Vulnerabilities, input.Data.Severity)

	// Build output
	output := api.TypedScanOutput{
		Success:         true,
		SessionID:       input.SessionID,
		Vulnerabilities: vulnerabilities,
		ScanMetrics: api.ScanMetrics{
			ScanTime:        time.Since(startTime),
			PackagesScanned: scanResult.PackagesScanned,
			VulnCount:       len(vulnerabilities),
			CriticalCount:   t.countBySeverity(vulnerabilities, "CRITICAL"),
			HighCount:       t.countBySeverity(vulnerabilities, "HIGH"),
			MediumCount:     t.countBySeverity(vulnerabilities, "MEDIUM"),
			LowCount:        t.countBySeverity(vulnerabilities, "LOW"),
		},
		ComplianceStatus: scanResult.ComplianceStatus,
	}

	// Build details
	details := api.ScanDetails{
		ExecutionDetails: api.ExecutionDetails{
			Duration:  time.Since(startTime),
			StartTime: startTime,
			EndTime:   time.Now(),
			ResourcesUsed: api.ResourceUsage{
				CPUTime:    int64(time.Since(startTime).Milliseconds()),
				MemoryPeak: scanResult.MemoryUsed,
				NetworkIO:  scanResult.NetworkIO,
				DiskIO:     scanResult.DiskIO,
			},
		},
		DatabaseVersion: scanResult.DatabaseVersion,
		DatabaseUpdated: scanResult.DatabaseUpdated,
		ScanEngine:      scanResult.ScanEngine,
	}

	// Check if scan should fail based on severity threshold
	if input.Context.FailOnSeverity != "" {
		if t.hasVulnerabilityAboveThreshold(vulnerabilities, input.Context.FailOnSeverity) {
			return api.TypedToolOutput[api.TypedScanOutput, api.ScanDetails]{
				Success: false,
				Data:    output,
				Details: details,
				Error:   fmt.Sprintf("Found vulnerabilities above threshold: %s", input.Context.FailOnSeverity),
			}, nil
		}
	}

	t.logger.Info("Security scan completed",
		"session_id", input.SessionID,
		"duration", time.Since(startTime),
		"vulnerabilities", len(vulnerabilities),
		"critical", output.ScanMetrics.CriticalCount,
		"high", output.ScanMetrics.HighCount)

	return api.TypedToolOutput[api.TypedScanOutput, api.ScanDetails]{
		Success: true,
		Data:    output,
		Details: details,
	}, nil
}

// Schema implements api.Tool interface for compatibility
func (t *TypeSafeSecurityScanTool) Schema() api.ToolSchema {
	return api.ToolSchema{
		Name:        "security_scan",
		Description: "Scans images, filesystems, or repositories for security vulnerabilities",
		Version:     "2.0.0",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"params": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"session_id": map[string]interface{}{
							"type":        "string",
							"description": "Session identifier",
						},
						"target": map[string]interface{}{
							"type":        "string",
							"description": "Target to scan",
						},
						"scan_type": map[string]interface{}{
							"type":        "string",
							"description": "Type of scan to perform",
						},
					},
					"required": []string{"session_id", "target", "scan_type"},
				},
			},
		},
		OutputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"result": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"success": map[string]interface{}{
							"type":        "boolean",
							"description": "Whether the scan was successful",
						},
						"vulnerabilities": map[string]interface{}{
							"type":        "array",
							"description": "List of vulnerabilities found",
						},
					},
				},
			},
		},
	}
}

// validateInput validates the typed input
func (t *TypeSafeSecurityScanTool) validateInput(input api.TypedToolInput[api.TypedScanInput, api.ScanContext]) error {
	if input.SessionID == "" {
		return errors.NewError().
			Code(errors.CodeInvalidParameter).
			Message("Session ID is required").
			Type(errors.ErrTypeValidation).
			Severity(errors.SeverityMedium).
			Build()
	}

	if input.Data.Target == "" {
		return errors.NewError().
			Code(errors.CodeInvalidParameter).
			Message("Scan target is required").
			Type(errors.ErrTypeValidation).
			Severity(errors.SeverityMedium).
			Build()
	}

	if input.Data.ScanType == "" {
		return errors.NewError().
			Code(errors.CodeInvalidParameter).
			Message("Scan type is required").
			Type(errors.ErrTypeValidation).
			Severity(errors.SeverityMedium).
			Build()
	}

	return nil
}

// executeScan performs the actual scan operation
func (t *TypeSafeSecurityScanTool) executeScan(ctx context.Context, input api.TypedToolInput[api.TypedScanInput, api.ScanContext]) (*TypedScanResult, error) {
	// If we have an atomic tool, use it
	if t.atomicTool != nil {
		// Convert TypedScanInput to AtomicScanImageSecurityParams
		atomicParams := toolstypes.AtomicScanImageSecurityParams{
			SessionParams: toolstypes.SessionParams{
				SessionID: input.SessionID,
			},
			ImageRef:       input.Data.Target,
			ScanTypes:      []string{"vulnerability", "secrets"},
			Severity:       strings.Join(input.Data.Severity, ","),
			IncludeSecrets: true,
		}

		atomicResult, err := t.atomicTool.ExecuteTypedInterface(ctx, atomicParams)
		if err != nil {
			return nil, err
		}
		// Convert atomic result to TypedScanResult
		return t.convertAtomicResult(atomicResult), nil
	}

	// Otherwise use the pipeline adapter
	scanParams := core.ConsolidatedScanParams{
		SessionID:      input.SessionID,
		ImageRef:       input.Data.Target,
		ScanType:       string(input.Data.ScanType),
		SeverityFilter: strings.Join(input.Data.Severity, ","),
	}

	result, err := t.pipelineAdapter.ScanSecurityTyped(ctx, input.SessionID, scanParams)
	if err != nil {
		return nil, err
	}

	return &TypedScanResult{
		Vulnerabilities:  t.convertSecurityFindings(result.VulnerabilityDetails),
		PackagesScanned:  len(result.VulnerabilityDetails),
		ComplianceStatus: map[string]bool{"has_issues": len(result.ComplianceIssues) > 0},
		DatabaseVersion:  "unknown",  // Not available in ScanResult
		DatabaseUpdated:  time.Now(), // Not available in ScanResult
		ScanEngine: func() string {
			if result.ScanReport != nil {
				return result.ScanReport.Scanner
			}
			return "unknown"
		}(),
		MemoryUsed: 0, // Not available in ScanResult
		NetworkIO:  0, // Not available in ScanResult
		DiskIO:     0, // Not available in ScanResult
	}, nil
}

// filterVulnerabilities filters vulnerabilities by severity
func (t *TypeSafeSecurityScanTool) filterVulnerabilities(vulns []api.Vulnerability, severities []string) []api.Vulnerability {
	if len(severities) == 0 {
		return vulns
	}

	severityMap := make(map[string]bool)
	for _, s := range severities {
		severityMap[strings.ToUpper(s)] = true
	}

	filtered := make([]api.Vulnerability, 0)
	for _, v := range vulns {
		if severityMap[strings.ToUpper(v.Severity)] {
			filtered = append(filtered, v)
		}
	}

	return filtered
}

// countBySeverity counts vulnerabilities by severity level
func (t *TypeSafeSecurityScanTool) countBySeverity(vulns []api.Vulnerability, severity string) int {
	count := 0
	for _, v := range vulns {
		if strings.EqualFold(v.Severity, severity) {
			count++
		}
	}
	return count
}

// hasVulnerabilityAboveThreshold checks if any vulnerability exceeds the threshold
func (t *TypeSafeSecurityScanTool) hasVulnerabilityAboveThreshold(vulns []api.Vulnerability, threshold string) bool {
	severityOrder := map[string]int{
		"LOW":      1,
		"MEDIUM":   2,
		"HIGH":     3,
		"CRITICAL": 4,
	}

	thresholdLevel := severityOrder[strings.ToUpper(threshold)]
	for _, v := range vulns {
		if severityOrder[strings.ToUpper(v.Severity)] >= thresholdLevel {
			return true
		}
	}

	return false
}

// convertVulnerabilities converts internal vulnerabilities to API format
func (t *TypeSafeSecurityScanTool) convertVulnerabilities(vulns []interface{}) []api.Vulnerability {
	result := make([]api.Vulnerability, 0, len(vulns))

	// TODO: Implement proper conversion based on actual vulnerability structure
	// This is a placeholder implementation
	for _, v := range vulns {
		if m, ok := v.(map[string]interface{}); ok {
			vuln := api.Vulnerability{
				ID:          t.getStringFromMap(m, "id"),
				Severity:    t.getStringFromMap(m, "severity"),
				Package:     t.getStringFromMap(m, "package"),
				Version:     t.getStringFromMap(m, "version"),
				FixedIn:     t.getStringFromMap(m, "fixedIn"),
				Description: t.getStringFromMap(m, "description"),
				CVSSScore:   t.getFloatFromMap(m, "cvssScore"),
				References:  t.getStringSliceFromMap(m, "references"),
			}
			result = append(result, vuln)
		}
	}

	return result
}

// Helper methods
func (t *TypeSafeSecurityScanTool) getStringFromMap(m map[string]interface{}, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

func (t *TypeSafeSecurityScanTool) getFloatFromMap(m map[string]interface{}, key string) float64 {
	if v, ok := m[key].(float64); ok {
		return v
	}
	return 0.0
}

func (t *TypeSafeSecurityScanTool) getStringSliceFromMap(m map[string]interface{}, key string) []string {
	if v, ok := m[key].([]string); ok {
		return v
	}
	if v, ok := m[key].([]interface{}); ok {
		result := make([]string, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok {
				result = append(result, s)
			}
		}
		return result
	}
	return nil
}

// errorOutput creates an error output
func (t *TypeSafeSecurityScanTool) errorOutput(sessionID, message string, err error) api.TypedToolOutput[api.TypedScanOutput, api.ScanDetails] {
	return api.TypedToolOutput[api.TypedScanOutput, api.ScanDetails]{
		Success: false,
		Data: api.TypedScanOutput{
			Success:   false,
			SessionID: sessionID,
			ErrorMsg:  fmt.Sprintf("%s: %v", message, err),
		},
		Error: err.Error(),
	}
}

// ScanResult represents internal scan result
type TypedScanResult struct {
	Vulnerabilities  []api.Vulnerability
	PackagesScanned  int
	ComplianceStatus map[string]bool
	DatabaseVersion  string
	DatabaseUpdated  time.Time
	ScanEngine       string
	MemoryUsed       int64
	NetworkIO        int64
	DiskIO           int64
}

// convertAtomicResult converts atomic tool result to TypedScanResult
func (t *TypeSafeSecurityScanTool) convertAtomicResult(atomicResult toolstypes.AtomicScanImageSecurityResult) *TypedScanResult {
	return &TypedScanResult{
		Vulnerabilities:  t.convertAtomicFindings(atomicResult.Findings),
		PackagesScanned:  atomicResult.TotalFindings,
		ComplianceStatus: map[string]bool{"compliant": atomicResult.Success},
		DatabaseVersion:  atomicResult.Scanner,
		DatabaseUpdated:  time.Now(), // Not available in atomic result, use current time
		ScanEngine:       atomicResult.Scanner,
		MemoryUsed:       0, // Not available in atomic result
		NetworkIO:        0, // Not available in atomic result
		DiskIO:           0, // Not available in atomic result
	}
}

// convertAtomicFindings converts atomic security findings to api.Vulnerability format
func (t *TypeSafeSecurityScanTool) convertAtomicFindings(findings []toolstypes.SecurityFinding) []api.Vulnerability {
	var vulns []api.Vulnerability
	for _, finding := range findings {
		// Only convert vulnerability findings
		if finding.Type == "vulnerability" {
			vulns = append(vulns, api.Vulnerability{
				ID:          finding.ID,
				Severity:    finding.Severity,
				Description: finding.Description,
				Package:     finding.Package,
				Version:     finding.Version,
				FixedIn:     finding.FixedIn,
			})
		}
	}
	return vulns
}

// convertSecurityFindings converts core.SecurityFinding to api.Vulnerability format
func (t *TypeSafeSecurityScanTool) convertSecurityFindings(findings []core.SecurityFinding) []api.Vulnerability {
	var vulns []api.Vulnerability
	for _, finding := range findings {
		vulns = append(vulns, api.Vulnerability{
			ID:          finding.ID,
			Severity:    finding.Severity,
			Description: finding.Description,
			Package:     finding.Package,
			Version:     finding.Version,
			FixedIn:     finding.FixedIn,
		})
	}
	return vulns
}

// getOrCreateSession gets or creates a session using appropriate interface (service or legacy)
func (t *TypeSafeSecurityScanTool) getOrCreateSession(ctx context.Context, sessionID string) (*session.SessionState, error) {
	// If service interfaces are available, use them (modern pattern)
	if t.sessionStore != nil && t.sessionState != nil {
		// Try to get existing session first
		sessionData, err := t.sessionStore.Get(ctx, sessionID)
		if err != nil {
			// Create new session if it doesn't exist
			newSessionID, err := t.sessionStore.Create(ctx, map[string]interface{}{
				"tool": "typesafe_security_scan",
				"type": "scan",
			})
			if err != nil {
				return nil, fmt.Errorf("failed to create session: %w", err)
			}
			sessionData, err = t.sessionStore.Get(ctx, newSessionID)
			if err != nil {
				return nil, fmt.Errorf("failed to get created session: %w", err)
			}
		}

		// Convert to session.SessionState for compatibility
		return &session.SessionState{
			SessionID: sessionData.ID,
		}, nil
	}

	// Fall back to legacy unified session manager
	if t.sessionManager != nil {
		return t.sessionManager.GetOrCreateSession(ctx, sessionID)
	}

	return nil, fmt.Errorf("no session management interface available")
}
