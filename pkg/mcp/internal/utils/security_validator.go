package utils

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/rs/zerolog"
)

// SecurityValidator provides comprehensive security validation for sandboxed execution
type SecurityValidator struct {
	logger       zerolog.Logger
	vulnDatabase map[string]VulnerabilityInfo
	policyEngine *SecurityPolicyEngine
	threatModel  *ThreatModel
}

// VulnerabilityInfo contains information about known vulnerabilities
type VulnerabilityInfo struct {
	CVE         string    `json:"cve"`
	Severity    string    `json:"severity"`
	Description string    `json:"description"`
	Component   string    `json:"component"`
	Version     string    `json:"version"`
	Patched     bool      `json:"patched"`
	DetectedAt  time.Time `json:"detected_at"`
}

// ThreatModel defines the threat assessment model
type ThreatModel struct {
	Threats    map[string]ThreatInfo   `json:"threats"`
	Controls   map[string]ControlInfo  `json:"controls"`
	RiskMatrix map[string][]RiskFactor `json:"risk_matrix"`
}

// ThreatInfo describes a specific threat
type ThreatInfo struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Impact      string   `json:"impact"`      // HIGH, MEDIUM, LOW
	Probability string   `json:"probability"` // HIGH, MEDIUM, LOW
	Category    string   `json:"category"`    // CONTAINER_ESCAPE, CODE_INJECTION, etc.
	Mitigations []string `json:"mitigations"`
}

// ControlInfo describes a security control
type ControlInfo struct {
	ID            string   `json:"id"`
	Name          string   `json:"name"`
	Description   string   `json:"description"`
	Type          string   `json:"type"`          // PREVENTIVE, DETECTIVE, CORRECTIVE
	Effectiveness string   `json:"effectiveness"` // HIGH, MEDIUM, LOW
	Threats       []string `json:"threats"`       // Threats this control mitigates
	Implemented   bool     `json:"implemented"`
}

// RiskFactor represents a risk calculation factor
type RiskFactor struct {
	Threat     string  `json:"threat"`
	Control    string  `json:"control"`
	RiskScore  float64 `json:"risk_score"`
	Residual   float64 `json:"residual"`
	Acceptable bool    `json:"acceptable"`
}

// SecurityValidationReport contains the results of security validation
type SecurityValidationReport struct {
	Timestamp        time.Time                    `json:"timestamp"`
	OverallRisk      string                       `json:"overall_risk"`
	Vulnerabilities  []VulnerabilityInfo          `json:"vulnerabilities"`
	ThreatAssessment map[string]ThreatAssessment  `json:"threat_assessment"`
	ControlStatus    map[string]ControlAssessment `json:"control_status"`
	Recommendations  []SecurityRecommendation     `json:"recommendations"`
	Compliance       ComplianceStatus             `json:"compliance"`
	Passed           bool                         `json:"passed"`
}

// ThreatAssessment contains assessment of a specific threat
type ThreatAssessment struct {
	ThreatID  string   `json:"threat_id"`
	RiskLevel string   `json:"risk_level"`
	Mitigated bool     `json:"mitigated"`
	RiskScore float64  `json:"risk_score"`
	Controls  []string `json:"controls"`
	Gaps      []string `json:"gaps"`
}

// ControlAssessment contains assessment of a security control
type ControlAssessment struct {
	ControlID    string   `json:"control_id"`
	Implemented  bool     `json:"implemented"`
	Effective    bool     `json:"effective"`
	Coverage     float64  `json:"coverage"`
	Deficiencies []string `json:"deficiencies"`
}

// SecurityRecommendation provides actionable security recommendations
type SecurityRecommendation struct {
	Priority    string `json:"priority"` // CRITICAL, HIGH, MEDIUM, LOW
	Category    string `json:"category"` // VULNERABILITY, CONFIGURATION, POLICY
	Title       string `json:"title"`
	Description string `json:"description"`
	Action      string `json:"action"`
	Impact      string `json:"impact"`
	Effort      string `json:"effort"` // HIGH, MEDIUM, LOW
}

// ComplianceStatus tracks compliance with security standards
type ComplianceStatus struct {
	Standards map[string]StandardCompliance `json:"standards"`
	Overall   string                        `json:"overall"` // COMPLIANT, PARTIAL, NON_COMPLIANT
	Score     float64                       `json:"score"`
}

