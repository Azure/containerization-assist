package analyze

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/application/api"
	"github.com/Azure/container-kit/pkg/mcp/application/core"
	"github.com/Azure/container-kit/pkg/mcp/domain/containerization/database_detectors"
	"github.com/Azure/container-kit/pkg/mcp/domain/errors"
	"github.com/Azure/container-kit/pkg/mcp/domain/types"
	"github.com/Azure/container-kit/pkg/mcp/domain/validation"
	"github.com/localrivet/gomcp/server"
)

// AtomicDetectDatabasesTool implements database detection with CLI parity
// It uses session context for database detection workflow management.
type AtomicDetectDatabasesTool struct {
	detectors map[database_detectors.DatabaseType]database_detectors.DatabaseDetector
	logger    *slog.Logger
}

// NewAtomicDetectDatabasesTool creates a new database detection tool with the specified dependencies.
func NewAtomicDetectDatabasesTool(logger *slog.Logger) *AtomicDetectDatabasesTool {
	return &AtomicDetectDatabasesTool{
		detectors: map[database_detectors.DatabaseType]database_detectors.DatabaseDetector{
			database_detectors.PostgreSQL: database_detectors.NewPostgreSQLDetector(),
			database_detectors.MySQL:      database_detectors.NewMySQLDetector(),
			database_detectors.MongoDB:    database_detectors.NewMongoDBDetector(),
			database_detectors.Redis:      database_detectors.NewRedisDetector(),
		},
		logger: logger.With("tool", "atomic_detect_databases"),
	}
}

// GetMetadata returns tool metadata
func (t *AtomicDetectDatabasesTool) GetMetadata() api.ToolMetadata {
	return api.ToolMetadata{
		Name:         "atomic_detect_databases",
		Description:  "Detects database usage in repositories with CLI parity",
		Version:      "1.0.0",
		Category:     api.ToolCategory("analyze"),
		Tags:         []string{"analysis", "database", "detection"},
		Status:       api.ToolStatus("active"),
		Dependencies: []string{},
		Capabilities: []string{
			"postgresql_detection",
			"mysql_detection",
			"mongodb_detection",
			"redis_detection",
			"docker_compose_analysis",
			"environment_variable_detection",
			"configuration_file_analysis",
		},
		Requirements: []string{
			"repository_access",
		},
		RegisteredAt: time.Now(),
		LastModified: time.Now(),
	}
}

// ExecuteWithContext executes database detection with the provided arguments.
// It validates inputs, detects databases using multiple evidence sources, and returns detection results.
//
// Returns:
//   - *DatabaseDetectionResult: Detection results including session context
//   - error: Any validation or execution errors
func (t *AtomicDetectDatabasesTool) ExecuteWithContext(ctx *server.Context, args *database_detectors.DatabaseDetectionParams) (*database_detectors.DatabaseDetectionResult, error) {
	startTime := time.Now()

	t.logger.Info("Starting database detection",
		"session_id", args.SessionID,
		"dry_run", args.DryRun,
		"repository_path", args.RepositoryPath)

	// Validate parameters
	if err := args.Validate(); err != nil {
		return nil, errors.Validation("detect_databases", err.Error())
	}

	// If dry run, return early with success
	if args.DryRun {
		result := &database_detectors.DatabaseDetectionResult{
			BaseToolResponse:    types.BaseToolResponse{Success: false, Timestamp: time.Now()},
			BaseAIContextResult: core.NewBaseAIContextResult("database_detection", true, time.Since(startTime)),
			Success:             true,
			DatabasesFound:      []database_detectors.DetectedDatabase{},
			ConfigFiles:         []database_detectors.DatabaseConfigFile{},
			Suggestions:         []string{"Dry run mode - no actual detection performed"},
			Metadata: database_detectors.DatabaseMetadata{
				ScanStarted:  startTime,
				ScanPath:     args.RepositoryPath,
				ScanDuration: time.Since(startTime),
			},
		}

		t.logger.Info("Database detection dry run completed",
			"session_id", args.SessionID,
			"duration", time.Since(startTime))

		return result, nil
	}

	// Execute actual detection
	execResult, err := t.Execute(context.Background(), *args)
	if err != nil {
		return nil, err
	}

	// Convert to pointer and update with session context
	result := &execResult
	result.BaseToolResponse = types.BaseToolResponse{Success: false, Timestamp: time.Now()}
	result.BaseAIContextResult = core.NewBaseAIContextResult("database_detection", true, time.Since(startTime))

	t.logger.Info("Database detection completed",
		"session_id", args.SessionID,
		"databases_found", len(result.DatabasesFound),
		"duration", time.Since(startTime))

	return result, nil
}

