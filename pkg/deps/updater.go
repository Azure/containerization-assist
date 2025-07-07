package deps

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	mcperrors "github.com/Azure/container-kit/pkg/mcp/domain/errors"
	"github.com/rs/zerolog"
)

// UpdateMode defines different update strategies
type UpdateMode string

const (
	UpdateModeMinor    UpdateMode = "minor"    // Update minor versions only
	UpdateModePatch    UpdateMode = "patch"    // Update patch versions only
	UpdateModeAll      UpdateMode = "all"      // Update all including major
	UpdateModeSecurity UpdateMode = "security" // Security updates only
)

// DependencyInfo represents information about a dependency
type DependencyInfo struct {
	Name           string    `json:"name"`
	CurrentVersion string    `json:"current_version"`
	LatestVersion  string    `json:"latest_version"`
	UpdateType     string    `json:"update_type"` // major, minor, patch
	SecurityUpdate bool      `json:"security_update"`
	Description    string    `json:"description"`
	UpdatedAt      time.Time `json:"updated_at"`
	ChangelogURL   string    `json:"changelog_url"`
}

// UpdateResult represents the result of an update operation
type UpdateResult struct {
	UpdatedDeps  []DependencyInfo `json:"updated_deps"`
	FailedDeps   []DependencyInfo `json:"failed_deps"`
	SkippedDeps  []DependencyInfo `json:"skipped_deps"`
	TotalChecked int              `json:"total_checked"`
	Duration     time.Duration    `json:"duration"`
	Timestamp    time.Time        `json:"timestamp"`
}

// UpdaterConfig holds configuration for the dependency updater
type UpdaterConfig struct {
	ProjectRoot     string
	Mode            UpdateMode
	DryRun          bool
	AutoCommit      bool
	IncludeIndirect bool
	ExcludePatterns []string
	Logger          zerolog.Logger
}

// Updater handles dependency updates
type Updater struct {
	config UpdaterConfig
	logger zerolog.Logger
}

// NewUpdater creates a new dependency updater
func NewUpdater(config UpdaterConfig) *Updater {
	return &Updater{
		config: config,
		logger: config.Logger.With().Str("component", "deps_updater").Logger(),
	}
}

// CheckUpdates checks for available dependency updates
func (u *Updater) CheckUpdates(ctx context.Context) (*UpdateResult, error) {
	u.logger.Info().
		Str("mode", string(u.config.Mode)).
		Bool("dry_run", u.config.DryRun).
		Msg("Checking for dependency updates")

	startTime := time.Now()

	// Get current dependencies
	currentDeps, err := u.getCurrentDependencies(ctx)
	if err != nil {
		return nil, mcperrors.NewError().Messagef("failed to get current dependencies: %w", err).WithLocation().Build()
	}

	u.logger.Info().Int("count", len(currentDeps)).Msg("Found dependencies")

	// Check for updates
	result := &UpdateResult{
		UpdatedDeps:  make([]DependencyInfo, 0),
		FailedDeps:   make([]DependencyInfo, 0),
		SkippedDeps:  make([]DependencyInfo, 0),
		TotalChecked: len(currentDeps),
		Timestamp:    time.Now(),
	}

	for _, dep := range currentDeps {
		if u.shouldSkipDependency(dep.Name) {
			result.SkippedDeps = append(result.SkippedDeps, dep)
			continue
		}

		latest, err := u.getLatestVersion(ctx, dep.Name)
		if err != nil {
			u.logger.Warn().Err(err).Str("dep", dep.Name).Msg("Failed to get latest version")
			dep.Description = fmt.Sprintf("Failed to check: %v", err)
			result.FailedDeps = append(result.FailedDeps, dep)
			continue
		}

		dep.LatestVersion = latest
		dep.UpdateType = u.determineUpdateType(dep.CurrentVersion, latest)
		dep.SecurityUpdate = u.isSecurityUpdate(ctx, dep.Name, dep.CurrentVersion, latest)
		dep.UpdatedAt = time.Now()
		dep.ChangelogURL = u.generateChangelogURL(dep.Name)

		if u.shouldUpdate(dep) {
			result.UpdatedDeps = append(result.UpdatedDeps, dep)
		} else {
			result.SkippedDeps = append(result.SkippedDeps, dep)
		}
	}

	result.Duration = time.Since(startTime)

	u.logger.Info().
		Int("updated", len(result.UpdatedDeps)).
		Int("failed", len(result.FailedDeps)).
		Int("skipped", len(result.SkippedDeps)).
		Dur("duration", result.Duration).
		Msg("Update check completed")

	return result, nil
}