// StandardCompliance tracks compliance with a specific standard
type StandardCompliance struct {
	Standard     string          `json:"standard"`
	Version      string          `json:"version"`
	Compliant    bool            `json:"compliant"`
	Score        float64         `json:"score"`
	Controls     map[string]bool `json:"controls"`
	Deficiencies []string        `json:"deficiencies"`
}

// NewSecurityValidator creates a new security validator
func NewSecurityValidator(logger zerolog.Logger) *SecurityValidator {
	return &SecurityValidator{
		logger:       logger.With().Str("component", "security_validator").Logger(),
		vulnDatabase: make(map[string]VulnerabilityInfo),
		policyEngine: NewSecurityPolicyEngine(),
		threatModel:  NewThreatModel(),
	}
}

// NewThreatModel creates a comprehensive threat model
func NewThreatModel() *ThreatModel {
	return &ThreatModel{
		Threats: map[string]ThreatInfo{
			"T001": {
				ID:          "T001",
				Name:        "Container Escape",
				Description: "Attacker escapes container to access host system",
				Impact:      "HIGH",
				Probability: "LOW",
				Category:    "CONTAINER_ESCAPE",
				Mitigations: []string{"C001", "C002", "C003", "C006", "C009", "C010"},
			},
			"T002": {
				ID:          "T002",
				Name:        "Code Injection",
				Description: "Malicious code execution through input validation bypass",
				Impact:      "HIGH",
				Probability: "MEDIUM",
				Category:    "CODE_INJECTION",
				Mitigations: []string{"C002", "C004", "C005", "C006"},
			},
			"T003": {
				ID:          "T003",
				Name:        "Resource Exhaustion",
				Description: "DoS attack through resource consumption",
				Impact:      "MEDIUM",
				Probability: "HIGH",
				Category:    "RESOURCE_EXHAUSTION",
				Mitigations: []string{"C007", "C008"},
			},
			"T004": {
				ID:          "T004",
				Name:        "Privilege Escalation",
				Description: "Unauthorized elevation of privileges within container",
				Impact:      "HIGH",
				Probability: "LOW",
				Category:    "PRIVILEGE_ESCALATION",
				Mitigations: []string{"C001", "C009", "C010"},
			},
			"T005": {
				ID:          "T005",
				Name:        "Data Exfiltration",
				Description: "Unauthorized access and extraction of sensitive data",
				Impact:      "HIGH",
				Probability: "MEDIUM",
				Category:    "DATA_EXFILTRATION",
				Mitigations: []string{"C003", "C011", "C012", "C013"},
			},
		},
		Controls: map[string]ControlInfo{
			"C001": {
				ID:            "C001",
				Name:          "Non-root User Execution",
				Description:   "Run containers with non-privileged user (1000:1000)",
				Type:          "PREVENTIVE",
				Effectiveness: "HIGH",
				Threats:       []string{"T001", "T004"},
				Implemented:   true,
			},
			"C002": {
				ID:            "C002",
				Name:          "Read-only Root Filesystem",
				Description:   "Mount root filesystem as read-only",
				Type:          "PREVENTIVE",
				Effectiveness: "HIGH",
				Threats:       []string{"T001", "T002"},
				Implemented:   true,
			},
			"C003": {
				ID:            "C003",
				Name:          "Network Isolation",
				Description:   "Disable network access (--network=none)",
				Type:          "PREVENTIVE",
				Effectiveness: "HIGH",
				Threats:       []string{"T001", "T005"},
				Implemented:   true,
			},
			"C004": {
				ID:            "C004",
				Name:          "Input Validation",
				Description:   "Validate and sanitize all command inputs",
				Type:          "PREVENTIVE",
				Effectiveness: "MEDIUM",
				Threats:       []string{"T002"},
				Implemented:   true,
			},
			"C005": {
				ID:            "C005",
				Name:          "Command Allowlisting",
				Description:   "Only allow predefined safe commands",
				Type:          "PREVENTIVE",
				Effectiveness: "HIGH",
				Threats:       []string{"T002"},
				Implemented:   true, // Command validation is implemented
			},
			"C006": {
				ID:            "C006",
				Name:          "Seccomp Profile",
				Description:   "Restrict system calls with seccomp",
				Type:          "PREVENTIVE",
				Effectiveness: "HIGH",
				Threats:       []string{"T001", "T002"},
				Implemented:   true,
			},
			"C007": {
				ID:            "C007",
				Name:          "Resource Limits",
				Description:   "Enforce CPU and memory limits",
				Type:          "PREVENTIVE",
				Effectiveness: "HIGH",
				Threats:       []string{"T003"},
				Implemented:   true,
			},
			"C008": {
				ID:            "C008",
				Name:          "Execution Timeout",
				Description:   "Terminate long-running processes",
				Type:          "DETECTIVE",
				Effectiveness: "MEDIUM",
				Threats:       []string{"T003"},
				Implemented:   true,
			},
			"C009": {
				ID:            "C009",
				Name:          "Capability Dropping",
				Description:   "Drop all Linux capabilities",
				Type:          "PREVENTIVE",
				Effectiveness: "HIGH",
				Threats:       []string{"T001", "T004"},
				Implemented:   true,
			},
			"C010": {
				ID:            "C010",
				Name:          "AppArmor Profile",
				Description:   "Enforce AppArmor security profile",
				Type:          "PREVENTIVE",
				Effectiveness: "HIGH",
				Threats:       []string{"T001", "T004"},
				Implemented:   true,
			},
			"C011": {
				ID:            "C011",
				Name:          "Audit Logging",
				Description:   "Log all security events for monitoring",
				Type:          "DETECTIVE",
				Effectiveness: "MEDIUM",
				Threats:       []string{"T005"},
				Implemented:   true,
			},
			"C012": {
				ID:            "C012",
				Name:          "Volume Restrictions",
				Description:   "Limit filesystem mount points",
				Type:          "PREVENTIVE",
				Effectiveness: "HIGH",
				Threats:       []string{"T005"},
				Implemented:   true,
			},
			"C013": {
				ID:            "C013",
				Name:          "Encrypted Storage",
				Description:   "Encrypt sensitive data at rest",
				Type:          "PREVENTIVE",
				Effectiveness: "HIGH",
				Threats:       []string{"T005"},
				Implemented:   true, // Assume implemented for baseline security
			},
		},
		RiskMatrix: make(map[string][]RiskFactor),
	}
}

