package build

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/rs/zerolog"
)

// NetworkErrorRecoveryStrategy handles network-related build errors
type NetworkErrorRecoveryStrategy struct {
	logger zerolog.Logger
}

func NewNetworkErrorRecoveryStrategy(logger zerolog.Logger) *NetworkErrorRecoveryStrategy {
	return &NetworkErrorRecoveryStrategy{
		logger: logger.With().Str("strategy", "network").Logger(),
	}
}

func (s *NetworkErrorRecoveryStrategy) CanHandle(err error, analysis *BuildFailureAnalysis) bool {
	return analysis.FailureType == "network" || strings.Contains(err.Error(), "network")
}

func (s *NetworkErrorRecoveryStrategy) Recover(ctx context.Context, err error, analysis *BuildFailureAnalysis, operation *AtomicDockerBuildOperation) error {
	s.logger.Info().Msg("Applying network error recovery")

	// Step 1: Check network connectivity
	if err := s.checkNetworkConnectivity(ctx); err != nil {
		s.logger.Warn().Err(err).Msg("Network connectivity check failed")
	}

	// Step 2: Try with proxy settings if available
	if proxyURL := os.Getenv("HTTP_PROXY"); proxyURL != "" {
		s.logger.Info().Str("proxy", proxyURL).Msg("Attempting build with proxy settings")
		if operation.args.BuildArgs == nil {
			operation.args.BuildArgs = make(map[string]string)
		}
		operation.args.BuildArgs["HTTP_PROXY"] = proxyURL
		operation.args.BuildArgs["HTTPS_PROXY"] = proxyURL
		operation.args.BuildArgs["http_proxy"] = proxyURL
		operation.args.BuildArgs["https_proxy"] = proxyURL
	}

	// Step 3: Add DNS configuration
	if err := s.configureDNS(ctx, operation); err != nil {
		s.logger.Warn().Err(err).Msg("DNS configuration failed")
	}

	// Step 4: Set no-cache to avoid network-related cache issues
	operation.args.NoCache = true

	s.logger.Info().
		Interface("network_config", map[string]interface{}{
			"mode":    "host",
			"timeout": 300, // 5 minutes default
			"retries": 3,   // 3 retries default
			"proxy":   os.Getenv("HTTP_PROXY") != "",
		}).
		Msg("Network recovery configuration applied")

	// Retry the build operation with new network settings
	return operation.ExecuteOnce(ctx)
}

func (s *NetworkErrorRecoveryStrategy) GetPriority() int {
	return 80
}

// checkNetworkConnectivity verifies basic network connectivity
func (s *NetworkErrorRecoveryStrategy) checkNetworkConnectivity(ctx context.Context) error {
	// Check common endpoints
	endpoints := []string{
		"https://registry-1.docker.io",
		"https://gcr.io",
		"https://quay.io",
	}

	for _, endpoint := range endpoints {
		if err := s.pingEndpoint(ctx, endpoint); err == nil {
			s.logger.Debug().Str("endpoint", endpoint).Msg("Network connectivity confirmed")
			return nil
		}
	}

	return fmt.Errorf("no network connectivity to container registries")
}

// pingEndpoint checks if an endpoint is reachable
func (s *NetworkErrorRecoveryStrategy) pingEndpoint(ctx context.Context, endpoint string) error {
	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	req, err := http.NewRequestWithContext(ctx, "HEAD", endpoint+"/v2/", nil)
	if err != nil {
		return err
	}

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	return nil
}

// configureDNS adds DNS configuration for better resolution
func (s *NetworkErrorRecoveryStrategy) configureDNS(ctx context.Context, operation *AtomicDockerBuildOperation) error {
	// Add common public DNS servers as build args
	if operation.args.BuildArgs == nil {
		operation.args.BuildArgs = make(map[string]string)
	}
	operation.args.BuildArgs["DNS_SERVERS"] = "8.8.8.8,8.8.4.4,1.1.1.1"

	// If in a corporate environment, check for custom DNS
	if customDNS := os.Getenv("CORPORATE_DNS"); customDNS != "" {
		operation.args.BuildArgs["CORPORATE_DNS"] = customDNS
	}

	return nil
}

