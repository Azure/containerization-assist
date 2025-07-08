// Package build contains business rules for container build operations
package build

import (
	"fmt"
	"regexp"
	"strings"
	"time"
)

// ValidationError represents a build validation error
type ValidationError struct {
	Field   string
	Message string
	Code    string
}

func (e ValidationError) Error() string {
	return fmt.Sprintf("build validation error: %s - %s", e.Field, e.Message)
}

// Validate performs domain-level validation on a build request
func (br *BuildRequest) Validate() []ValidationError {
	var errors []ValidationError

	// Session ID is required
	if br.SessionID == "" {
		errors = append(errors, ValidationError{
			Field:   "session_id",
			Message: "session ID is required",
			Code:    "MISSING_SESSION_ID",
		})
	}

	// Image name is required and must be valid
	if br.ImageName == "" {
		errors = append(errors, ValidationError{
			Field:   "image_name",
			Message: "image name is required",
			Code:    "MISSING_IMAGE_NAME",
		})
	} else if !isValidImageName(br.ImageName) {
		errors = append(errors, ValidationError{
			Field:   "image_name",
			Message: "image name format is invalid",
			Code:    "INVALID_IMAGE_NAME",
		})
	}

	// Context path is required
	if br.Context == "" {
		errors = append(errors, ValidationError{
			Field:   "context",
			Message: "build context is required",
			Code:    "MISSING_CONTEXT",
		})
	}

	// Dockerfile path validation
	if br.Dockerfile == "" {
		br.Dockerfile = "Dockerfile" // Default value
	}

	// Validate tags
	for i, tag := range br.Tags {
		if !isValidTag(tag) {
			errors = append(errors, ValidationError{
				Field:   fmt.Sprintf("tags[%d]", i),
				Message: fmt.Sprintf("invalid tag format: %s", tag),
				Code:    "INVALID_TAG",
			})
		}
	}

	// Validate platform format
	if br.Platform != "" && !isValidPlatform(br.Platform) {
		errors = append(errors, ValidationError{
			Field:   "platform",
			Message: "invalid platform format, expected format: os/architecture",
			Code:    "INVALID_PLATFORM",
		})
	}

	// Validate build options
	if br.Options.Timeout < 0 {
		errors = append(errors, ValidationError{
			Field:   "options.timeout",
			Message: "timeout cannot be negative",
			Code:    "INVALID_TIMEOUT",
		})
	}

	return errors
}

// Business Rules for Build Operations

// IsCompleted returns true if the build has completed (successfully or with failure)
func (br *BuildResult) IsCompleted() bool {
	return br.Status == BuildStatusCompleted || 
		   br.Status == BuildStatusFailed || 
		   br.Status == BuildStatusCancelled ||
		   br.Status == BuildStatusTimeout
}

// IsSuccessful returns true if the build completed successfully
func (br *BuildResult) IsSuccessful() bool {
	return br.Status == BuildStatusCompleted
}

// CanBeCancelled returns true if the build can be cancelled
func (br *BuildResult) CanBeCancelled() bool {
	return br.Status == BuildStatusPending || 
		   br.Status == BuildStatusQueued || 
		   br.Status == BuildStatusRunning
}

// GetCriticalVulnerabilities returns vulnerabilities with critical severity
func (br *BuildResult) GetCriticalVulnerabilities() []Vulnerability {
	if br.Metadata.SecurityScan == nil {
		return nil
	}

	var critical []Vulnerability
	for _, vuln := range br.Metadata.SecurityScan.Vulnerabilities {
		if vuln.Severity == SeverityCritical {
			critical = append(critical, vuln)
		}
	}
	return critical
}

// HasSecurityIssues returns true if the build has security vulnerabilities above a threshold
func (br *BuildResult) HasSecurityIssues(maxSeverity SeverityLevel) bool {
	if br.Metadata.SecurityScan == nil {
		return false
	}

	severityOrder := map[SeverityLevel]int{
		SeverityCritical: 5,
		SeverityHigh:     4,
		SeverityMedium:   3,
		SeverityLow:      2,
		SeverityInfo:     1,
		SeverityUnknown:  0,
	}

	threshold := severityOrder[maxSeverity]
	for _, vuln := range br.Metadata.SecurityScan.Vulnerabilities {
		if severityOrder[vuln.Severity] >= threshold {
			return true
		}
	}
	return false
}

