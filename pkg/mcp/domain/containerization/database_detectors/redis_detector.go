package database_detectors

import (
	"regexp"
	"strings"
)

// RedisDetector implements Redis detection logic using the template method pattern
type RedisDetector struct {
	*BaseDetector
}

// NewRedisDetector creates a new Redis detector with Redis-specific configuration
func NewRedisDetector() *RedisDetector {
	config := DatabaseDetectorConfig{
		DatabaseType:   Redis,
		DefaultPort:    6379,
		ServiceNames:   []string{"redis"},
		EnvVarPrefixes: []string{"REDIS_"},
		ConfigFiles:    []string{"redis.conf", "redis.config"},
		ConnectionStringPatterns: []*regexp.Regexp{
			regexp.MustCompile(`redis://[^\s\'"]+`),      // Standard Redis connection string
			regexp.MustCompile(`rediss://[^\s\'"]+`),     // Redis with SSL
			regexp.MustCompile(`redis\+tls://[^\s\'"]+`), // Redis with TLS
		},
		CodePatterns: []*regexp.Regexp{
			regexp.MustCompile(`(?i)redis`),
			regexp.MustCompile(`(?i)import.*redis`),
			regexp.MustCompile(`(?i)from.*redis`),
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

	detector := &RedisDetector{}
	detector.BaseDetector = NewBaseDetector(config, detector)
	return detector
}

// Detect performs Redis detection in the repository using the template method
func (d *RedisDetector) Detect(repoPath string) ([]DetectedDatabase, error) {
	return d.BaseDetector.Detect(repoPath)
}

// extractDockerConnectionInfo extracts Redis-specific connection info from Docker environment
func (d *RedisDetector) extractDockerConnectionInfo(db *DetectedDatabase, env map[string]string) {
	if host, exists := env["REDIS_HOST"]; exists {
		db.ConnectionInfo.Host = host
	}
	if port, exists := env["REDIS_PORT"]; exists {
		if p := parsePort(port); p > 0 {
			db.ConnectionInfo.Port = p
		}
	}
	if password, exists := env["REDIS_PASSWORD"]; exists && password != "" {
		db.ConnectionInfo.Username = "default" // Redis 6+ ACL default user
	}
	if redisDB, exists := env["REDIS_DB"]; exists {
		db.ConnectionInfo.Database = redisDB
	}
}

// extractEnvConnectionInfo extracts Redis-specific connection info from environment variables
func (d *RedisDetector) extractEnvConnectionInfo(db *DetectedDatabase, name, value string) {
	switch strings.ToUpper(name) {
	case "REDIS_HOST":
		db.ConnectionInfo.Host = value
	case "REDIS_PORT":
		if p := parsePort(value); p > 0 {
			db.ConnectionInfo.Port = p
		}
	case "REDIS_DB", "REDIS_DATABASE":
		db.ConnectionInfo.Database = value
	case "REDIS_PASSWORD":
		if value != "" {
			db.ConnectionInfo.Username = "default" // Redis 6+ ACL
		}
	}
}

// GetSupportedTypes returns the database types this detector supports
func (d *RedisDetector) GetSupportedTypes() []DatabaseType {
	return []DatabaseType{Redis}
}

// ValidateConfiguration validates Redis configuration
func (d *RedisDetector) ValidateConfiguration(_ map[string]string) error {
	// Basic validation for Redis configuration
	return nil
}
