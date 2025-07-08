package analyze

import (
	"fmt"
	"strconv"
	"strings"

	constants "github.com/Azure/container-kit/pkg/mcp/core"
)

// calculateValidationScore computes an overall validation score (0-100) based on
// the validation results including errors, warnings, security issues, and best practices.
func (t *AtomicValidateDockerfileTool) calculateValidationScore(result *ExtendedValidationResult) int {
	score := 100

	// Deduct points for errors and critical issues
	score -= len(result.Errors) * 10
	score -= result.CriticalIssues * 15

	// Deduct points for security issues
	score -= len(result.SecurityIssues) * 15

	// Deduct points for warnings
	score -= len(result.Warnings) * 3

	// Add bonus points for good practices
	if result.SecurityAnalysis != nil {
		if extSec, ok := interface{}(result.SecurityAnalysis).(*ExtendedSecurityAnalysis); ok {
			if extSec.UsesPackagePin {
				score += 5
			}
			if !extSec.RunsAsRoot {
				score += 10
			}
			if extSec.SecurityScore > 80 {
				score += 5
			}
		}
	}

	// Ensure score is within valid range
	if score < 0 {
		score = 0
	}
	if score > 100 {
		score = 100
	}

	return score
}

// analyzeBaseImage performs comprehensive analysis of the base image used in the Dockerfile.
// This includes checking for official images, trusted registries, version pinning, and alternatives.
func (t *AtomicValidateDockerfileTool) analyzeBaseImage(lines []string) ExtendedBaseImageAnalysis {
	analysis := ExtendedBaseImageAnalysis{
		BaseImageAnalysis: BaseImageAnalysis{
			Recommendations: make([]string, 0),
		},
		Alternatives: make([]string, 0),
	}

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(strings.ToUpper(trimmed), "FROM") {
			parts := strings.Fields(trimmed)
			if len(parts) >= 2 {
				analysis.Image = parts[1]

				// Determine registry and trust status
				if strings.Contains(analysis.Image, "/") {
					analysis.Registry = strings.Split(analysis.Image, "/")[0]
					analysis.IsTrusted = isTrustedRegistry(analysis.Registry)
				} else {
					analysis.Registry = "docker.io"
					analysis.IsTrusted = true
				}

				// Check if it's an official image
				analysis.IsOfficial = isOfficialImage(analysis.Image)
				analysis.Official = analysis.IsOfficial
				analysis.Trusted = analysis.IsTrusted

				// Check for version pinning
				if strings.Contains(analysis.Image, ":latest") || !strings.Contains(analysis.Image, ":") {
					analysis.Recommendations = append(analysis.Recommendations, "Use specific version tags instead of 'latest'")
					analysis.HasKnownVulns = true // Assume latest might have vulns
				}

				// Suggest alternatives
				analysis.Alternatives = suggestAlternativeImages(analysis.Image)
				analysis.AlternativeImages = analysis.Alternatives
			}
			break
		}
	}

	return analysis
}

// analyzeDockerfileLayers analyzes the layer structure of the Dockerfile to identify
// optimization opportunities and problematic patterns.
func (t *AtomicValidateDockerfileTool) analyzeDockerfileLayers(lines []string) ExtendedLayerAnalysis {
	analysis := ExtendedLayerAnalysis{
		LayerAnalysis: LayerAnalysis{
			OptimizationTips: make([]string, 0),
		},
		ProblematicSteps: make([]ProblematicStep, 0),
		Optimizations:    make([]LayerOptimization, 0),
	}

	runCommands := 0
	cacheableSteps := 0

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(strings.ToUpper(trimmed), "RUN") {
			runCommands++
			analysis.TotalLayers++

			// Check if step is cacheable
			if !strings.Contains(trimmed, "apt-get update") && !strings.Contains(trimmed, "npm install") {
				cacheableSteps++
			}

			// Identify problematic patterns
			if strings.Count(trimmed, "&&") == 0 && runCommands > 1 {
				analysis.ProblematicSteps = append(analysis.ProblematicSteps, ProblematicStep{
					Line:        i + 1,
					Instruction: "RUN",
					Issue:       "Multiple RUN commands can be combined",
					Impact:      "Larger image size due to additional layers",
				})
			}
		} else if strings.HasPrefix(strings.ToUpper(trimmed), "COPY") || strings.HasPrefix(strings.ToUpper(trimmed), "ADD") {
			analysis.TotalLayers++
			cacheableSteps++
		}
	}

	analysis.CacheableSteps = cacheableSteps
	analysis.CacheableLayers = cacheableSteps

	// Generate optimization recommendations
	if runCommands > 3 {
		analysis.Optimizations = append(analysis.Optimizations, LayerOptimization{
			Type:        "layer_consolidation",
			Description: "Combine multiple RUN commands",
			Before:      "RUN cmd1\nRUN cmd2\nRUN cmd3",
			After:       "RUN cmd1 && \\\n    cmd2 && \\\n    cmd3",
			Benefit:     "Reduces image layers and size",
		})
	}

	return analysis
}

