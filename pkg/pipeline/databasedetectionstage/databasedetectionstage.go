package databasedetectionstage

import (
	"context"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/Azure/container-copilot/pkg/logger"
	"github.com/Azure/container-copilot/pkg/pipeline"
	"github.com/Azure/container-copilot/pkg/utils"
)

// Ensure DatabaseDetectionStage implements pipeline.PipelineStage interface.
var _ pipeline.PipelineStage = &DatabaseDetectionStage{}

// DatabaseDetectionStage implements the pipeline.PipelineStage interface for detecting database usage.
type DatabaseDetectionStage struct {
	errors []error
}

// Initialize prepares initial state for database detection.
func (d *DatabaseDetectionStage) Initialize(ctx context.Context, state *pipeline.PipelineState, path string) error {
	// You can set up any initial metadata in state.Metadata if needed.
	return nil
}

// Generate performs the actual analysis to detect the database type and version.
func (d *DatabaseDetectionStage) Generate(ctx context.Context, state *pipeline.PipelineState, targetDir string) error {
	// Nothing to generate for repo analysis
	return nil
}

// GetErrors returns all errors encountered during database detection.
func (d *DatabaseDetectionStage) GetErrors(state *pipeline.PipelineState) string {
	var errStrings []string
	for _, err := range d.errors {
		errStrings = append(errStrings, err.Error())
	}
	return strings.Join(errStrings, "\n")
}

// WriteSuccessfulFiles writes any successful output to disk if applicable.
func (d *DatabaseDetectionStage) WriteSuccessfulFiles(state *pipeline.PipelineState) error {
	// No files to write; return nil.
	return nil
}

// Run ties together the stage's initialization and generation.
func (d *DatabaseDetectionStage) Run(ctx context.Context, state *pipeline.PipelineState, clientsObj interface{}, options pipeline.RunnerOptions) error {
	targetDir := options.TargetDirectory

	// Call the helper to detect the database.
	detectedDatabases, err := d.detectDatabases(targetDir)
	if err != nil {
		d.errors = append(d.errors, err)
		return err
	}

	// Save the detection results into the pipeline state
	state.Metadata["detectedDatabases"] = detectedDatabases
	logger.Infof("Final detected databases: %v", detectedDatabases)

	return nil
}

// Deploy performs any final deployment steps required by this stage.
func (d *DatabaseDetectionStage) Deploy(ctx context.Context, state *pipeline.PipelineState, clientsObj interface{}) error {
	return nil
}

// detectDatabases inspects repository files to detect database types and versions.
func (d *DatabaseDetectionStage) detectDatabases(targetDir string) ([]DatabaseDetectionResult, error) {
	logger.Infof("Detecting databases in repository: %s", targetDir)

	// Initialize progress tracker
	progressTracker := utils.NewProgressTracker()

	totalFiles, _ := calculateTotalFiles(targetDir)
	var processedFiles int
	detectedDatabases := make(map[DatabaseType]*DatabaseDetectionResult) // Use a map to avoid duplicates

	err := filepath.Walk(targetDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Only scan files (skip directories)
		if info.IsDir() {
			return nil
		}

		data, err := os.ReadFile(path)
		if err != nil {
			logger.Warnf("Failed to read file %s: %v", path, err)
			return nil
		}

		// Search for database patterns and version patterns in the file content
		for dbType, dbPattern := range DatabasePatterns {
			if !dbPattern.Match(data) {
				continue
			}

			version := "unknown"
			if versionPattern, ok := VersionPatterns[dbType]; ok {
				version = extractVersion(string(data), versionPattern)
			}

			logger.Debugf("Detected database type %s (version %s) in file %s", dbType, version, path)

			// Update or add the database detection result in the map
			if existing, exists := detectedDatabases[dbType]; exists && existing.Version == "unknown" || !exists{
				// Only overwrite if the existing version is "unknown"
				detectedDatabases[dbType] = &DatabaseDetectionResult{
					Type:    string(dbType),
					Version: version,
					Source:  path,
				}
			}
		}

		processedFiles++
		progressTracker.UpdateProgress(processedFiles, totalFiles)
		return nil
	})

	if err != nil {
		logger.Errorf("Error scanning repository: %v", err)
		return nil, err
	}

	// Convert the map to a slice for the final result
	var result []DatabaseDetectionResult
	for _, db := range detectedDatabases {
		result = append(result, *db)
	}

	// Sort the result by database type for consistent ordering
	sort.Slice(result, func(i, j int) bool {
		return result[i].Type < result[j].Type
	})

	return result, nil
}

// extractVersion extracts the version number from the given data string using the provided regular expression.
// It returns the extracted version number as a string, or "unknown" if no valid version number is found.
func extractVersion(data string, versionPattern *regexp.Regexp) string {
	matches := versionPattern.FindStringSubmatch(data)

	// Regex to validate version format (e.g., "X.Y" or "X.Y.Z")
	versionFormat := regexp.MustCompile(`^\d+\.\d+(\.\d+)?$`)

	if len(matches) > 2 {
		for _, group := range matches[2:] {
			if versionFormat.MatchString(group) { // Check if the group matches the version format
				return group
			}
		}
	}
	return "unknown"
}

func calculateTotalFiles(targetDir string) (int, error) {
	var totalFiles int
	err := filepath.Walk(targetDir, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			totalFiles++
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	return totalFiles, nil
}