// ApplyUpdates applies the dependency updates
func (u *Updater) ApplyUpdates(ctx context.Context, updates []DependencyInfo) error {
	if u.config.DryRun {
		u.logger.Info().Msg("Dry run mode - no updates will be applied")
		return nil
	}

	u.logger.Info().Int("count", len(updates)).Msg("Applying dependency updates")

	for _, dep := range updates {
		if err := u.updateDependency(ctx, dep); err != nil {
			u.logger.Error().Err(err).Str("dep", dep.Name).Msg("Failed to update dependency")
			return mcperrors.NewError().Messagef("failed to update %s: %w", dep.Name, err).WithLocation().Build()
		}

		u.logger.Info().
			Str("dep", dep.Name).
			Str("from", dep.CurrentVersion).
			Str("to", dep.LatestVersion).
			Msg("Updated dependency")
	}

	// Run go mod tidy to clean up
	if err := u.runGoModTidy(ctx); err != nil {
		u.logger.Warn().Err(err).Msg("Failed to run go mod tidy")
	}

	// Auto commit if enabled
	if u.config.AutoCommit && len(updates) > 0 {
		if err := u.commitUpdates(ctx, updates); err != nil {
			u.logger.Warn().Err(err).Msg("Failed to commit updates")
		}
	}

	return nil
}

// getCurrentDependencies gets the current dependencies from go.mod
func (u *Updater) getCurrentDependencies(ctx context.Context) ([]DependencyInfo, error) {
	cmd := exec.CommandContext(ctx, "go", "list", "-m", "-json", "all")
	cmd.Dir = u.config.ProjectRoot

	output, err := cmd.Output()
	if err != nil {
		// Try to get stderr for more helpful error message
		if exitErr, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Errorf("failed to list modules: %w\nStderr: %s", err, string(exitErr.Stderr))
		}
		return nil, fmt.Errorf("failed to list modules: %w", err)
	}

	var deps []DependencyInfo
	decoder := json.NewDecoder(strings.NewReader(string(output)))

	for decoder.More() {
		var module struct {
			Path     string `json:"Path"`
			Version  string `json:"Version"`
			Main     bool   `json:"Main"`
			Indirect bool   `json:"Indirect"`
		}

		if err := decoder.Decode(&module); err != nil {
			continue
		}

		// Skip main module and indirect deps if not included
		if module.Main || (module.Indirect && !u.config.IncludeIndirect) {
			continue
		}

		deps = append(deps, DependencyInfo{
			Name:           module.Path,
			CurrentVersion: module.Version,
		})
	}

	return deps, nil
}

// getLatestVersion gets the latest version for a dependency
func (u *Updater) getLatestVersion(ctx context.Context, depName string) (string, error) {
	cmd := exec.CommandContext(ctx, "go", "list", "-m", "-versions", depName)
	cmd.Dir = u.config.ProjectRoot

	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get versions for %s: %w", depName, err)
	}

	// Parse versions line: "module.name v1.0.0 v1.1.0 v1.2.0"
	parts := strings.Fields(strings.TrimSpace(string(output)))
	if len(parts) < 2 {
		return "", fmt.Errorf("no versions found for %s", depName)
	}

	// Last version is the latest
	return parts[len(parts)-1], nil
}

// determineUpdateType determines if an update is major, minor, or patch
func (u *Updater) determineUpdateType(current, latest string) string {
	currentParts := parseVersion(current)
	latestParts := parseVersion(latest)

	if len(currentParts) < 3 || len(latestParts) < 3 {
		return "unknown"
	}

	if currentParts[0] != latestParts[0] {
		return "major"
	}
	if currentParts[1] != latestParts[1] {
		return "minor"
	}
	if currentParts[2] != latestParts[2] {
		return "patch"
	}

	return "none"
}