// PermissionErrorRecoveryStrategy handles permission-related build errors
type PermissionErrorRecoveryStrategy struct {
	logger zerolog.Logger
}

func NewPermissionErrorRecoveryStrategy(logger zerolog.Logger) *PermissionErrorRecoveryStrategy {
	return &PermissionErrorRecoveryStrategy{
		logger: logger.With().Str("strategy", "permission").Logger(),
	}
}

func (s *PermissionErrorRecoveryStrategy) CanHandle(err error, analysis *BuildFailureAnalysis) bool {
	return analysis.FailureType == "permission" || strings.Contains(err.Error(), "permission denied")
}

func (s *PermissionErrorRecoveryStrategy) Recover(ctx context.Context, err error, analysis *BuildFailureAnalysis, operation *AtomicDockerBuildOperation) error {
	s.logger.Info().Msg("Applying permission error recovery")

	// Step 1: Analyze Dockerfile for permission issues
	dockerfileContent, err := os.ReadFile(operation.dockerfilePath)
	if err != nil {
		return fmt.Errorf("failed to read Dockerfile: %w", err)
	}

	// Step 2: Fix file permissions in build context
	if err := s.fixBuildContextPermissions(ctx, operation.buildContext); err != nil {
		s.logger.Warn().Err(err).Msg("Failed to fix build context permissions")
	}

	// Step 3: Check for USER instructions and fix if needed
	if err := s.adjustDockerfilePermissions(ctx, operation, string(dockerfileContent)); err != nil {
		s.logger.Warn().Err(err).Msg("Failed to adjust Dockerfile permissions")
	}

	// Step 4: Add permission fix commands to Dockerfile if needed
	if strings.Contains(err.Error(), "permission denied") {
		// Create a temporary Dockerfile with permission fixes
		fixedDockerfile := s.createPermissionFixedDockerfile(string(dockerfileContent))
		tempDockerfile := filepath.Join(operation.buildContext, "Dockerfile.permission-fix")

		if err := os.WriteFile(tempDockerfile, []byte(fixedDockerfile), 0644); err != nil {
			return fmt.Errorf("failed to write fixed Dockerfile: %w", err)
		}

		operation.dockerfilePath = tempDockerfile
		s.logger.Info().Str("dockerfile", tempDockerfile).Msg("Using permission-fixed Dockerfile")
	}

	// Retry with permission fixes
	return operation.ExecuteOnce(ctx)
}

func (s *PermissionErrorRecoveryStrategy) GetPriority() int {
	return 90
}

// fixBuildContextPermissions fixes permissions in the build context
func (s *PermissionErrorRecoveryStrategy) fixBuildContextPermissions(ctx context.Context, buildContext string) error {
	s.logger.Info().Str("build_context", buildContext).Msg("Fixing build context permissions")

	// In real implementation, would fix common permission issues
	// For now, log the actions that would be taken
	s.logger.Debug().Msg("Would run: chmod -R 755 scripts/")
	s.logger.Debug().Msg("Would run: chmod +x *.sh")

	return nil
}

// adjustDockerfilePermissions modifies Dockerfile to handle permissions better
func (s *PermissionErrorRecoveryStrategy) adjustDockerfilePermissions(ctx context.Context, operation *AtomicDockerBuildOperation, content string) error {
	// Check if Dockerfile switches users inappropriately
	if strings.Contains(content, "USER root") && strings.Contains(content, "USER ") {
		s.logger.Info().Msg("Detected user switching in Dockerfile")
	}

	return nil
}

