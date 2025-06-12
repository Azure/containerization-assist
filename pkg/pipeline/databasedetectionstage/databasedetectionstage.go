package databasedetectionstage

import (
	"context"
	"os"
	"time"
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/Azure/container-copilot/pkg/logger"
	"github.com/Azure/container-copilot/pkg/pipeline"
)

// DatabaseType defines a custom type for database types.
type DatabaseType string

// Enum-like constants for known database types.
const (
	MySQL      DatabaseType = "MySQL"
	PostgreSQL DatabaseType = "PostgreSQL"
	MongoDB    DatabaseType = "MongoDB"
	Redis      DatabaseType = "Redis"
	Cassandra  DatabaseType = "Cassandra"
	DynamoDB   DatabaseType = "DynamoDB"
	SQLite     DatabaseType = "SQLite"
	SQLServer  DatabaseType = "SQLServer"
	CosmosDB   DatabaseType = "CosmosDB"
)

// KnownDatabaseTypes is a list of all valid database types.
var KnownDatabaseTypes = []DatabaseType{
	MySQL,
	PostgreSQL,
	MongoDB,
	Redis,
	Cassandra,
	DynamoDB,
	SQLite,
	SQLServer,
	CosmosDB,
}

type DatabaseDetectionResult struct {
	Type    string `json:"type"`
	Version string `json:"version"`
}

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
	// Use state.TargetDir as the path for both initialization and generation.
	if err := d.Initialize(ctx, state, targetDir); err != nil {
		return err
	}

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

	// Define key terms and version patterns for database detection
	databasePatterns := map[DatabaseType]*regexp.Regexp{
		MySQL:      regexp.MustCompile(`(?i)\bmysql\b|(?i)\bmariadb\b`),
		PostgreSQL: regexp.MustCompile(`(?i)\bpostgres\b|(?i)\bpostgresql\b`),
		MongoDB:    regexp.MustCompile(`(?i)\bmongodb\b`),
		Redis:      regexp.MustCompile(`(?i)\bredis\b`),
		Cassandra:  regexp.MustCompile(`(?i)\bcassandra\b`),
		DynamoDB:   regexp.MustCompile(`(?i)\bdynamodb\b`),
		SQLite:     regexp.MustCompile(`(?i)\bsqlite\b`),
		SQLServer:  regexp.MustCompile(`(?i)\bsqlserver\b|(?i)\bmssql\b`),
		CosmosDB:   regexp.MustCompile(`(?i)\bcosmosdb\b`),
	}

	versionPatterns := map[DatabaseType]*regexp.Regexp{
		MySQL:      regexp.MustCompile(`(?i)(mysql|mariadb)[\s-]?(\d+\.\d+(\.\d+)?)|<mysql\.version>(\d+\.\d+(\.\d+)?)</mysql\.version>|mysql\.version[\s-]?(\d+\.\d+(\.\d+)?)`),
		PostgreSQL: regexp.MustCompile(`(?i)(postgres|postgresql)[\s-]?(\d+\.\d+(\.\d+)?)|<postgresql\.version>(\d+\.\d+(\.\d+)?)</postgresql\.version>|postgresql\.version[\s-]?(\d+\.\d+(\.\d+)?)`),
		MongoDB:    regexp.MustCompile(`(?i)(mongodb)[\s-]?(\d+\.\d+(\.\d+)?)|<mongodb\.version>(\d+\.\d+(\.\d+)?)</mongodb\.version>|mongodb\.version[\s-]?(\d+\.\d+(\.\d+)?)`),
		Redis:      regexp.MustCompile(`(?i)(redis)[\s-]?(\d+\.\d+(\.\d+)?)|<redis\.version>(\d+\.\d+(\.\d+)?)</redis\.version>|redis\.version[\s-]?(\d+\.\d+(\.\d+)?)`),
		Cassandra:  regexp.MustCompile(`(?i)(cassandra)[\s-]?(\d+\.\d+(\.\d+)?)|<cassandra\.version>(\d+\.\d+(\.\d+)?)</cassandra\.version>|cassandra\.version[\s-]?(\d+\.\d+(\.\d+)?)`),
		DynamoDB:   regexp.MustCompile(`(?i)(dynamodb)[\s-]?(\d+\.\d+(\.\d+)?)|<dynamodb\.version>(\d+\.\d+(\.\d+)?)</dynamodb\.version>|dynamodb\.version[\s-]?(\d+\.\d+(\.\d+)?)`),
		SQLite:     regexp.MustCompile(`(?i)(sqlite)[\s-]?(\d+\.\d+(\.\d+)?)|<sqlite\.version>(\d+\.\d+(\.\d+)?)</sqlite\.version>|sqlite\.version[\s-]?(\d+\.\d+(\.\d+)?)`),
		SQLServer:  regexp.MustCompile(`(?i)(sqlserver|mssql)[\s-]?(\d+\.\d+(\.\d+)?)|<sqlserver\.version>(\d+\.\d+(\.\d+)?)</sqlserver\.version>|sqlserver\.version[\s-]?(\d+\.\d+(\.\d+)?)`),
		CosmosDB:   regexp.MustCompile(`(?i)(cosmosdb)[\s-]?(\d+\.\d+(\.\d+)?)|<cosmosdb\.version>(\d+\.\d+(\.\d+)?)</cosmosdb\.version>|cosmosdb\.version[\s-]?(\d+\.\d+(\.\d+)?)`),
	}

	// Scan the repository for database-related terms and versions
	spinner := []rune{'|', '/', '-', '\\'}
	var totalFiles int
	totalFiles, _ = calculateTotalFiles(targetDir)
	var processedFiles int
	var detectedDatabases []DatabaseDetectionResult

	
	err := filepath.Walk(targetDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Only scan files (skip directories)
		if !info.IsDir() {
			data, err := os.ReadFile(path)
			if err != nil {
				logger.Warnf("Failed to read file %s: %v", path, err)
				return nil
			}

			// Search for database patterns and version patterns in the file content
			for dbType, pattern := range databasePatterns {
				if pattern.Match(data) {
					version := "unknown"
					if versionPattern, ok := versionPatterns[dbType]; ok {
						matches := versionPattern.FindStringSubmatch(string(data))
						// Check all possible groups for a version match
						if len(matches) > 2 { 				
							for _, group := range matches[2:] {
								if group != "" {
									version = group
									break
								}
							}
						}
					}
					logger.Debugf("Detected database type %s (version %s) in file %s", dbType, version, path)
					detectedDatabases = append(detectedDatabases, DatabaseDetectionResult{
						Type:    string(dbType),
						Version: version,
					})
				}
			}
			processedFiles++
            progress := float64(processedFiles) / float64(totalFiles) * 100
            spinnerChar := spinner[processedFiles%len(spinner)]
			if progress == 100 {
				fmt.Printf("\r        Progress: [%-30s] %c 100.00%%\n", strings.Repeat("=", 30), spinnerChar)
			} else {
            	fmt.Printf("\r        Progress: [%-30s] %c %.2f%%", strings.Repeat("=", int(progress/3.33)), spinnerChar, progress)
				time.Sleep(25 * time.Millisecond)
			}
		}
		return nil
	})

	if err != nil {
		logger.Errorf("Error scanning repository: %v", err)
		return nil, err
	}

	// Remove duplicates from detected databases
	detectedDatabases = removeDuplicateDatabases(detectedDatabases)

	return detectedDatabases, nil
}

// removeDuplicateDatabases removes duplicate entries from the detected databases list.
func removeDuplicateDatabases(databases []DatabaseDetectionResult) []DatabaseDetectionResult {
	unique := make(map[string]DatabaseDetectionResult)
	unknownVersions := make(map[string]bool)

	// Iterate through the detected databases
	for _, db := range databases {
		if db.Version == "unknown" {
			unknownVersions[db.Type] = true
		} else {
			unique[db.Type] = db
		}
	}

	var result []DatabaseDetectionResult

	// Add databases with actual versions to the result
	for _, db := range unique {
		result = append(result, db)
	}

	// Add databases with "unknown" versions only if no actual version exists
	for dbType := range unknownVersions {
		if _, exists := unique[dbType]; !exists {
			result = append(result, DatabaseDetectionResult{
				Type:    dbType,
				Version: "unknown",
			})
		}
	}

	// Sort the result by database type for consistent ordering
	sort.Slice(result, func(i, j int) bool {
		return result[i].Type < result[j].Type
	})

	return result
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