// ShouldOptimize determines if the build should be optimized based on size and layers
func (br *BuildResult) ShouldOptimize() bool {
	// Large images should be optimized
	if br.Size > 1024*1024*1024 { // > 1GB
		return true
	}

	// Too many layers indicate optimization opportunity
	if br.Metadata.Layers > 20 {
		return true
	}

	// Low cache hit rate suggests optimization potential
	if br.Metadata.CacheHits+br.Metadata.CacheMisses > 0 {
		cacheHitRate := float64(br.Metadata.CacheHits) / float64(br.Metadata.CacheHits+br.Metadata.CacheMisses)
		if cacheHitRate < 0.5 {
			return true
		}
	}

	return false
}

// GetOptimizationRecommendations returns recommendations for optimizing the build
func (br *BuildResult) GetOptimizationRecommendations() []OptimizationRecommendation {
	var recommendations []OptimizationRecommendation

	// Size-based recommendations
	if br.Size > 1024*1024*1024 { // > 1GB
		recommendations = append(recommendations, OptimizationRecommendation{
			Type:        OptimizationTypeMultiStage,
			Priority:    PriorityHigh,
			Description: "Use multi-stage builds to reduce final image size",
			PotentialSavings: "Up to 80% size reduction",
		})
	}

	// Layer-based recommendations
	if br.Metadata.Layers > 20 {
		recommendations = append(recommendations, OptimizationRecommendation{
			Type:        OptimizationTypeLayerMerging,
			Priority:    PriorityMedium,
			Description: "Combine RUN commands to reduce layer count",
			PotentialSavings: fmt.Sprintf("Reduce from %d to ~10 layers", br.Metadata.Layers),
		})
	}

	// Cache-based recommendations
	if br.Metadata.CacheHits+br.Metadata.CacheMisses > 0 {
		cacheHitRate := float64(br.Metadata.CacheHits) / float64(br.Metadata.CacheHits+br.Metadata.CacheMisses)
		if cacheHitRate < 0.5 {
			recommendations = append(recommendations, OptimizationRecommendation{
				Type:        OptimizationTypeCache,
				Priority:    PriorityMedium,
				Description: "Reorder Dockerfile commands to improve cache utilization",
				PotentialSavings: "Faster builds through better caching",
			})
		}
	}

	// Security-based recommendations
	if br.HasSecurityIssues(SeverityHigh) {
		recommendations = append(recommendations, OptimizationRecommendation{
			Type:        OptimizationTypeBaseImage,
			Priority:    PriorityHigh,
			Description: "Update to a more secure base image",
			PotentialSavings: "Eliminate high/critical security vulnerabilities",
		})
	}

	return recommendations
}

// OptimizationRecommendation represents a recommendation for build optimization
type OptimizationRecommendation struct {
	Type             OptimizationType `json:"type"`
	Priority         PriorityLevel    `json:"priority"`
	Description      string           `json:"description"`
	PotentialSavings string           `json:"potential_savings"`
}

// PriorityLevel represents the priority of a recommendation
type PriorityLevel string

const (
	PriorityHigh   PriorityLevel = "high"
	PriorityMedium PriorityLevel = "medium"
	PriorityLow    PriorityLevel = "low"
)

// Business Rules for Build Strategy Selection

// SelectOptimalStrategy determines the best build strategy based on requirements
func SelectOptimalStrategy(req *BuildRequest) BuildStrategy {
	// If explicitly specified, use that strategy
	if req.Options.Strategy != "" {
		return req.Options.Strategy
	}

	// For multi-platform builds, prefer BuildKit
	if req.Platform != "" && strings.Contains(req.Platform, ",") {
		return BuildStrategyBuildKit
	}

	// For builds with advanced features, prefer BuildKit
	if req.Options.EnableBuildKit || len(req.Options.SecurityOpt) > 0 {
		return BuildStrategyBuildKit
	}

	// For simple builds, Docker is sufficient
	return BuildStrategyDocker
}