// parseVersion extracts version numbers from version string
func parseVersion(version string) []string {
	// Remove 'v' prefix and extract numbers
	version = strings.TrimPrefix(version, "v")
	re := regexp.MustCompile(`(\d+)\.(\d+)\.(\d+)`)
	matches := re.FindStringSubmatch(version)
	if len(matches) >= 4 {
		return matches[1:4]
	}
	return []string{}
}

// isSecurityUpdate checks if an update contains security fixes
func (u *Updater) isSecurityUpdate(ctx context.Context, depName, currentVersion, latestVersion string) bool {
	// This is a simplified check - in a real implementation, you might:
	// - Check against vulnerability databases
	// - Parse changelogs for security keywords
	// - Use specialized security scanning tools

	// For now, we'll use a heuristic approach
	securityKeywords := []string{"security", "vulnerability", "CVE", "exploit", "fix"}

	// Try to get module info
	cmd := exec.CommandContext(ctx, "go", "list", "-m", "-json", fmt.Sprintf("%s@%s", depName, latestVersion))
	cmd.Dir = u.config.ProjectRoot

	output, err := cmd.Output()
	if err != nil {
		return false
	}

	outputLower := strings.ToLower(string(output))
	for _, keyword := range securityKeywords {
		if strings.Contains(outputLower, keyword) {
			return true
		}
	}

	return false
}

// shouldUpdate determines if a dependency should be updated based on mode
func (u *Updater) shouldUpdate(dep DependencyInfo) bool {
	if dep.CurrentVersion == dep.LatestVersion {
		return false
	}

	switch u.config.Mode {
	case UpdateModeSecurity:
		return dep.SecurityUpdate
	case UpdateModePatch:
		return dep.UpdateType == "patch" || dep.SecurityUpdate
	case UpdateModeMinor:
		return dep.UpdateType == "minor" || dep.UpdateType == "patch" || dep.SecurityUpdate
	case UpdateModeAll:
		return true
	default:
		return false
	}
}

// shouldSkipDependency checks if a dependency should be skipped
func (u *Updater) shouldSkipDependency(depName string) bool {
	for _, pattern := range u.config.ExcludePatterns {
		if matched, _ := filepath.Match(pattern, depName); matched {
			return true
		}
		if strings.Contains(depName, pattern) {
			return true
		}
	}
	return false
}

// updateDependency updates a single dependency
func (u *Updater) updateDependency(ctx context.Context, dep DependencyInfo) error {
	cmd := exec.CommandContext(ctx, "go", "get", fmt.Sprintf("%s@%s", dep.Name, dep.LatestVersion))
	cmd.Dir = u.config.ProjectRoot

	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("go get failed: %w, output: %s", err, output)
	}

	return nil
}

// runGoModTidy runs go mod tidy to clean up dependencies
func (u *Updater) runGoModTidy(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, "go", "mod", "tidy")
	cmd.Dir = u.config.ProjectRoot

	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("go mod tidy failed: %w, output: %s", err, output)
	}

	return nil
}

// commitUpdates commits the dependency updates to git
func (u *Updater) commitUpdates(ctx context.Context, updates []DependencyInfo) error {
	if len(updates) == 0 {
		return nil
	}

	// Stage go.mod and go.sum
	cmd := exec.CommandContext(ctx, "git", "add", "go.mod", "go.sum")
	cmd.Dir = u.config.ProjectRoot
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to stage files: %w", err)
	}

	// Create commit message
	commitMsg := u.generateCommitMessage(updates)

	cmd = exec.CommandContext(ctx, "git", "commit", "-m", commitMsg)
	cmd.Dir = u.config.ProjectRoot
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to commit: %w", err)
	}

	u.logger.Info().Msg("Committed dependency updates")
	return nil
}