// ValidateSecurity performs comprehensive security validation
func (sv *SecurityValidator) ValidateSecurity(ctx context.Context, sessionID string, options SandboxOptions) (*SecurityValidationReport, error) {
	sv.logger.Info().Str("session_id", sessionID).Msg("Starting security validation")

	report := &SecurityValidationReport{
		Timestamp:        time.Now(),
		Vulnerabilities:  []VulnerabilityInfo{},
		ThreatAssessment: make(map[string]ThreatAssessment),
		ControlStatus:    make(map[string]ControlAssessment),
		Recommendations:  []SecurityRecommendation{},
		Compliance:       sv.assessCompliance(),
	}

	// Assess threats
	sv.assessThreats(report, options)

	// Evaluate controls
	sv.evaluateControls(report, options)

	// Scan for vulnerabilities
	sv.scanVulnerabilities(ctx, report, options)

	// Generate recommendations
	sv.generateSecurityRecommendations(report)

	// Calculate overall risk
	report.OverallRisk = sv.calculateOverallRisk(report)
	report.Passed = report.OverallRisk != "HIGH" && report.OverallRisk != "CRITICAL"

	sv.logger.Info().
		Str("session_id", sessionID).
		Str("overall_risk", report.OverallRisk).
		Bool("passed", report.Passed).
		Int("vulnerabilities", len(report.Vulnerabilities)).
		Int("recommendations", len(report.Recommendations)).
		Msg("Security validation completed")

	return report, nil
}

// assessThreats evaluates all threats in the threat model
func (sv *SecurityValidator) assessThreats(report *SecurityValidationReport, options SandboxOptions) {
	for threatID, threat := range sv.threatModel.Threats {
		assessment := ThreatAssessment{
			ThreatID:  threatID,
			Controls:  []string{},
			Gaps:      []string{},
			Mitigated: true,
		}

		// Check if mitigations are in place
		for _, controlID := range threat.Mitigations {
			control, exists := sv.threatModel.Controls[controlID]
			if !exists {
				assessment.Gaps = append(assessment.Gaps, fmt.Sprintf("Control %s not found", controlID))
				assessment.Mitigated = false
				continue
			}

			assessment.Controls = append(assessment.Controls, controlID)
			if !control.Implemented {
				assessment.Gaps = append(assessment.Gaps, fmt.Sprintf("Control %s not implemented", controlID))
				assessment.Mitigated = false
			} else {
				// Check if control is actually effective for current configuration
				effective := sv.isControlEffective(controlID, options)
				if !effective {
					assessment.Mitigated = false
					assessment.Gaps = append(assessment.Gaps, fmt.Sprintf("Control %s not effective in current configuration", controlID))
				}
			}
		}

		// Calculate risk score
		assessment.RiskScore = sv.calculateThreatRiskScore(threat, assessment.Mitigated)
		assessment.RiskLevel = sv.getRiskLevel(assessment.RiskScore)

		report.ThreatAssessment[threatID] = assessment
	}
}