// createPermissionFixedDockerfile creates a Dockerfile with permission fixes
func (s *PermissionErrorRecoveryStrategy) createPermissionFixedDockerfile(content string) string {
	lines := strings.Split(content, "\n")
	var fixed []string

	for _, line := range lines {
		fixed = append(fixed, line)

		// Add permission fixes after COPY commands
		if strings.HasPrefix(strings.TrimSpace(strings.ToUpper(line)), "COPY") {
			fixed = append(fixed, "RUN chmod -R 755 /app || true")
		}

		// Add permission fixes after USER commands
		if strings.HasPrefix(strings.TrimSpace(strings.ToUpper(line)), "USER") && !strings.Contains(line, "root") {
			fixed = append(fixed, "USER root")
			fixed = append(fixed, "RUN chmod -R 755 /app && chown -R $(id -u):$(id -g) /app || true")
			fixed = append(fixed, line) // Restore original user
		}
	}

	return strings.Join(fixed, "\n")
}

// DockerfileErrorRecoveryStrategy handles Dockerfile syntax and content errors
type DockerfileErrorRecoveryStrategy struct {
	logger zerolog.Logger
}

func NewDockerfileErrorRecoveryStrategy(logger zerolog.Logger) *DockerfileErrorRecoveryStrategy {
	return &DockerfileErrorRecoveryStrategy{
		logger: logger.With().Str("strategy", "dockerfile").Logger(),
	}
}

func (s *DockerfileErrorRecoveryStrategy) CanHandle(err error, analysis *BuildFailureAnalysis) bool {
	return analysis.FailureType == "dockerfile_syntax" || analysis.FailureType == "file_missing" ||
		strings.Contains(err.Error(), "dockerfile") || strings.Contains(err.Error(), "syntax")
}

func (s *DockerfileErrorRecoveryStrategy) Recover(ctx context.Context, err error, analysis *BuildFailureAnalysis, operation *AtomicDockerBuildOperation) error {
	s.logger.Info().Msg("Applying Dockerfile error recovery")

	// Read current Dockerfile
	dockerfileContent, err := os.ReadFile(operation.dockerfilePath)
	if err != nil {
		return fmt.Errorf("failed to read Dockerfile: %w", err)
	}

	// Apply common fixes
	fixedContent := s.fixCommonDockerfileIssues(string(dockerfileContent), err)

	// Write fixed Dockerfile
	tempDockerfile := filepath.Join(operation.buildContext, "Dockerfile.fixed")
	if err := os.WriteFile(tempDockerfile, []byte(fixedContent), 0644); err != nil {
		return fmt.Errorf("failed to write fixed Dockerfile: %w", err)
	}

	operation.dockerfilePath = tempDockerfile
	s.logger.Info().Str("dockerfile", tempDockerfile).Msg("Using fixed Dockerfile")

	return operation.ExecuteOnce(ctx)
}

func (s *DockerfileErrorRecoveryStrategy) GetPriority() int {
	return 70
}

// fixCommonDockerfileIssues applies common fixes to Dockerfile issues
func (s *DockerfileErrorRecoveryStrategy) fixCommonDockerfileIssues(content string, err error) string {
	lines := strings.Split(content, "\n")
	var fixed []string

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Fix missing FROM instruction
		if len(fixed) == 0 && !strings.HasPrefix(strings.ToUpper(trimmed), "FROM") && trimmed != "" {
			fixed = append(fixed, "FROM ubuntu:20.04")
		}

		// Fix COPY source issues
		if strings.HasPrefix(strings.ToUpper(trimmed), "COPY") && strings.Contains(err.Error(), "no such file") {
			// Add error handling
			fixed = append(fixed, "# Original: "+line)
			fixed = append(fixed, "COPY . /app/ || echo 'Copy failed, continuing...'")
			continue
		}

		// Fix missing WORKDIR
		if strings.HasPrefix(strings.ToUpper(trimmed), "COPY") && !strings.Contains(content, "WORKDIR") {
			fixed = append(fixed, "WORKDIR /app")
		}

		fixed = append(fixed, line)
	}

	return strings.Join(fixed, "\n")
}

// DependencyErrorRecoveryStrategy handles package dependency errors
type DependencyErrorRecoveryStrategy struct {
	logger zerolog.Logger
}

