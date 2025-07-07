package database_detectors

import (
	"fmt"
	"regexp"
	"strings"
)

// PostgreSQLDetector implements PostgreSQL detection logic using template method pattern
type PostgreSQLDetector struct {
	*BaseDetector
}

// NewPostgreSQLDetector creates a new PostgreSQL detector with configured patterns
func NewPostgreSQLDetector() *PostgreSQLDetector {
	config := DatabaseDetectorConfig{
		DatabaseType:   PostgreSQL,
		DefaultPort:    5432,
		ServiceNames:   []string{"postgres", "postgresql"},
		EnvVarPrefixes: []string{"POSTGRES_", "POSTGRESQL_", "PG_"},
		ConfigFiles:    []string{"postgresql.conf", "pg_hba.conf", "pg_ident.conf"},
		ConnectionStringPatterns: []*regexp.Regexp{
			regexp.MustCompile(`(?i)postgresql://`),
			regexp.MustCompile(`(?i)postgres://`),
		},
		CodePatterns: []*regexp.Regexp{
			regexp.MustCompile(`(?i)psycopg[0-9]*`),            // Python PostgreSQL adapter
			regexp.MustCompile(`(?i)pg\s*\(`),                  // Node.js pg module
			regexp.MustCompile(`(?i)sequelize.*postgres`),      // Sequelize with PostgreSQL
			regexp.MustCompile(`(?i)gorm.*postgres`),           // Go GORM with PostgreSQL
			regexp.MustCompile(`(?i)database.*postgresql`),     // General database config
			regexp.MustCompile(`(?i)activerecord.*postgresql`), // Rails ActiveRecord
		},
		FilePatterns: []string{
			"*.js", "*.ts", "*.jsx", "*.tsx", // JavaScript/TypeScript
			"*.py",           // Python
			"*.go",           // Go
			"*.rb",           // Ruby
			"*.php",          // PHP
			"*.java", "*.kt", // Java/Kotlin
			"*.cs",             // C#
			"package.json",     // Node.js dependencies
			"requirements.txt", // Python dependencies
			"Pipfile",          // Python Pipenv
			"go.mod",           // Go modules
			"Gemfile",          // Ruby dependencies
			"composer.json",    // PHP dependencies
			"pom.xml",          // Java Maven
			"build.gradle",     // Java Gradle
			"*.csproj",         // .NET project files
		},
	}

	extractor := &PostgreSQLConnectionExtractor{}
	baseDetector := NewBaseDetector(config, extractor)

	return &PostgreSQLDetector{
		BaseDetector: baseDetector,
	}
}

// Detect performs PostgreSQL detection using the template method pattern
func (d *PostgreSQLDetector) Detect(repoPath string) ([]DetectedDatabase, error) {
	return d.BaseDetector.Detect(repoPath)
}

// PostgreSQLConnectionExtractor handles PostgreSQL-specific connection info extraction
type PostgreSQLConnectionExtractor struct{}

// extractDockerConnectionInfo extracts PostgreSQL connection info from Docker environment
func (e *PostgreSQLConnectionExtractor) extractDockerConnectionInfo(db *DetectedDatabase, env map[string]string) {
	if host, exists := env["POSTGRES_HOST"]; exists {
		db.ConnectionInfo.Host = host
	}
	if port, exists := env["POSTGRES_PORT"]; exists {
		if p := parsePort(port); p > 0 {
			db.ConnectionInfo.Port = p
		}
	}
	if dbName, exists := env["POSTGRES_DB"]; exists {
		db.ConnectionInfo.Database = dbName
	}
	if user, exists := env["POSTGRES_USER"]; exists {
		db.ConnectionInfo.Username = user
	}
}

// extractEnvConnectionInfo extracts PostgreSQL connection info from environment variables
func (e *PostgreSQLConnectionExtractor) extractEnvConnectionInfo(db *DetectedDatabase, name, value string) {
	switch strings.ToUpper(name) {
	case "POSTGRES_HOST", "POSTGRESQL_HOST", "PG_HOST":
		db.ConnectionInfo.Host = value
	case "POSTGRES_PORT", "POSTGRESQL_PORT", "PG_PORT":
		if p := parsePort(value); p > 0 {
			db.ConnectionInfo.Port = p
		}
	case "POSTGRES_DB", "POSTGRESQL_DB", "PG_DATABASE":
		db.ConnectionInfo.Database = value
	case "POSTGRES_USER", "POSTGRESQL_USER", "PG_USER":
		db.ConnectionInfo.Username = value
	}
}

// GetSupportedTypes returns the database types this detector supports
func (d *PostgreSQLDetector) GetSupportedTypes() []DatabaseType {
	return []DatabaseType{PostgreSQL}
}

// ValidateConfiguration validates PostgreSQL configuration
func (d *PostgreSQLDetector) ValidateConfiguration(config map[string]string) error {
	// Basic validation for PostgreSQL configuration
	return nil
}

// Helper functions
func parsePort(portStr string) int {
	// Simple port parsing - in a real implementation, you'd want more robust parsing
	var port int
	if _, err := fmt.Sscanf(portStr, "%d", &port); err == nil && port > 0 && port < 65536 {
		return port
	}
	return 0
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