// Validate validates the tool parameters using tag-based validation
func (t *AtomicDetectDatabasesTool) Validate(ctx context.Context, args interface{}) error {
	// Validate using tag-based validation
	return validation.ValidateTaggedStruct(args)
}

// Execute performs database detection with parallel processing
func (t *AtomicDetectDatabasesTool) Execute(ctx context.Context, params database_detectors.DatabaseDetectionParams) (database_detectors.DatabaseDetectionResult, error) {
	startTime := time.Now()

	// Initialize result
	result := database_detectors.DatabaseDetectionResult{
		BaseToolResponse: types.BaseToolResponse{
			Timestamp: startTime,
		},
		BaseAIContextResult: core.BaseAIContextResult{
			AIContextType: "database_detection",
			IsSuccessful:  false,
		},
		Success:        false,
		DatabasesFound: []database_detectors.DetectedDatabase{},
		ConfigFiles:    []database_detectors.DatabaseConfigFile{},
		Suggestions:    []string{},
		Metadata: database_detectors.DatabaseMetadata{
			ScanStarted: startTime,
			ScanPath:    params.RepositoryPath,
		},
	}

	// Validate parameters
	if err := params.Validate(); err != nil {
		return result, errors.Validation("detect_databases", err.Error())
	}

	// Set default scan depth
	scanDepth := params.ScanDepth
	if scanDepth == 0 {
		scanDepth = 5
	}

	// Determine which detectors to run
	detectorsToRun := t.getDetectorsToRun(params.DetectTypes)

	// Run detection in parallel with controlled concurrency
	var wg sync.WaitGroup
	resultsChan := make(chan database_detectors.DetectedDatabase, len(detectorsToRun)*10) // Buffer for multiple databases per detector
	errorsChan := make(chan error, len(detectorsToRun))

	for dbType, detector := range detectorsToRun {
		wg.Add(1)
		go func(dt database_detectors.DatabaseType, d database_detectors.DatabaseDetector) {
			defer wg.Done()

			databases, err := d.Detect(params.RepositoryPath)
			if err != nil {
				errorsChan <- errors.Wrapf(err, "analyze", "failed to detect %s databases", dt)
				return
			}

			for _, db := range databases {
				// Validate and score confidence
				db.Confidence = database_detectors.ValidateDetection(db)
				resultsChan <- db
			}
		}(dbType, detector)
	}

	// Wait for all detectors to complete
	go func() {
		wg.Wait()
		close(resultsChan)
		close(errorsChan)
	}()

	// Collect results
	var detectionErrors []error
	detectedDatabases := make(map[database_detectors.DatabaseType]*database_detectors.DetectedDatabase)

	// Process results
	for db := range resultsChan {
		// Avoid duplicates, keep the one with higher confidence
		if existing, exists := detectedDatabases[db.Type]; !exists || existing.Confidence < db.Confidence {
			dbCopy := db
			detectedDatabases[db.Type] = &dbCopy
		}
	}

	// Process errors
	for err := range errorsChan {
		detectionErrors = append(detectionErrors, err)
	}

	// Convert map to slice
	for _, db := range detectedDatabases {
		result.DatabasesFound = append(result.DatabasesFound, *db)
	}

	// Generate suggestions based on detected databases
	result.Suggestions = t.generateSuggestions(result.DatabasesFound)

	// Update metadata
	endTime := time.Now()
	result.Metadata.ScanCompleted = endTime
	result.Metadata.ScanDuration = endTime.Sub(startTime)
	result.Metadata.DetectionRules = len(detectorsToRun)

	// Set success status
	if len(detectionErrors) > 0 && len(result.DatabasesFound) == 0 {
		// Complete failure case - return error
		return result, errors.NewError().
			Code(errors.CodeValidationFailed).
			Type(errors.ErrTypeBusiness).
			Severity(errors.SeverityMedium).
			Message("Database detection failed").
			Context("scan_path", params.RepositoryPath).
			Context("detector_count", len(detectorsToRun)).
			Cause(detectionErrors[0]).
			Build()
	}

	// Success case (with possible warnings)
	result.Success = true
	result.BaseAIContextResult.IsSuccessful = true
	result.BaseAIContextResult.Duration = time.Since(startTime)

	if len(detectionErrors) > 0 {
		result.Suggestions = append(result.Suggestions, "Some detection methods encountered errors but databases were still found")
	}

	return result, nil
}