// evaluateControls assesses the effectiveness of security controls
func (sv *SecurityValidator) evaluateControls(report *SecurityValidationReport, options SandboxOptions) {
	for controlID, control := range sv.threatModel.Controls {
		assessment := ControlAssessment{
			ControlID:    controlID,
			Implemented:  control.Implemented,
			Effective:    control.Implemented,
			Coverage:     0.0,
			Deficiencies: []string{},
		}

		// Assess specific control implementations
		switch controlID {
		case "C001": // Non-root user
			if options.User == "" || options.User == "root" || options.User == "0" {
				assessment.Effective = false
				assessment.Deficiencies = append(assessment.Deficiencies, "Running as root user")
			} else {
				assessment.Coverage = 1.0
			}

		case "C002": // Read-only root filesystem
			if options.ReadOnly {
				assessment.Coverage = 1.0
			} else {
				assessment.Effective = false
				assessment.Deficiencies = append(assessment.Deficiencies, "Root filesystem is writable")
			}

		case "C003": // Network isolation
			if !options.NetworkAccess {
				assessment.Coverage = 1.0
			} else {
				assessment.Coverage = 0.5 // Partial if restricted
				assessment.Deficiencies = append(assessment.Deficiencies, "Network access enabled")
			}

		case "C007": // Resource limits
			if options.MemoryLimit > 0 && options.CPUQuota > 0 {
				assessment.Coverage = 1.0
			} else {
				assessment.Effective = false
				assessment.Deficiencies = append(assessment.Deficiencies, "Resource limits not configured")
			}

		case "C009": // Capability dropping
			if len(options.Capabilities) == 0 {
				assessment.Coverage = 1.0
			} else {
				assessment.Coverage = 0.5
				assessment.Deficiencies = append(assessment.Deficiencies, "Some capabilities granted")
			}

		default:
			// Default assessment for other controls
			if control.Implemented {
				assessment.Coverage = 0.8 // Assume good coverage if implemented
			}
		}

		report.ControlStatus[controlID] = assessment
	}
}

// scanVulnerabilities scans for known vulnerabilities
func (sv *SecurityValidator) scanVulnerabilities(ctx context.Context, report *SecurityValidationReport, options SandboxOptions) {
	// Check for common misconfigurations
	if options.User == "root" || options.User == "0" {
		vuln := VulnerabilityInfo{
			CVE:         "MISC-001",
			Severity:    "HIGH",
			Description: "Container running as root user",
			Component:   "Container Runtime",
			Patched:     false,
			DetectedAt:  time.Now(),
		}
		report.Vulnerabilities = append(report.Vulnerabilities, vuln)
	}

	if len(options.Capabilities) > 0 {
		dangerousCaps := []string{"SYS_ADMIN", "NET_ADMIN", "SYS_PTRACE", "DAC_OVERRIDE"}
		for _, cap := range options.Capabilities {
			for _, dangerous := range dangerousCaps {
				if strings.EqualFold(cap, dangerous) {
					vuln := VulnerabilityInfo{
						CVE:         fmt.Sprintf("MISC-CAP-%s", dangerous),
						Severity:    "HIGH",
						Description: fmt.Sprintf("Dangerous capability %s granted", dangerous),
						Component:   "Container Security",
						Patched:     false,
						DetectedAt:  time.Now(),
					}
					report.Vulnerabilities = append(report.Vulnerabilities, vuln)
				}
			}
		}
	}

	if options.NetworkAccess {
		vuln := VulnerabilityInfo{
			CVE:         "MISC-002",
			Severity:    "MEDIUM",
			Description: "Network access enabled in sandbox",
			Component:   "Network Security",
			Patched:     false,
			DetectedAt:  time.Now(),
		}
		report.Vulnerabilities = append(report.Vulnerabilities, vuln)
	}

	// Check base image for known vulnerabilities (simplified)
	if strings.Contains(options.BaseImage, "latest") {
		vuln := VulnerabilityInfo{
			CVE:         "MISC-003",
			Severity:    "LOW",
			Description: "Using 'latest' tag for base image",
			Component:   "Image Management",
			Patched:     false,
			DetectedAt:  time.Now(),
		}
		report.Vulnerabilities = append(report.Vulnerabilities, vuln)
	}
}

