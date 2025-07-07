package security

import (
	"strings"

	"github.com/rs/zerolog"
)

// ThreatAssessor evaluates and assesses security threats
type ThreatAssessor struct {
	logger      zerolog.Logger
	threatModel *ThreatModel
}

// NewThreatAssessor creates a new threat assessor
func NewThreatAssessor(logger zerolog.Logger) *ThreatAssessor {
	return &ThreatAssessor{
		logger:      logger.With().Str("component", "threat_assessor").Logger(),
		threatModel: initializeDefaultThreatModel(),
	}
}

// AssessThreats evaluates threats for a given operation
func (ta *ThreatAssessor) AssessThreats(operation string, params map[string]interface{}) []ThreatInfo {
	threats := make([]ThreatInfo, 0)

	// Check operation-specific threats
	for _, threat := range ta.threatModel.Threats {
		if ta.operationMatchesThreat(operation, params, threat) {
			threats = append(threats, threat)
		}
	}

	ta.logger.Debug().
		Str("operation", operation).
		Int("threats_found", len(threats)).
		Msg("Threat assessment completed")

	return threats
}

// CalculateRiskScore calculates an overall risk score based on threats
func (ta *ThreatAssessor) CalculateRiskScore(threats []ThreatInfo) float64 {
	if len(threats) == 0 {
		return 0.0
	}

	totalScore := 0.0
	for _, threat := range threats {
		impactScore := ta.getImpactScore(threat.Impact)
		probabilityScore := ta.getProbabilityScore(threat.Probability)

		// Risk = Impact × Probability
		threatScore := impactScore * probabilityScore

		// Apply risk factors if available
		if factors, exists := ta.threatModel.RiskMatrix[threat.ID]; exists {
			for _, factor := range factors {
				threatScore *= factor.Weight
			}
		}

		totalScore += threatScore
	}

	// Normalize to 0-100 scale
	normalizedScore := (totalScore / float64(len(threats))) * 100
	if normalizedScore > 100 {
		normalizedScore = 100
	}

	return normalizedScore
}

// GetMitigations returns recommended mitigations for identified threats
func (ta *ThreatAssessor) GetMitigations(threats []ThreatInfo) []string {
	mitigationSet := make(map[string]bool)

	for _, threat := range threats {
		for _, mitigation := range threat.Mitigations {
			mitigationSet[mitigation] = true
		}
	}

	mitigations := make([]string, 0, len(mitigationSet))
	for mitigation := range mitigationSet {
		mitigations = append(mitigations, mitigation)
	}

	return mitigations
}

// GetControls returns security controls that address the identified threats
func (ta *ThreatAssessor) GetControls(threats []ThreatInfo) []ControlInfo {
	controls := make([]ControlInfo, 0)
	controlMap := make(map[string]bool)

	for _, threat := range threats {
		for _, control := range ta.threatModel.Controls {
			for _, threatID := range control.Threats {
				if threatID == threat.ID && !controlMap[control.ID] {
					controls = append(controls, control)
					controlMap[control.ID] = true
				}
			}
		}
	}

	return controls
}

// operationMatchesThreat checks if an operation matches threat criteria
func (ta *ThreatAssessor) operationMatchesThreat(operation string, params map[string]interface{}, threat ThreatInfo) bool {
	switch threat.Category {
	case "CONTAINER_ESCAPE":
		return ta.checkContainerEscape(operation, params)
	case "CODE_INJECTION":
		return ta.checkCodeInjection(operation, params)
	case "PATH_TRAVERSAL":
		return ta.checkPathTraversal(operation, params)
	case "PRIVILEGE_ESCALATION":
		return ta.checkPrivilegeEscalation(operation, params)
	case "DATA_EXPOSURE":
		return ta.checkDataExposure(operation, params)
	default:
		return false
	}
}