// getDetectorsToRun determines which detectors to run based on parameters
func (t *AtomicDetectDatabasesTool) getDetectorsToRun(detectTypes []database_detectors.DatabaseType) map[database_detectors.DatabaseType]database_detectors.DatabaseDetector {
	detectorsToRun := make(map[database_detectors.DatabaseType]database_detectors.DatabaseDetector)

	if len(detectTypes) == 0 {
		// Run all detectors
		return t.detectors
	}

	// Run only specified detectors
	for _, dbType := range detectTypes {
		if detector, exists := t.detectors[dbType]; exists {
			detectorsToRun[dbType] = detector
		}
	}

	return detectorsToRun
}

// generateSuggestions generates recommendations based on detected databases
func (t *AtomicDetectDatabasesTool) generateSuggestions(databases []database_detectors.DetectedDatabase) []string {
	var suggestions []string

	if len(databases) == 0 {
		suggestions = append(suggestions, "No databases detected. Consider adding database configuration if your application uses one.")
		return suggestions
	}

	// Database-specific suggestions
	for _, db := range databases {
		switch db.Type {
		case database_detectors.PostgreSQL:
			suggestions = append(suggestions, "PostgreSQL detected: Consider using connection pooling (PgBouncer) for production deployments")
			if db.Version != "" && db.Version != "unknown" {
				suggestions = append(suggestions, "Use PostgreSQL version "+db.Version+" in your Dockerfile for consistency")
			}
		case database_detectors.MySQL:
			suggestions = append(suggestions, "MySQL detected: Consider using MySQL 8.0+ for better performance and security")
			suggestions = append(suggestions, "Configure proper character encoding (utf8mb4) for full Unicode support")
		case database_detectors.MongoDB:
			suggestions = append(suggestions, "MongoDB detected: Consider using replica sets for production deployments")
			suggestions = append(suggestions, "Enable authentication and use connection strings with proper credentials")
		case database_detectors.Redis:
			suggestions = append(suggestions, "Redis detected: Consider configuring persistence (RDB/AOF) based on your use case")
			suggestions = append(suggestions, "Use Redis clustering for high availability in production")
		}

		// Confidence-based suggestions
		if db.Confidence < 0.7 {
			suggestions = append(suggestions, "Low confidence detection for "+string(db.Type)+": Please verify database configuration")
		}
	}

	// Multi-database suggestions
	if len(databases) > 1 {
		suggestions = append(suggestions, "Multiple databases detected: Consider using Docker Compose for orchestration")
		suggestions = append(suggestions, "Ensure proper network configuration for inter-service communication")
	}

	return suggestions
}