// performSecurityAnalysis conducts comprehensive security analysis of the Dockerfile
// including user permissions, exposed ports, secrets detection, and package pinning.
func (t *AtomicValidateDockerfileTool) performSecurityAnalysis(lines []string) ExtendedSecurityAnalysis {
	analysis := ExtendedSecurityAnalysis{
		SecurityAnalysis: SecurityAnalysis{
			ExposedPorts:     make([]int, 0),
			SecurityFeatures: make(map[string]bool),
		},
	}

	hasUser := false
	analysis.UsesPackagePin = true // Assume true until proven otherwise
	securityScore := 100

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		upper := strings.ToUpper(trimmed)

		// Check for non-root user
		if strings.HasPrefix(upper, "USER") && !strings.Contains(trimmed, "root") {
			hasUser = true
		}

		// Analyze exposed ports
		if strings.HasPrefix(upper, "EXPOSE") {
			parts := strings.Fields(trimmed)
			for _, part := range parts[1:] {
				if port, err := strconv.Atoi(strings.TrimSuffix(part, "/tcp")); err == nil {
					analysis.ExposedPorts = append(analysis.ExposedPorts, port)
				}
			}
		}

		// Detect potential secrets
		if strings.Contains(upper, "PASSWORD") || strings.Contains(upper, "SECRET") || strings.Contains(upper, "KEY") {
			analysis.HasSecrets = true
			analysis.HardcodedSecrets = true
			securityScore -= 30
		}

		// Check for package pinning
		if strings.Contains(trimmed, "apt-get install") && !strings.Contains(trimmed, "=") {
			analysis.UsesPackagePin = false
			securityScore -= 10
		}
	}

	// Analyze user permissions
	analysis.RunsAsRoot = !hasUser
	analysis.RunAsRoot = analysis.RunsAsRoot
	if analysis.RunsAsRoot {
		securityScore -= 20
	}

	// Finalize security score
	analysis.SecurityScore = securityScore
	analysis.Score = float64(securityScore)
	if analysis.SecurityScore < 0 {
		analysis.SecurityScore = 0
		analysis.Score = 0
	}

	return analysis
}

// generateOptimizationTips analyzes the Dockerfile for optimization opportunities
// including layer consolidation, caching improvements, and build efficiency.
func (t *AtomicValidateDockerfileTool) generateOptimizationTips(lines []string, layerAnalysis ExtendedLayerAnalysis) []OptimizationTip {
	tips := make([]OptimizationTip, 0)

	// Check for too many layers
	if layerAnalysis.TotalLayers > 10 {
		tips = append(tips, OptimizationTip{
			Type:            "layer_consolidation",
			Description:     "Too many layers detected",
			CurrentImpact:   "size_reduction",
			Implementation:  "Combine related RUN commands using && to reduce layers",
			PotentialSaving: "10-20% size reduction",
		})
	}

	// Analyze caching patterns
	copyBeforeRun := false
	lastCopyLine := -1
	lastRunLine := -1

	for i, line := range lines {
		trimmed := strings.TrimSpace(strings.ToUpper(line))
		if strings.HasPrefix(trimmed, "COPY") {
			lastCopyLine = i
		} else if strings.HasPrefix(trimmed, "RUN") {
			lastRunLine = i
			if lastCopyLine > lastRunLine {
				copyBeforeRun = true
			}
		}
	}

	// Check for cache-breaking patterns
	if copyBeforeRun {
		tips = append(tips, OptimizationTip{
			Type:           "cache_optimization",
			Line:           lastCopyLine + 1,
			Description:    "COPY after RUN breaks Docker cache",
			CurrentImpact:  "build_speed",
			Implementation: "Move COPY commands before RUN commands when possible",
		})
	}

	return tips
}

