package database_detectors

import (
	"time"

	"github.com/Azure/container-kit/pkg/mcp/application/core"
	"github.com/Azure/container-kit/pkg/mcp/domain/errors"
	"github.com/Azure/container-kit/pkg/mcp/domain/types"
)

// DatabaseType represents the type of database detected
type DatabaseType string

const (
	PostgreSQL DatabaseType = "PostgreSQL"
	MySQL      DatabaseType = "MySQL"
	MongoDB    DatabaseType = "MongoDB"
	Redis      DatabaseType = "Redis"
)

// DetectedDatabase represents a single detected database
type DetectedDatabase struct {
	Type             DatabaseType           `json:"type"`
	Version          string                 `json:"version,omitempty"`
	Confidence       float64                `json:"confidence"` // 0.0-1.0
	ConfigPath       string                 `json:"config_path,omitempty"`
	ConnectionInfo   DatabaseConnectionInfo `json:"connection_info,omitempty"`
	EnvironmentVars  []string               `json:"environment_vars,omitempty"`
	EvidenceSources  []string               `json:"evidence_sources"`  // What indicated this database
	ValidationStatus string                 `json:"validation_status"` // active, deprecated, test-only
}

// DatabaseConnectionInfo represents connection information for a database
type DatabaseConnectionInfo struct {
	Host     string `json:"host,omitempty"`
	Port     int    `json:"port,omitempty"`
	Database string `json:"database,omitempty"`
	Username string `json:"username,omitempty"`
	SSL      bool   `json:"ssl,omitempty"`
}

// DockerService represents a Docker service definition
type DockerService struct {
	Name        string            `json:"name"`
	Image       string            `json:"image"`
	Ports       []string          `json:"ports,omitempty"`
	Environment map[string]string `json:"environment,omitempty"`
	Volumes     []string          `json:"volumes,omitempty"`
}

// EnvironmentVar represents an environment variable
type EnvironmentVar struct {
	Name   string `json:"name"`
	Value  string `json:"value,omitempty"`
	Source string `json:"source"` // file path where found
}

// ConfigFile represents a configuration file
type ConfigFile struct {
	Path     string            `json:"path"`
	Type     string            `json:"type"`
	Settings map[string]string `json:"settings,omitempty"`
}

// DatabaseDetectionParams represents parameters for database detection
type DatabaseDetectionParams struct {
	types.BaseToolArgs                // REQUIRED: provides session_id, dry_run
	RepositoryPath     string         `json:"repository_path" validate:"required,secure_path" description:"Path to the repository to analyze"`
	ScanDepth          int            `json:"scan_depth,omitempty" validate:"omitempty,min=1,max=20" description:"Maximum directory depth to scan (default: 5)"`
	DetectTypes        []DatabaseType `json:"detect_types,omitempty" validate:"omitempty,dive,oneof=PostgreSQL MySQL MongoDB Redis" description:"Specific database types to detect (empty means all)"`
	IncludeConfig      bool           `json:"include_config,omitempty" description:"Include detailed configuration analysis"`
}

// Validate validates the detection parameters
func (p DatabaseDetectionParams) Validate() error {
	if p.RepositoryPath == "" {
		return errors.Validation("analyze", "repository_path is required")
	}
	return nil
}

// DatabaseDetectionResult represents the result of database detection
type DatabaseDetectionResult struct {
	types.BaseToolResponse                        // REQUIRED: version, tool, timestamp, session_id, dry_run
	core.BaseAIContextResult                      // REQUIRED: ai_context_type, duration, etc.
	Success                  bool                 `json:"success"` // REQUIRED: operation success indicator
	DatabasesFound           []DetectedDatabase   `json:"databases_found"`
	ConfigFiles              []DatabaseConfigFile `json:"config_files,omitempty"`
	Suggestions              []string             `json:"suggestions,omitempty"`
	Metadata                 DatabaseMetadata     `json:"metadata"`
}

// IsSuccess returns whether the detection was successful
func (r DatabaseDetectionResult) IsSuccess() bool {
	return r.Success
}

// DatabaseConfigFile represents a detected database configuration file
type DatabaseConfigFile struct {
	Path     string            `json:"path"`
	Type     string            `json:"type"` // config, docker-compose, env, etc.
	Database DatabaseType      `json:"database"`
	Settings map[string]string `json:"settings,omitempty"`
}

// DatabaseMetadata represents metadata about the detection process
type DatabaseMetadata struct {
	ScanStarted     time.Time     `json:"scan_started"`
	ScanCompleted   time.Time     `json:"scan_completed"`
	ScanDuration    time.Duration `json:"scan_duration"`
	ScanPath        string        `json:"scan_path"`
	FilesScanned    int           `json:"files_scanned"`
	DirectoriesSkip int           `json:"directories_skipped"`
	DetectionRules  int           `json:"detection_rules_applied"`
}

// DatabaseDetector interface for database-specific detection logic
type DatabaseDetector interface {
	Detect(repoPath string) ([]DetectedDatabase, error)
	GetSupportedTypes() []DatabaseType
	ValidateConfiguration(config map[string]string) error
}