// generateCommitMessage generates a commit message for dependency updates
func (u *Updater) generateCommitMessage(updates []DependencyInfo) string {
	if len(updates) == 1 {
		dep := updates[0]
		return fmt.Sprintf("deps: update %s from %s to %s", dep.Name, dep.CurrentVersion, dep.LatestVersion)
	}

	var majorUpdates, minorUpdates, patchUpdates, securityUpdates int
	for _, dep := range updates {
		switch dep.UpdateType {
		case "major":
			majorUpdates++
		case "minor":
			minorUpdates++
		case "patch":
			patchUpdates++
		}
		if dep.SecurityUpdate {
			securityUpdates++
		}
	}

	var parts []string
	if securityUpdates > 0 {
		parts = append(parts, fmt.Sprintf("%d security", securityUpdates))
	}
	if majorUpdates > 0 {
		parts = append(parts, fmt.Sprintf("%d major", majorUpdates))
	}
	if minorUpdates > 0 {
		parts = append(parts, fmt.Sprintf("%d minor", minorUpdates))
	}
	if patchUpdates > 0 {
		parts = append(parts, fmt.Sprintf("%d patch", patchUpdates))
	}

	return fmt.Sprintf("deps: update %d dependencies (%s)", len(updates), strings.Join(parts, ", "))
}

// generateChangelogURL generates a changelog URL for a dependency
func (u *Updater) generateChangelogURL(depName string) string {
	// For GitHub dependencies, generate GitHub releases URL
	if strings.HasPrefix(depName, "github.com/") {
		return fmt.Sprintf("https://%s/releases", depName)
	}

	// For other dependencies, return pkg.go.dev
	return fmt.Sprintf("https://pkg.go.dev/%s", depName)
}

// GenerateUpdateReport generates a human-readable update report
func (r *UpdateResult) GenerateUpdateReport() string {
	var report strings.Builder

	report.WriteString("Dependency Update Report\n")
	report.WriteString("=======================\n\n")

	report.WriteString(fmt.Sprintf("Timestamp: %s\n", r.Timestamp.Format(time.RFC3339)))
	report.WriteString(fmt.Sprintf("Duration: %v\n", r.Duration))
	report.WriteString(fmt.Sprintf("Total Checked: %d\n\n", r.TotalChecked))

	if len(r.UpdatedDeps) > 0 {
		report.WriteString("Updated Dependencies:\n")
		report.WriteString("--------------------\n")
		for _, dep := range r.UpdatedDeps {
			securityFlag := ""
			if dep.SecurityUpdate {
				securityFlag = " (SECURITY)"
			}
			report.WriteString(fmt.Sprintf("• %s: %s → %s (%s)%s\n",
				dep.Name, dep.CurrentVersion, dep.LatestVersion, dep.UpdateType, securityFlag))
		}
		report.WriteString("\n")
	}

	if len(r.FailedDeps) > 0 {
		report.WriteString("Failed Updates:\n")
		report.WriteString("---------------\n")
		for _, dep := range r.FailedDeps {
			report.WriteString(fmt.Sprintf("• %s: %s\n", dep.Name, dep.Description))
		}
		report.WriteString("\n")
	}

	if len(r.SkippedDeps) > 0 {
		report.WriteString(fmt.Sprintf("Skipped Dependencies: %d\n", len(r.SkippedDeps)))
		report.WriteString("(Use --include-indirect or --mode=all to include more dependencies)\n\n")
	}

	return report.String()
}

// SaveUpdateReport saves the update report to a file
func (r *UpdateResult) SaveUpdateReport(filePath string) error {
	report := r.GenerateUpdateReport()
	return os.WriteFile(filePath, []byte(report), 0644)
}

// GetSecurityUpdates returns only security updates
func (r *UpdateResult) GetSecurityUpdates() []DependencyInfo {
	var securityUpdates []DependencyInfo
	for _, dep := range r.UpdatedDeps {
		if dep.SecurityUpdate {
			securityUpdates = append(securityUpdates, dep)
		}
	}
	return securityUpdates
}

// SortByUpdateType sorts dependencies by update type priority
func (r *UpdateResult) SortByUpdateType() {
	sort.Slice(r.UpdatedDeps, func(i, j int) bool {
		priority := map[string]int{
			"major": 0,
			"minor": 1,
			"patch": 2,
		}

		// Security updates get highest priority
		if r.UpdatedDeps[i].SecurityUpdate && !r.UpdatedDeps[j].SecurityUpdate {
			return true
		}
		if !r.UpdatedDeps[i].SecurityUpdate && r.UpdatedDeps[j].SecurityUpdate {
			return false
		}

		return priority[r.UpdatedDeps[i].UpdateType] < priority[r.UpdatedDeps[j].UpdateType]
	})
}