// generateCorrectedDockerfile creates a corrected version of the Dockerfile
// by applying common fixes for validation errors and best practices.
func (t *AtomicValidateDockerfileTool) generateCorrectedDockerfile(dockerfileContent string, _ *types.BuildValidationResult) (string, []string) {
	fixes := make([]string, 0)
	lines := strings.Split(dockerfileContent, "\n")
	corrected := make([]string, len(lines))
	copy(corrected, lines)

	for i, line := range corrected {
		lineNum := i + 1
		trimmed := strings.TrimSpace(line)

		// Add missing FROM instruction
		if i == 0 && !strings.HasPrefix(strings.ToUpper(trimmed), "FROM") {
			corrected = append([]string{"FROM alpine:latest"}, corrected...)
			fixes = append(fixes, "Added missing FROM instruction")
			continue
		}

		// Fix apt-get patterns
		if strings.Contains(line, "apt-get install") && !strings.Contains(line, "apt-get update") {
			corrected[i] = strings.Replace(line, "apt-get install", "apt-get update && apt-get install", 1)
			fixes = append(fixes, fmt.Sprintf("Line %d: Added apt-get update before install", lineNum))
		}

		// Add apt cache cleanup
		if strings.Contains(line, "apt-get install") && !strings.Contains(line, "rm -rf /var/lib/apt/lists/*") {
			corrected[i] = line + " && rm -rf /var/lib/apt/lists/*"
			fixes = append(fixes, fmt.Sprintf("Line %d: Added apt cache cleanup", lineNum))
		}

		// Add non-root user at the end
		if i == len(lines)-1 && !containsUserInstruction(corrected) {
			corrected = append(corrected, "", "# Create non-root user", "RUN adduser -D appuser", "USER appuser")
			fixes = append(fixes, "Added non-root user for security")
		}
	}

	return strings.Join(corrected, "\n"), fixes
}

// GenerateRecommendations creates actionable recommendations based on validation results.
func (r *ExtendedValidationResult) GenerateRecommendations() []ExtendedRecommendation {
	recommendations := make([]ExtendedRecommendation, 0)

	// Security recommendations
	if len(r.SecurityIssues) > 0 {
		recommendations = append(recommendations, ExtendedRecommendation{
			Recommendation: Recommendation{
				Title:       "Address Security Issues",
				Description: "Fix identified security vulnerabilities in Dockerfile",
				Category:    "security",
				Priority:    string(types.SeverityHigh),
				Impact:      string(types.SeverityHigh),
				Effort:      "medium",
			},
			RecommendationID: fmt.Sprintf("security-fixes-%s", r.SessionID),
			Type:             "fix",
			Tags:             []string{"security", "dockerfile", "vulnerabilities"},
			ActionType:       "immediate",
			Benefits:         []string{"Improved security posture", "Reduced attack surface"},
			Risks:            []string{"Build process changes", "Compatibility issues"},
			Urgency:          "immediate",
			Confidence:       95,
		})
	}

	// Validation error recommendations
	if len(r.Errors) > 0 {
		recommendations = append(recommendations, ExtendedRecommendation{
			Recommendation: Recommendation{
				Title:       "Fix Validation Errors",
				Description: "Address validation errors in Dockerfile",
				Category:    "quality",
				Priority:    string(types.SeverityHigh),
				Impact:      string(types.SeverityHigh),
				Effort:      "low",
			},
			RecommendationID: fmt.Sprintf("validation-errors-%s", r.SessionID),
			Type:             "fix",
			Tags:             []string{"validation", "dockerfile", "github.com/Azure/container-kit/pkg/mcp/errors"},
			ActionType:       "immediate",
			Benefits:         []string{"Valid Dockerfile", "Successful builds"},
			Risks:            []string{"None"},
			Urgency:          "immediate",
			Confidence:       100,
		})
	}

	// Best practices recommendations
	if len(r.Warnings) > 5 {
		recommendations = append(recommendations, ExtendedRecommendation{
			Recommendation: Recommendation{
				Title:       "Follow Docker Best Practices",
				Description: "Implement Docker best practices for better maintainability",
				Category:    "quality",
				Priority:    string(types.SeverityMedium),
				Impact:      string(types.SeverityMedium),
				Effort:      "low",
			},
			RecommendationID: fmt.Sprintf("best-practices-%s", r.SessionID),
			Type:             "improvement",
			Tags:             []string{"best-practices", "dockerfile", "quality"},
			ActionType:       "soon",
			Benefits:         []string{"Better maintainability", "Improved performance", "Reduced image size"},
			Risks:            []string{"Build changes required"},
			Urgency:          "soon",
			Confidence:       85,
		})
	}

	// Optimization recommendations
	if len(r.OptimizationTips) > 0 {
		recommendations = append(recommendations, ExtendedRecommendation{
			Recommendation: Recommendation{
				Title:       "Apply Dockerfile Optimizations",
				Description: "Implement suggested optimizations for better performance",
				Category:    "performance",
				Priority:    string(types.SeverityLow),
				Impact:      string(types.SeverityMedium),
				Effort:      "medium",
			},
			RecommendationID: fmt.Sprintf("optimizations-%s", r.SessionID),
			Type:             "optimization",
			Tags:             []string{"optimization", "dockerfile", "performance"},
			ActionType:       "when_convenient",
			Benefits:         []string{"Smaller image size", "Faster builds", "Better caching"},
			Risks:            []string{"Minimal"},
			Urgency:          "low",
			Confidence:       80,
		})
	}

	return recommendations
}