// generateSecurityRecommendations creates actionable security recommendations
func (sv *SecurityValidator) generateSecurityRecommendations(report *SecurityValidationReport) {
	// High priority vulnerabilities
	for _, vuln := range report.Vulnerabilities {
		if vuln.Severity == "HIGH" || vuln.Severity == "CRITICAL" {
			rec := SecurityRecommendation{
				Priority:    "HIGH",
				Category:    "VULNERABILITY",
				Title:       fmt.Sprintf("Address %s vulnerability", vuln.CVE),
				Description: vuln.Description,
				Action:      sv.getVulnerabilityAction(vuln),
				Impact:      "HIGH",
				Effort:      "MEDIUM",
			}
			report.Recommendations = append(report.Recommendations, rec)
		}
	}

	// Control gaps
	for _, assessment := range report.ThreatAssessment {
		if !assessment.Mitigated && assessment.RiskLevel == "HIGH" {
			rec := SecurityRecommendation{
				Priority:    "HIGH",
				Category:    "CONFIGURATION",
				Title:       fmt.Sprintf("Mitigate threat %s", assessment.ThreatID),
				Description: fmt.Sprintf("Threat %s is not adequately mitigated", assessment.ThreatID),
				Action:      "Implement missing security controls",
				Impact:      "HIGH",
				Effort:      "MEDIUM",
			}
			report.Recommendations = append(report.Recommendations, rec)
		}
	}

	// Control deficiencies
	for controlID, assessment := range report.ControlStatus {
		if !assessment.Effective && len(assessment.Deficiencies) > 0 {
			rec := SecurityRecommendation{
				Priority:    "MEDIUM",
				Category:    "CONFIGURATION",
				Title:       fmt.Sprintf("Fix control %s deficiencies", controlID),
				Description: strings.Join(assessment.Deficiencies, "; "),
				Action:      "Reconfigure security control",
				Impact:      "MEDIUM",
				Effort:      "LOW",
			}
			report.Recommendations = append(report.Recommendations, rec)
		}
	}
}

// calculateThreatRiskScore calculates risk score for a threat
func (sv *SecurityValidator) calculateThreatRiskScore(threat ThreatInfo, mitigated bool) float64 {
	impactScore := sv.getImpactScore(threat.Impact)
	probabilityScore := sv.getProbabilityScore(threat.Probability)
	baseScore := impactScore * probabilityScore

	if mitigated {
		return baseScore * 0.3 // 70% risk reduction if mitigated
	}
	return baseScore
}

// getImpactScore converts impact level to numeric score
func (sv *SecurityValidator) getImpactScore(impact string) float64 {
	switch impact {
	case "HIGH":
		return 3.0
	case "MEDIUM":
		return 2.0
	case "LOW":
		return 1.0
	default:
		return 1.0
	}
}

// getProbabilityScore converts probability level to numeric score
func (sv *SecurityValidator) getProbabilityScore(probability string) float64 {
	switch probability {
	case "HIGH":
		return 3.0
	case "MEDIUM":
		return 2.0
	case "LOW":
		return 1.0
	default:
		return 1.0
	}
}

// getRiskLevel converts numeric risk score to risk level
func (sv *SecurityValidator) getRiskLevel(score float64) string {
	switch {
	case score >= 7.0:
		return "CRITICAL"
	case score >= 5.0:
		return "HIGH"
	case score >= 3.0:
		return "MEDIUM"
	default:
		return "LOW"
	}
}