// checkContainerEscape checks for container escape threats
func (ta *ThreatAssessor) checkContainerEscape(operation string, params map[string]interface{}) bool {
	// Check for operations that might lead to container escape
	dangerousOps := []string{"docker", "container", "mount", "namespace", "privileged"}

	for _, op := range dangerousOps {
		if strings.Contains(strings.ToLower(operation), op) {
			return true
		}
	}

	// Check for privileged mode
	if privileged, ok := params["privileged"].(bool); ok && privileged {
		return true
	}

	// Check for dangerous capabilities
	if caps, ok := params["capabilities"].([]string); ok {
		dangerousCaps := []string{"SYS_ADMIN", "SYS_MODULE", "SYS_RAWIO"}
		for _, cap := range caps {
			for _, dangerous := range dangerousCaps {
				if cap == dangerous {
					return true
				}
			}
		}
	}

	return false
}

// checkCodeInjection checks for code injection threats
func (ta *ThreatAssessor) checkCodeInjection(_ string, params map[string]interface{}) bool {
	// Check command parameters for injection patterns
	if cmd, ok := params["command"].(string); ok {
		dangerousPatterns := []string{
			";", "&&", "||", "|", "`", "$", "(", ")",
			"eval", "exec", "system", "spawn",
		}
		for _, pattern := range dangerousPatterns {
			if strings.Contains(cmd, pattern) {
				return true
			}
		}
	}

	// Check for script execution
	if script, ok := params["script"].(string); ok {
		if len(script) > 0 {
			return true // Any script execution is potentially dangerous
		}
	}

	return false
}

// checkPathTraversal checks for path traversal threats
func (ta *ThreatAssessor) checkPathTraversal(_ string, params map[string]interface{}) bool {
	// Check path parameters
	pathKeys := []string{"path", "file", "directory", "source", "destination"}

	for _, key := range pathKeys {
		if path, ok := params[key].(string); ok {
			// Check for path traversal patterns
			if strings.Contains(path, "..") ||
				strings.Contains(path, "../") ||
				strings.Contains(path, "..\\") {
				return true
			}

			// Check for sensitive system paths
			sensitivePaths := []string{
				"/etc/", "/proc/", "/sys/", "/dev/",
				"C:\\Windows\\", "C:\\System",
			}
			for _, sensitive := range sensitivePaths {
				if strings.HasPrefix(path, sensitive) {
					return true
				}
			}
		}
	}

	return false
}

// checkPrivilegeEscalation checks for privilege escalation threats
func (ta *ThreatAssessor) checkPrivilegeEscalation(operation string, params map[string]interface{}) bool {
	// Check for operations that change privileges
	if strings.Contains(operation, "setuid") ||
		strings.Contains(operation, "sudo") ||
		strings.Contains(operation, "root") {
		return true
	}

	// Check for user changes
	if user, ok := params["user"].(string); ok {
		if user == "root" || user == "0" {
			return true
		}
	}

	// Check for capability additions
	if _, ok := params["add_capabilities"]; ok {
		return true
	}

	return false
}

// checkDataExposure checks for data exposure threats
func (ta *ThreatAssessor) checkDataExposure(operation string, params map[string]interface{}) bool {
	// Check for operations that might expose data
	if strings.Contains(operation, "export") ||
		strings.Contains(operation, "publish") ||
		strings.Contains(operation, "share") {
		return true
	}

	// Check for volume mounts that might expose sensitive data
	if volumes, ok := params["volumes"].([]string); ok {
		for _, volume := range volumes {
			if strings.Contains(volume, "/etc") ||
				strings.Contains(volume, "/root") ||
				strings.Contains(volume, "/home") {
				return true
			}
		}
	}

	return false
}

// getImpactScore converts impact level to numeric score
func (ta *ThreatAssessor) getImpactScore(impact string) float64 {
	switch strings.ToUpper(impact) {
	case "CRITICAL":
		return 1.0
	case "HIGH":
		return 0.8
	case "MEDIUM":
		return 0.5
	case "LOW":
		return 0.2
	default:
		return 0.1
	}
}

// getProbabilityScore converts probability level to numeric score
func (ta *ThreatAssessor) getProbabilityScore(probability string) float64 {
	switch strings.ToUpper(probability) {
	case "HIGH":
		return 0.9
	case "MEDIUM":
		return 0.5
	case "LOW":
		return 0.2
	default:
		return 0.1
	}
}