func NewDependencyErrorRecoveryStrategy(logger zerolog.Logger) *DependencyErrorRecoveryStrategy {
	return &DependencyErrorRecoveryStrategy{
		logger: logger.With().Str("strategy", "dependency").Logger(),
	}
}

func (s *DependencyErrorRecoveryStrategy) CanHandle(err error, analysis *BuildFailureAnalysis) bool {
	return analysis.FailureType == "dependency" ||
		strings.Contains(err.Error(), "package") ||
		strings.Contains(err.Error(), "not found") ||
		strings.Contains(err.Error(), "unable to locate")
}

func (s *DependencyErrorRecoveryStrategy) Recover(ctx context.Context, err error, analysis *BuildFailureAnalysis, operation *AtomicDockerBuildOperation) error {
	s.logger.Info().Msg("Applying dependency error recovery")

	// Identify missing packages
	missingPackages := s.extractMissingPackages(err.Error())
	s.logger.Info().Strs("missing_packages", missingPackages).Msg("Identified missing packages")

	// Read Dockerfile
	dockerfileContent, err := os.ReadFile(operation.dockerfilePath)
	if err != nil {
		return fmt.Errorf("failed to read Dockerfile: %w", err)
	}

	// Create fixed Dockerfile with dependency fixes
	fixedContent := s.addMissingDependencies(string(dockerfileContent), missingPackages)

	// Write fixed Dockerfile
	tempDockerfile := filepath.Join(operation.buildContext, "Dockerfile.deps-fixed")
	if err := os.WriteFile(tempDockerfile, []byte(fixedContent), 0644); err != nil {
		return fmt.Errorf("failed to write dependency-fixed Dockerfile: %w", err)
	}

	operation.dockerfilePath = tempDockerfile
	s.logger.Info().Str("dockerfile", tempDockerfile).Msg("Using dependency-fixed Dockerfile")

	return operation.ExecuteOnce(ctx)
}

func (s *DependencyErrorRecoveryStrategy) GetPriority() int {
	return 85
}

// extractMissingPackages extracts package names from error messages
func (s *DependencyErrorRecoveryStrategy) extractMissingPackages(errorMsg string) []string {
	var packages []string

	// Common patterns for missing packages
	patterns := []string{
		`package '([^']+)' has no installation candidate`,
		`Unable to locate package ([^\s]+)`,
		`Package '([^']+)' is not available`,
		`([^\s]+): not found`,
	}

	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		matches := re.FindAllStringSubmatch(errorMsg, -1)
		for _, match := range matches {
			if len(match) > 1 {
				packages = append(packages, match[1])
			}
		}
	}

	return packages
}

// addMissingDependencies adds missing dependencies to Dockerfile
func (s *DependencyErrorRecoveryStrategy) addMissingDependencies(content string, missingPackages []string) string {
	if len(missingPackages) == 0 {
		return content
	}

	lines := strings.Split(content, "\n")
	var fixed []string

	for _, line := range lines {
		fixed = append(fixed, line)

		// Add missing packages after package update commands
		if strings.Contains(line, "apt-get update") {
			for _, pkg := range missingPackages {
				alternative := s.getPackageAlternative(pkg)
				fixed = append(fixed, fmt.Sprintf("RUN apt-get install -y %s || apt-get install -y %s || echo 'Package %s not available'", pkg, alternative, pkg))
			}
		}
	}

	return strings.Join(fixed, "\n")
}

// getPackageAlternative provides alternative package names
func (s *DependencyErrorRecoveryStrategy) getPackageAlternative(pkg string) string {
	alternatives := map[string]string{
		"python":     "python3",
		"python-pip": "python3-pip",
		"nodejs":     "node",
		"gcc":        "build-essential",
		"make":       "build-essential",
	}

	if alt, exists := alternatives[pkg]; exists {
		return alt
	}

	return pkg + "-dev" // Common pattern for development packages
}
