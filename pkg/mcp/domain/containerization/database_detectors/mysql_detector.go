package database_detectors //nolint:revive // package name follows project conventions

import (
	"regexp"
	"strings"
)

// MySQLDetector implements MySQL detection logic using the template method pattern
type MySQLDetector struct {
	*BaseDetector
}

// NewMySQLDetector creates a new MySQL detector with MySQL-specific configuration
func NewMySQLDetector() *MySQLDetector {
	config := DatabaseDetectorConfig{
		DatabaseType:   MySQL,
		DefaultPort:    3306,
		ServiceNames:   []string{"mysql", "mariadb"},
		EnvVarPrefixes: []string{"MYSQL_", "DB_"},
		ConfigFiles:    []string{"my.cnf", "mysql.conf", ".my.cnf"},
		ConnectionStringPatterns: []*regexp.Regexp{
			regexp.MustCompile(`mysql://[^\s\'"]+`),
			regexp.MustCompile(`mysql\+pymysql://[^\s\'"]+`),
			regexp.MustCompile(`jdbc:mysql://[^\s\'"]+`),
		},
		CodePatterns: []*regexp.Regexp{
			regexp.MustCompile(`(?i)mysql`),
			regexp.MustCompile(`(?i)import.*mysql`),
			regexp.MustCompile(`(?i)from.*mysql`),
			regexp.MustCompile(`(?i)mariadb`),
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

	detector := &MySQLDetector{}
	detector.BaseDetector = NewBaseDetector(config, detector)
	return detector
}

// Detect performs MySQL detection in the repository using the template method
func (d *MySQLDetector) Detect(repoPath string) ([]DetectedDatabase, error) {
	return d.BaseDetector.Detect(repoPath)
}

// extractDockerConnectionInfo extracts MySQL-specific connection info from Docker environment
func (d *MySQLDetector) extractDockerConnectionInfo(db *DetectedDatabase, env map[string]string) {
	if host, exists := env["MYSQL_HOST"]; exists {
		db.ConnectionInfo.Host = host
	}
	if port, exists := env["MYSQL_PORT"]; exists {
		if p := parsePort(port); p > 0 {
			db.ConnectionInfo.Port = p
		}
	}
	if user, exists := env["MYSQL_USER"]; exists {
		db.ConnectionInfo.Username = user
	}
	// Note: Password info is not stored in connection info for security
	if database, exists := env["MYSQL_DATABASE"]; exists {
		db.ConnectionInfo.Database = database
	}
}

// extractEnvConnectionInfo extracts MySQL-specific connection info from environment variables
func (d *MySQLDetector) extractEnvConnectionInfo(db *DetectedDatabase, name, value string) {
	switch strings.ToUpper(name) {
	case "MYSQL_HOST", "DB_HOST":
		db.ConnectionInfo.Host = value
	case "MYSQL_PORT", "DB_PORT":
		if p := parsePort(value); p > 0 {
			db.ConnectionInfo.Port = p
		}
	case "MYSQL_USER", "DB_USER", "MYSQL_USERNAME", "DB_USERNAME":
		db.ConnectionInfo.Username = value
	case "MYSQL_PASSWORD", "DB_PASSWORD":
		// Note: Password info is not stored in connection info for security
	case "MYSQL_DATABASE", "DB_NAME", "MYSQL_DB":
		db.ConnectionInfo.Database = value
	}
}

// GetSupportedTypes returns the database types this detector supports
func (d *MySQLDetector) GetSupportedTypes() []DatabaseType {
	return []DatabaseType{MySQL}
}

// ValidateConfiguration validates MySQL configuration
func (d *MySQLDetector) ValidateConfiguration(_ map[string]string) error {
	// Basic validation for MySQL configuration
	return nil
}