// EstimateBuildTime estimates build duration based on context and previous builds
func EstimateBuildTime(req *BuildRequest, stats *BuildStats) time.Duration {
	baseTime := 5 * time.Minute // Default base time

	// Adjust based on historical data
	if stats != nil && stats.AverageDuration > 0 {
		baseTime = stats.AverageDuration
	}

	// Adjust for no-cache builds
	if req.NoCache {
		baseTime *= 2
	}

	// Adjust for multi-stage builds (detected by target)
	if req.Target != "" {
		baseTime = time.Duration(float64(baseTime) * 1.5)
	}

	// Adjust for complex build args
	if len(req.BuildArgs) > 5 {
		baseTime = time.Duration(float64(baseTime) * 1.2)
	}

	return baseTime
}

// Validation helper functions

// isValidImageName validates Docker image name format
func isValidImageName(name string) bool {
	// Basic Docker image name validation
	// Pattern: [registry/]namespace/repository[:tag]
	pattern := `^([a-zA-Z0-9._-]+\.)?[a-zA-Z0-9._-]+(/[a-zA-Z0-9._-]+)*$`
	matched, err := regexp.MatchString(pattern, name)
	if err != nil {
		return false
	}
	return matched && len(name) <= 255
}

// isValidTag validates Docker tag format
func isValidTag(tag string) bool {
	// Docker tag validation: alphanumeric, dashes, underscores, periods, max 128 chars
	if len(tag) == 0 || len(tag) > 128 {
		return false
	}
	pattern := `^[a-zA-Z0-9._-]+$`
	matched, err := regexp.MatchString(pattern, tag)
	return err == nil && matched
}

// isValidPlatform validates platform format (os/arch)
func isValidPlatform(platform string) bool {
	parts := strings.Split(platform, "/")
	if len(parts) != 2 {
		return false
	}
	
	validOS := map[string]bool{
		"linux": true, "windows": true, "darwin": true,
	}
	validArch := map[string]bool{
		"amd64": true, "arm64": true, "arm": true, "386": true,
	}
	
	return validOS[parts[0]] && validArch[parts[1]]
}

// Business Rules for Resource Management

// CalculateResourceRequirements estimates resource needs for a build
func CalculateResourceRequirements(req *BuildRequest) ResourceRequirements {
	requirements := ResourceRequirements{
		CPU:    "1",      // 1 CPU core default
		Memory: "2Gi",    // 2GB RAM default
		Disk:   "10Gi",   // 10GB disk default
	}

	// Increase requirements for complex builds
	if req.NoCache {
		requirements.CPU = "2"
		requirements.Memory = "4Gi"
		requirements.Disk = "20Gi"
	}

	// Increase for multi-stage builds
	if req.Target != "" {
		requirements.Memory = "4Gi"
		requirements.Disk = "15Gi"
	}

	return requirements
}

// ResourceRequirements represents the resource requirements for a build
type ResourceRequirements struct {
	CPU    string `json:"cpu"`
	Memory string `json:"memory"`
	Disk   string `json:"disk"`
}

// Business Rules for Build Scheduling

// CanScheduleBuild determines if a build can be scheduled based on system capacity
func CanScheduleBuild(req *BuildRequest, currentBuilds int, maxConcurrentBuilds int) bool {
	if currentBuilds >= maxConcurrentBuilds {
		return false
	}

	// High priority builds can always be scheduled if there's any capacity
	if req.Options.Strategy == BuildStrategyBuildKit {
		return currentBuilds < maxConcurrentBuilds
	}

	// Regular builds need more conservative scheduling
	return currentBuilds < (maxConcurrentBuilds - 1)
}

// GetBuildPriority calculates build priority based on various factors
func GetBuildPriority(req *BuildRequest) int {
	priority := 5 // Default priority

	// Increase priority for BuildKit builds (more advanced)
	if req.Options.Strategy == BuildStrategyBuildKit {
		priority += 2
	}

	// Increase priority for production builds (inferred from tags)
	for _, tag := range req.Tags {
		if strings.Contains(tag, "prod") || strings.Contains(tag, "release") {
			priority += 3
			break
		}
	}

	// Decrease priority for development builds
	for _, tag := range req.Tags {
		if strings.Contains(tag, "dev") || strings.Contains(tag, "test") {
			priority -= 1
			break
		}
	}

	return priority
}