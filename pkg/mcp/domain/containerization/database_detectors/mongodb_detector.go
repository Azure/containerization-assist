package database_detectors //nolint:revive // package name follows project conventions

import (
	"regexp"
	"strings"
)

// MongoDBDetector implements MongoDB detection logic using the template method pattern
type MongoDBDetector struct {
	*BaseDetector
}

// NewMongoDBDetector creates a new MongoDB detector with MongoDB-specific configuration
func NewMongoDBDetector() *MongoDBDetector {
	config := DatabaseDetectorConfig{
		DatabaseType:   MongoDB,
		DefaultPort:    27017,
		ServiceNames:   []string{"mongo", "mongodb"},
		EnvVarPrefixes: []string{"MONGO_", "MONGODB_", "DB_"},
		ConfigFiles:    []string{"mongod.conf", "mongodb.conf"},
		ConnectionStringPatterns: []*regexp.Regexp{
			regexp.MustCompile(`mongodb://[^\s\'"]+`),
			regexp.MustCompile(`mongodb\+srv://[^\s\'"]+`),
		},
		CodePatterns: []*regexp.Regexp{
			regexp.MustCompile(`(?i)mongodb`),
			regexp.MustCompile(`(?i)mongo`),
			regexp.MustCompile(`(?i)import.*mongo`),
			regexp.MustCompile(`(?i)from.*mongo`),
		},
		FilePatterns: []string{
			"*.js", "*.ts", "*.jsx", "*.tsx",
			"*.py", "*.go", "*.rb", "*.php",
			"*.java", "*.kt", "*.cs",
			"*.env", "*.env.*",
			"*.yml", "*.yaml", "*.json",
			"*.conf", "*.config",
		},
	}

	detector := &MongoDBDetector{}
	detector.BaseDetector = NewBaseDetector(config, detector)
	return detector
}

// Detect performs MongoDB detection in the repository using the template method
func (d *MongoDBDetector) Detect(repoPath string) ([]DetectedDatabase, error) {
	return d.BaseDetector.Detect(repoPath)
}

// extractDockerConnectionInfo extracts MongoDB-specific connection info from Docker environment
func (d *MongoDBDetector) extractDockerConnectionInfo(db *DetectedDatabase, env map[string]string) {
	if host, exists := env["MONGO_HOST"]; exists {
		db.ConnectionInfo.Host = host
	}
	if port, exists := env["MONGO_PORT"]; exists {
		if p := parsePort(port); p > 0 {
			db.ConnectionInfo.Port = p
		}
	}
	if user, exists := env["MONGO_INITDB_ROOT_USERNAME"]; exists {
		db.ConnectionInfo.Username = user
	}
	if database, exists := env["MONGO_INITDB_DATABASE"]; exists {
		db.ConnectionInfo.Database = database
	}
}

// extractEnvConnectionInfo extracts MongoDB-specific connection info from environment variables
func (d *MongoDBDetector) extractEnvConnectionInfo(db *DetectedDatabase, name, value string) {
	switch strings.ToUpper(name) {
	case "MONGO_HOST", "MONGODB_HOST", "DB_HOST":
		db.ConnectionInfo.Host = value
	case "MONGO_PORT", "MONGODB_PORT", "DB_PORT":
		if p := parsePort(value); p > 0 {
			db.ConnectionInfo.Port = p
		}
	case "MONGO_USER", "MONGODB_USER", "MONGO_USERNAME", "MONGODB_USERNAME":
		db.ConnectionInfo.Username = value
	case "MONGO_PASSWORD", "MONGODB_PASSWORD":
		// Note: Password info is not stored in connection info for security
	case "MONGO_DATABASE", "MONGODB_DATABASE", "MONGO_DB", "MONGODB_DB":
		db.ConnectionInfo.Database = value
	}
}

// GetSupportedTypes returns the database types this detector supports
func (d *MongoDBDetector) GetSupportedTypes() []DatabaseType {
	return []DatabaseType{MongoDB}
}

// ValidateConfiguration validates MongoDB configuration
func (d *MongoDBDetector) ValidateConfiguration(_ map[string]string) error {
	// Basic validation for MongoDB configuration
	return nil
}