// CreateRemediationPlan creates a structured remediation plan for addressing issues.
func (r *ExtendedValidationResult) CreateRemediationPlan() interface{} {
	return map[string]interface{}{
		"plan_id":     fmt.Sprintf("dockerfile-validation-%s", r.SessionID),
		"title":       "Dockerfile Validation Plan",
		"description": "Plan to address Dockerfile validation issues",
		"priority":    "medium",
	}
}

// GetAlternativeStrategies provides alternative approaches for common issues.
func (r *ExtendedValidationResult) GetAlternativeStrategies() interface{} {
	return []map[string]interface{}{
		{
			"strategy":    "Use validated base images",
			"description": "Switch to security-scanned base images",
		},
	}
}

// determineImpact categorizes the impact of different warning types.
func determineImpact(warningType string) string {
	switch warningType {
	case "security":
		return "security"
	case "best_practice":
		return "maintainability"
	default:
		return "performance"
	}
}

// isTrustedRegistry checks if a registry is in the list of trusted registries.
func isTrustedRegistry(registry string) bool {
	trustedRegistries := constants.KnownRegistries

	for _, trusted := range trustedRegistries {
		if registry == trusted {
			return true
		}
	}
	return false
}

// isOfficialImage determines if an image is an official Docker Hub image.
func isOfficialImage(image string) bool {
	parts := strings.Split(image, "/")
	return len(parts) == 1 || (len(parts) == 2 && parts[0] == "library")
}

// suggestAlternativeImages provides alternative base images for common patterns.
func suggestAlternativeImages(image string) []string {
	alternatives := make([]string, 0)

	baseImage := strings.Split(image, ":")[0]
	switch {
	case strings.Contains(baseImage, "ubuntu"):
		alternatives = append(alternatives, "debian:slim", "alpine:latest")
	case strings.Contains(baseImage, "debian"):
		alternatives = append(alternatives, "debian:slim", "alpine:latest")
	case strings.Contains(baseImage, "centos"):
		alternatives = append(alternatives, "rockylinux:minimal", "almalinux:minimal")
	case strings.Contains(baseImage, "node"):
		alternatives = append(alternatives, "node:alpine", "node:slim")
	}

	return alternatives
}

// containsUserInstruction checks if the Dockerfile contains a USER instruction.
func containsUserInstruction(lines []string) bool {
	for _, line := range lines {
		if strings.HasPrefix(strings.ToUpper(strings.TrimSpace(line)), "USER") {
			return true
		}
	}
	return false
}