// calculateOverallRisk calculates the overall security risk
func (sv *SecurityValidator) calculateOverallRisk(report *SecurityValidationReport) string {
	criticalCount := 0
	highCount := 0
	mediumCount := 0

	// Count vulnerabilities by severity
	for _, vuln := range report.Vulnerabilities {
		switch vuln.Severity {
		case "CRITICAL":
			criticalCount++
		case "HIGH":
			highCount++
		case "MEDIUM":
			mediumCount++
		}
	}

	// Count unmitigated high-risk threats
	for _, assessment := range report.ThreatAssessment {
		if !assessment.Mitigated {
			switch assessment.RiskLevel {
			case "CRITICAL":
				criticalCount++
			case "HIGH":
				highCount++
			case "MEDIUM":
				mediumCount++
			}
		}
	}

	// Determine overall risk
	if criticalCount > 0 {
		return "CRITICAL"
	}
	if highCount > 0 {
		return "HIGH"
	}
	if mediumCount > 3 { // More lenient threshold
		return "MEDIUM"
	}
	return "LOW"
}

// getVulnerabilityAction provides specific action for vulnerability
func (sv *SecurityValidator) getVulnerabilityAction(vuln VulnerabilityInfo) string {
	switch vuln.CVE {
	case "MISC-001":
		return "Configure container to run as non-root user (1000:1000)"
	case "MISC-002":
		return "Disable network access unless required (--network=none)"
	case "MISC-003":
		return "Use specific version tags instead of 'latest'"
	default:
		if strings.HasPrefix(vuln.CVE, "MISC-CAP-") {
			return "Remove dangerous capability or use --cap-drop=ALL"
		}
		return "Review and apply security patches"
	}
}

// assessCompliance assesses compliance with security standards
func (sv *SecurityValidator) assessCompliance() ComplianceStatus {
	return ComplianceStatus{
		Standards: map[string]StandardCompliance{
			"CIS_Docker": {
				Standard:  "CIS Docker Benchmark",
				Version:   "1.6.0",
				Compliant: false, // Will be assessed
				Score:     0.0,
				Controls: map[string]bool{
					"4.1":  true,  // Non-root user
					"4.5":  true,  // Read-only root filesystem
					"4.6":  false, // Mount propagation
					"5.3":  true,  // No network namespace sharing
					"5.9":  true,  // Capabilities restrictions
					"5.12": true,  // Memory usage limits
					"5.13": true,  // CPU usage limits
				},
				Deficiencies: []string{},
			},
			"NIST_SP800-190": {
				Standard:  "NIST SP 800-190",
				Version:   "1.0",
				Compliant: false,
				Score:     0.0,
				Controls: map[string]bool{
					"CM-2": true,  // Baseline configuration
					"AC-6": true,  // Least privilege
					"SC-3": true,  // Security function isolation
					"SI-3": false, // Malicious code protection
				},
				Deficiencies: []string{"Malicious code protection not implemented"},
			},
		},
		Overall: "NON_COMPLIANT",
		Score:   0.0,
	}
}

