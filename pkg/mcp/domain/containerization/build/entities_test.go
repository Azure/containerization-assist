package build

import (
	"testing"
	"time"
)

func TestBuildRequest_Validate(t *testing.T) {
	validRequest := &BuildRequest{
		SessionID: "test-session",
		ImageName: "myapp",
		Context:   "/build/context",
		Tags:      []string{"latest", "v1.0.0"},
		Platform:  "linux/amd64",
		Options: BuildOptions{
			Timeout: time.Hour,
		},
	}

	errors := validRequest.Validate()
	if len(errors) != 0 {
		t.Errorf("expected no validation errors, got %d: %v", len(errors), errors)
	}

	// Test invalid request
	invalidRequest := &BuildRequest{
		SessionID: "",
		ImageName: "",
		Context:   "",
		Tags:      []string{"INVALID_TAG!"},
		Platform:  "invalid-platform",
		Options: BuildOptions{
			Timeout: -time.Hour,
		},
	}

	errors = invalidRequest.Validate()
	if len(errors) == 0 {
		t.Error("expected validation errors for invalid request")
	}

	// Check for specific error codes
	errorCodes := make(map[string]bool)
	for _, err := range errors {
		errorCodes[err.Code] = true
	}

	expectedCodes := []string{"MISSING_SESSION_ID", "MISSING_IMAGE_NAME", "MISSING_CONTEXT", "INVALID_TAG", "INVALID_PLATFORM", "INVALID_TIMEOUT"}
	for _, code := range expectedCodes {
		if !errorCodes[code] {
			t.Errorf("expected error code %s", code)
		}
	}
}

func TestBuildResult_IsCompleted(t *testing.T) {
	tests := []struct {
		status   BuildStatus
		expected bool
	}{
		{BuildStatusCompleted, true},
		{BuildStatusFailed, true},
		{BuildStatusCancelled, true},
		{BuildStatusTimeout, true},
		{BuildStatusPending, false},
		{BuildStatusQueued, false},
		{BuildStatusRunning, false},
	}

	for _, tt := range tests {
		t.Run(string(tt.status), func(t *testing.T) {
			result := &BuildResult{Status: tt.status}
			if result.IsCompleted() != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result.IsCompleted())
			}
		})
	}
}

func TestBuildResult_GetCriticalVulnerabilities(t *testing.T) {
	result := &BuildResult{
		Metadata: BuildMetadata{
			SecurityScan: &SecurityScanResult{
				Vulnerabilities: []Vulnerability{
					{ID: "CVE-1", Severity: SeverityCritical},
					{ID: "CVE-2", Severity: SeverityHigh},
					{ID: "CVE-3", Severity: SeverityCritical},
					{ID: "CVE-4", Severity: SeverityMedium},
				},
			},
		},
	}

	critical := result.GetCriticalVulnerabilities()
	if len(critical) != 2 {
		t.Errorf("expected 2 critical vulnerabilities, got %d", len(critical))
	}

	for _, vuln := range critical {
		if vuln.Severity != SeverityCritical {
			t.Error("expected only critical severity vulnerabilities")
		}
	}

	// Test with no security scan
	resultNoScan := &BuildResult{}
	critical = resultNoScan.GetCriticalVulnerabilities()
	if critical != nil {
		t.Error("expected nil for result with no security scan")
	}
}

func TestBuildResult_ShouldOptimize(t *testing.T) {
	// Large image should be optimized
	largeImage := &BuildResult{
		Size: 2 * 1024 * 1024 * 1024, // 2GB
	}
	if !largeImage.ShouldOptimize() {
		t.Error("expected large image to need optimization")
	}

	// Many layers should be optimized
	manyLayers := &BuildResult{
		Size: 100 * 1024 * 1024, // 100MB
		Metadata: BuildMetadata{
			Layers: 25,
		},
	}
	if !manyLayers.ShouldOptimize() {
		t.Error("expected image with many layers to need optimization")
	}

	// Low cache hit rate should be optimized
	lowCacheHit := &BuildResult{
		Size: 100 * 1024 * 1024, // 100MB
		Metadata: BuildMetadata{
			Layers:      10,
			CacheHits:   2,
			CacheMisses: 8,
		},
	}
	if !lowCacheHit.ShouldOptimize() {
		t.Error("expected image with low cache hit rate to need optimization")
	}

	// Small, efficient image should not need optimization
	efficientImage := &BuildResult{
		Size: 50 * 1024 * 1024, // 50MB
		Metadata: BuildMetadata{
			Layers:      8,
			CacheHits:   8,
			CacheMisses: 2,
		},
	}
	if efficientImage.ShouldOptimize() {
		t.Error("expected efficient image to not need optimization")
	}
}