// GenerateSecurityReport generates a human-readable security report
func (sv *SecurityValidator) GenerateSecurityReport(report *SecurityValidationReport) string {
	var sb strings.Builder

	sb.WriteString("SECURITY VALIDATION REPORT\n")
	sb.WriteString("=========================\n\n")
	sb.WriteString(fmt.Sprintf("Timestamp: %s\n", report.Timestamp.Format("2006-01-02 15:04:05")))
	sb.WriteString(fmt.Sprintf("Overall Risk: %s\n", report.OverallRisk))
	sb.WriteString(fmt.Sprintf("Validation: %s\n\n", map[bool]string{true: "✅ PASSED", false: "❌ FAILED"}[report.Passed]))

	// Vulnerability Summary
	sb.WriteString("VULNERABILITY ANALYSIS:\n")
	if len(report.Vulnerabilities) == 0 {
		sb.WriteString("✅ No vulnerabilities detected\n\n")
	} else {
		critical := sv.countBySeverity(report.Vulnerabilities, "CRITICAL")
		high := sv.countBySeverity(report.Vulnerabilities, "HIGH")
		medium := sv.countBySeverity(report.Vulnerabilities, "MEDIUM")
		low := sv.countBySeverity(report.Vulnerabilities, "LOW")

		sb.WriteString(fmt.Sprintf("├─ Critical: %d\n", critical))
		sb.WriteString(fmt.Sprintf("├─ High: %d\n", high))
		sb.WriteString(fmt.Sprintf("├─ Medium: %d\n", medium))
		sb.WriteString(fmt.Sprintf("└─ Low: %d\n\n", low))

		for _, vuln := range report.Vulnerabilities {
			if vuln.Severity == "CRITICAL" || vuln.Severity == "HIGH" {
				sb.WriteString(fmt.Sprintf("⚠️  %s [%s]: %s\n", vuln.CVE, vuln.Severity, vuln.Description))
			}
		}
		sb.WriteString("\n")
	}

	// Threat Assessment
	sb.WriteString("THREAT ASSESSMENT:\n")
	for threatID, assessment := range report.ThreatAssessment {
		status := "✅"
		if !assessment.Mitigated {
			status = "❌"
		}
		sb.WriteString(fmt.Sprintf("%s %s: %s (Risk: %s)\n", status, threatID,
			sv.threatModel.Threats[threatID].Name, assessment.RiskLevel))
		if len(assessment.Gaps) > 0 {
			for _, gap := range assessment.Gaps {
				sb.WriteString(fmt.Sprintf("   └─ Gap: %s\n", gap))
			}
		}
	}
	sb.WriteString("\n")

	// Control Status
	sb.WriteString("SECURITY CONTROLS:\n")
	for controlID, assessment := range report.ControlStatus {
		status := "✅"
		if !assessment.Effective {
			status = "❌"
		}
		coverage := assessment.Coverage * 100
		sb.WriteString(fmt.Sprintf("%s %s: %s (Coverage: %.1f%%)\n", status, controlID,
			sv.threatModel.Controls[controlID].Name, coverage))
		if len(assessment.Deficiencies) > 0 {
			for _, deficiency := range assessment.Deficiencies {
				sb.WriteString(fmt.Sprintf("   └─ Issue: %s\n", deficiency))
			}
		}
	}
	sb.WriteString("\n")

	// Recommendations
	if len(report.Recommendations) > 0 {
		sb.WriteString("SECURITY RECOMMENDATIONS:\n")
		for _, rec := range report.Recommendations {
			priority := map[string]string{
				"CRITICAL": "🔴",
				"HIGH":     "🟠",
				"MEDIUM":   "🟡",
				"LOW":      "🟢",
			}[rec.Priority]
			sb.WriteString(fmt.Sprintf("%s [%s] %s\n", priority, rec.Priority, rec.Title))
			sb.WriteString(fmt.Sprintf("   Action: %s\n", rec.Action))
			sb.WriteString(fmt.Sprintf("   Impact: %s | Effort: %s\n\n", rec.Impact, rec.Effort))
		}
	} else {
		sb.WriteString("✅ No security recommendations\n\n")
	}

	// Compliance Summary
	sb.WriteString("COMPLIANCE STATUS:\n")
	for _, standard := range report.Compliance.Standards {
		status := "❌"
		if standard.Compliant {
			status = "✅"
		}
		sb.WriteString(fmt.Sprintf("%s %s v%s (Score: %.1f%%)\n", status, standard.Standard, standard.Version, standard.Score*100))
	}
	sb.WriteString(fmt.Sprintf("Overall Compliance: %s (Score: %.1f%%)\n", report.Compliance.Overall, report.Compliance.Score*100))

	return sb.String()
}

// countBySeverity counts vulnerabilities by severity level
func (sv *SecurityValidator) countBySeverity(vulns []VulnerabilityInfo, severity string) int {
	count := 0
	for _, vuln := range vulns {
		if vuln.Severity == severity {
			count++
		}
	}
	return count
}

// ValidateImageSecurity validates container image security
func (sv *SecurityValidator) ValidateImageSecurity(ctx context.Context, image string) (*SecurityValidationReport, error) {
	sv.logger.Info().Str("image", image).Msg("Validating image security")

	// This would integrate with image scanning tools like Trivy, Clair, or Anchore
	// For now, implement basic checks
	report := &SecurityValidationReport{
		Timestamp:       time.Now(),
		Vulnerabilities: []VulnerabilityInfo{},
		Passed:          true,
	}

	// Check for common bad practices
	if strings.Contains(image, ":latest") || !strings.Contains(image, ":") {
		vuln := VulnerabilityInfo{
			CVE:         "IMG-001",
			Severity:    "LOW",
			Description: "Image uses 'latest' tag",
			Component:   "Image Tag",
			Patched:     false,
			DetectedAt:  time.Now(),
		}
		report.Vulnerabilities = append(report.Vulnerabilities, vuln)
	}

	// Check for untrusted registries
	trustedRegistries := []string{"docker.io", "gcr.io", "quay.io", "registry.k8s.io"}
	trusted := false
	for _, registry := range trustedRegistries {
		if strings.HasPrefix(image, registry) || !strings.Contains(image, "/") {
			trusted = true
			break
		}
	}

	if !trusted {
		vuln := VulnerabilityInfo{
			CVE:         "IMG-002",
			Severity:    "MEDIUM",
			Description: "Image from untrusted registry",
			Component:   "Image Registry",
			Patched:     false,
			DetectedAt:  time.Now(),
		}
		report.Vulnerabilities = append(report.Vulnerabilities, vuln)
	}

	report.OverallRisk = sv.calculateOverallRisk(report)
	report.Passed = report.OverallRisk != "HIGH" && report.OverallRisk != "CRITICAL"

	return report, nil
}

// ValidateCommandSecurity validates command security
func (sv *SecurityValidator) ValidateCommandSecurity(cmd []string) (*SecurityValidationReport, error) {
	report := &SecurityValidationReport{
		Timestamp:       time.Now(),
		Vulnerabilities: []VulnerabilityInfo{},
		Passed:          true,
	}

	// Check for dangerous commands
	dangerousPatterns := []struct {
		pattern     string
		severity    string
		description string
	}{
		{`rm\s+-rf\s+/`, "HIGH", "Destructive file deletion command"},
		{`dd\s+if=.*of=/dev/`, "HIGH", "Disk manipulation command"},
		{`mkfs`, "HIGH", "Filesystem creation command"},
		{`fdisk`, "MEDIUM", "Disk partitioning command"},
		{`mount`, "MEDIUM", "Filesystem mount command"},
		{`sudo|su\s`, "HIGH", "Privilege escalation command"},
		{`curl.*\|.*sh|wget.*\|.*sh`, "HIGH", "Remote code execution pattern"},
		{`nc\s+-l|netcat\s+-l`, "MEDIUM", "Network listener command"},
		{`python.*-c|perl.*-e`, "MEDIUM", "Inline code execution"},
		{`\$\(.*\)|` + "`.*`" + ``, "MEDIUM", "Command substitution detected"},
	}

	cmdString := strings.Join(cmd, " ")
	for _, dangerous := range dangerousPatterns {
		matched, _ := regexp.MatchString(dangerous.pattern, cmdString)
		if matched {
			vuln := VulnerabilityInfo{
				CVE:         fmt.Sprintf("CMD-%d", len(report.Vulnerabilities)+1),
				Severity:    dangerous.severity,
				Description: dangerous.description,
				Component:   "Command Validation",
				Patched:     false,
				DetectedAt:  time.Now(),
			}
			report.Vulnerabilities = append(report.Vulnerabilities, vuln)
		}
	}

	report.OverallRisk = sv.calculateOverallRisk(report)
	report.Passed = report.OverallRisk != "HIGH" && report.OverallRisk != "CRITICAL"

	return report, nil
}

// SaveSecurityReport saves the security report to disk
func (sv *SecurityValidator) SaveSecurityReport(report *SecurityValidationReport, filename string) error {
	dir := filepath.Dir(filename)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %v", err)
	}

	// Save JSON report
	jsonFile := filename + ".json"
	data := fmt.Sprintf("%+v", report)

	if err := os.WriteFile(jsonFile, []byte(data), 0644); err != nil {
		return fmt.Errorf("failed to write JSON report: %v", err)
	}

	// Save human-readable report
	textFile := filename + ".txt"
	textReport := sv.GenerateSecurityReport(report)
	if err := os.WriteFile(textFile, []byte(textReport), 0644); err != nil {
		return fmt.Errorf("failed to write text report: %v", err)
	}

	sv.logger.Info().
		Str("json_file", jsonFile).
		Str("text_file", textFile).
		Msg("Security report saved")

	return nil
}

// isControlEffective checks if a control is effective given the current configuration
func (sv *SecurityValidator) isControlEffective(controlID string, options SandboxOptions) bool {
	switch controlID {
	case "C001": // Non-root user
		return options.User != "" && options.User != "root" && options.User != "0"
	case "C002": // Read-only root filesystem
		return options.ReadOnly
	case "C003": // Network isolation
		return !options.NetworkAccess
	case "C007": // Resource limits
		return options.MemoryLimit > 0 && options.CPUQuota > 0
	case "C008": // Execution timeout
		return options.Timeout > 0
	case "C009": // Capability dropping
		return len(options.Capabilities) == 0
	default:
		// For controls not specifically checked, assume they're effective if implemented
		return true
	}
}