func TestSelectOptimalStrategy(t *testing.T) {
	// Explicit strategy should be used
	explicitReq := &BuildRequest{
		Options: BuildOptions{
			Strategy: BuildStrategyPodman,
		},
	}
	if SelectOptimalStrategy(explicitReq) != BuildStrategyPodman {
		t.Error("expected explicit strategy to be used")
	}

	// Multi-platform should use BuildKit
	multiPlatformReq := &BuildRequest{
		Platform: "linux/amd64,linux/arm64",
	}
	if SelectOptimalStrategy(multiPlatformReq) != BuildStrategyBuildKit {
		t.Error("expected BuildKit for multi-platform builds")
	}

	// Advanced features should use BuildKit
	advancedReq := &BuildRequest{
		Options: BuildOptions{
			EnableBuildKit: true,
		},
	}
	if SelectOptimalStrategy(advancedReq) != BuildStrategyBuildKit {
		t.Error("expected BuildKit for advanced features")
	}

	// Simple build should use Docker
	simpleReq := &BuildRequest{}
	if SelectOptimalStrategy(simpleReq) != BuildStrategyDocker {
		t.Error("expected Docker for simple builds")
	}
}

func TestEstimateBuildTime(t *testing.T) {
	stats := &BuildStats{
		AverageDuration: 10 * time.Minute,
	}

	// Regular build
	regularReq := &BuildRequest{}
	duration := EstimateBuildTime(regularReq, stats)
	if duration != 10*time.Minute {
		t.Errorf("expected 10 minutes, got %v", duration)
	}

	// No-cache build should take longer
	noCacheReq := &BuildRequest{
		NoCache: true,
	}
	duration = EstimateBuildTime(noCacheReq, stats)
	if duration != 20*time.Minute {
		t.Errorf("expected 20 minutes for no-cache build, got %v", duration)
	}

	// Multi-stage build should take longer
	multiStageReq := &BuildRequest{
		Target: "production",
	}
	duration = EstimateBuildTime(multiStageReq, stats)
	expected := time.Duration(float64(10*time.Minute) * 1.5)
	if duration != expected {
		t.Errorf("expected %v for multi-stage build, got %v", expected, duration)
	}
}

func TestCanScheduleBuild(t *testing.T) {
	simpleReq := &BuildRequest{}
	buildkitReq := &BuildRequest{
		Options: BuildOptions{
			Strategy: BuildStrategyBuildKit,
		},
	}

	// Can schedule when under capacity
	if !CanScheduleBuild(simpleReq, 2, 5) {
		t.Error("expected to schedule build when under capacity")
	}

	// Cannot schedule when at capacity
	if CanScheduleBuild(simpleReq, 5, 5) {
		t.Error("expected not to schedule build when at capacity")
	}

	// BuildKit builds have higher priority
	if !CanScheduleBuild(buildkitReq, 4, 5) {
		t.Error("expected to schedule BuildKit build even near capacity")
	}
}

func TestGetBuildPriority(t *testing.T) {
	// Default priority
	defaultReq := &BuildRequest{}
	if GetBuildPriority(defaultReq) != 5 {
		t.Error("expected default priority of 5")
	}

	// BuildKit increases priority
	buildkitReq := &BuildRequest{
		Options: BuildOptions{
			Strategy: BuildStrategyBuildKit,
		},
	}
	if GetBuildPriority(buildkitReq) != 7 {
		t.Error("expected priority of 7 for BuildKit")
	}

	// Production tags increase priority
	prodReq := &BuildRequest{
		Tags: []string{"prod-v1.0.0"},
	}
	if GetBuildPriority(prodReq) != 8 {
		t.Error("expected priority of 8 for production build")
	}

	// Development tags decrease priority
	devReq := &BuildRequest{
		Tags: []string{"dev-branch"},
	}
	if GetBuildPriority(devReq) != 4 {
		t.Error("expected priority of 4 for development build")
	}
}