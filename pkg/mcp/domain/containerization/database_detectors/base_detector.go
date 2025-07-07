package database_detectors //nolint:revive // package name follows project conventions

import (
	"os"
	"path/filepath"
	"regexp"
)

// DatabaseDetectorConfig provides database-specific configuration for the template method
type DatabaseDetectorConfig struct {
	DatabaseType             DatabaseType
	DefaultPort              int
	ServiceNames             []string
	EnvVarPrefixes           []string
	ConfigFiles              []string
	ConnectionStringPatterns []*regexp.Regexp
	CodePatterns             []*regexp.Regexp
	FilePatterns             []string
}

// ConnectionInfoExtractor defines methods for extracting database-specific connection info
type ConnectionInfoExtractor interface {
	extractDockerConnectionInfo(db *DetectedDatabase, env map[string]string)
	extractEnvConnectionInfo(db *DetectedDatabase, name, value string)
}

// BaseDetector implements the template method pattern for database detection
type BaseDetector struct {
	config    DatabaseDetectorConfig
	extractor ConnectionInfoExtractor
}

// NewBaseDetector creates a base detector with the given configuration
func NewBaseDetector(config DatabaseDetectorConfig, extractor ConnectionInfoExtractor) *BaseDetector {
	return &BaseDetector{
		config:    config,
		extractor: extractor,
	}
}

// Detect implements the template method pattern - defines the algorithm skeleton
func (d *BaseDetector) Detect(repoPath string) ([]DetectedDatabase, error) {
	var databases []DetectedDatabase

	databases = d.detectFromDocker(repoPath, databases)

	databases = d.detectFromEnvironment(repoPath, databases)

	databases = d.detectFromConfigFiles(repoPath, databases)

	databases = d.detectFromConnectionStrings(repoPath, databases)

	databases = d.detectFromCode(repoPath, databases)

	return databases, nil
}

// detectFromDocker detects database from Docker Compose files
func (d *BaseDetector) detectFromDocker(repoPath string, databases []DetectedDatabase) []DetectedDatabase {
	if d == nil {
		return databases
	}
	dockerServices, err := DetectDockerComposeServices(repoPath, d.config.ServiceNames)
	if err != nil || len(dockerServices) == 0 {
		return databases
	}

	for _, service := range dockerServices {
		version := ExtractVersion(service.Image, d.config.ServiceNames)

		db := DetectedDatabase{
			Type:             d.config.DatabaseType,
			Version:          version,
			ConfigPath:       "",
			EnvironmentVars:  []string{},
			EvidenceSources:  []string{"docker-compose"},
			ValidationStatus: "active",
			ConnectionInfo: DatabaseConnectionInfo{
				Port: d.config.DefaultPort,
			},
		}

		if d.extractor != nil {
			d.extractor.extractDockerConnectionInfo(&db, service.Environment)
		}
		databases = append(databases, db)
	}

	return databases
}

// detectFromEnvironment detects database from environment variables
func (d *BaseDetector) detectFromEnvironment(repoPath string, databases []DetectedDatabase) []DetectedDatabase {
	envVars, err := DetectEnvironmentVariables(repoPath, d.config.EnvVarPrefixes)
	if err != nil || len(envVars) == 0 {
		return databases
	}

	if len(databases) == 0 {
		db := DetectedDatabase{
			Type:             d.config.DatabaseType,
			Version:          "unknown",
			EnvironmentVars:  []string{},
			EvidenceSources:  []string{"environment-variables"},
			ValidationStatus: "active",
			ConnectionInfo: DatabaseConnectionInfo{
				Port: d.config.DefaultPort,
			},
		}
		databases = append(databases, db)
	}

	for i := range databases {
		for _, envVar := range envVars {
			databases[i].EnvironmentVars = append(databases[i].EnvironmentVars, envVar.Name)
			if d.extractor != nil {
				d.extractor.extractEnvConnectionInfo(&databases[i], envVar.Name, envVar.Value)
			}
		}

		if !contains(databases[i].EvidenceSources, "environment-variables") {
			databases[i].EvidenceSources = append(databases[i].EvidenceSources, "environment-variables")
		}
	}

	return databases
}

// detectFromConfigFiles detects database from configuration files
func (d *BaseDetector) detectFromConfigFiles(repoPath string, databases []DetectedDatabase) []DetectedDatabase {
	configFiles, err := DetectConfigFiles(repoPath, d.config.ConfigFiles)
	if err != nil || len(configFiles) == 0 {
		return databases
	}

	// If we don't already have a database, create one
	if len(databases) == 0 {
		db := DetectedDatabase{
			Type:             d.config.DatabaseType,
			Version:          "unknown",
			EnvironmentVars:  []string{},
			EvidenceSources:  []string{"config-file"},
			ValidationStatus: "active",
			ConnectionInfo: DatabaseConnectionInfo{
				Port: d.config.DefaultPort,
			},
		}
		databases = append(databases, db)
	}

	// Add config file info to the database
	for i := range databases {
		if databases[i].ConfigPath == "" && len(configFiles) > 0 {
			databases[i].ConfigPath = configFiles[0].Path
		}

		if !contains(databases[i].EvidenceSources, "config-file") {
			databases[i].EvidenceSources = append(databases[i].EvidenceSources, "config-file")
		}
	}

	return databases
}

// detectFromConnectionStrings detects database from connection strings
func (d *BaseDetector) detectFromConnectionStrings(repoPath string, databases []DetectedDatabase) []DetectedDatabase {
	connectionStrings := d.findConnectionStrings(repoPath)
	if len(connectionStrings) == 0 {
		return databases
	}

	// If we don't already have a database, create one
	if len(databases) == 0 {
		db := DetectedDatabase{
			Type:             d.config.DatabaseType,
			Version:          "unknown",
			EnvironmentVars:  []string{},
			EvidenceSources:  []string{"connection-string"},
			ValidationStatus: "active",
			ConnectionInfo: DatabaseConnectionInfo{
				Port: d.config.DefaultPort,
			},
		}
		databases = append(databases, db)
	}

	// Add connection string evidence
	for i := range databases {
		if !contains(databases[i].EvidenceSources, "connection-string") {
			databases[i].EvidenceSources = append(databases[i].EvidenceSources, "connection-string")
		}
	}

	return databases
}

// detectFromCode detects database from source code
func (d *BaseDetector) detectFromCode(repoPath string, databases []DetectedDatabase) []DetectedDatabase {
	codeEvidence := d.findCodeEvidence(repoPath)
	if len(codeEvidence) == 0 {
		return databases
	}

	// If we don't already have a database, create one
	if len(databases) == 0 {
		db := DetectedDatabase{
			Type:             d.config.DatabaseType,
			Version:          "unknown",
			EnvironmentVars:  []string{},
			EvidenceSources:  []string{"source-code"},
			ValidationStatus: "active",
			ConnectionInfo: DatabaseConnectionInfo{
				Port: d.config.DefaultPort,
			},
		}
		databases = append(databases, db)
	}

	// Add source code evidence
	for i := range databases {
		if !contains(databases[i].EvidenceSources, "source-code") {
			databases[i].EvidenceSources = append(databases[i].EvidenceSources, "source-code")
		}
	}

	return databases
}

// findConnectionStrings finds connection strings in files using configured patterns
func (d *BaseDetector) findConnectionStrings(repoPath string) []string {
	var connectionStrings []string

	err := filepath.Walk(repoPath, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}

		// Check if file matches our patterns
		matched := false
		for _, pattern := range d.config.FilePatterns {
			if matched, _ = filepath.Match(pattern, info.Name()); matched {
				break
			}
		}

		if !matched {
			return nil
		}

		// Read and search file content
		content, err := os.ReadFile(path)
		if err != nil {
			return nil
		}

		contentStr := string(content)
		for _, pattern := range d.config.ConnectionStringPatterns {
			matches := pattern.FindAllString(contentStr, -1)
			connectionStrings = append(connectionStrings, matches...)
		}

		return nil
	})

	if err != nil {
		return []string{}
	}

	return connectionStrings
}

// findCodeEvidence finds evidence in source code using configured patterns
func (d *BaseDetector) findCodeEvidence(repoPath string) []string {
	var evidence []string

	err := filepath.Walk(repoPath, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}

		// Check if file matches our patterns
		matched := false
		for _, pattern := range d.config.FilePatterns {
			if matched, _ = filepath.Match(pattern, info.Name()); matched {
				break
			}
		}

		if !matched {
			return nil
		}

		// Read and search file content
		content, err := os.ReadFile(path)
		if err != nil {
			return nil
		}

		contentStr := string(content)
		for _, pattern := range d.config.CodePatterns {
			if pattern.MatchString(contentStr) {
				evidence = append(evidence, "found pattern in "+path)
			}
		}

		return nil
	})

	if err != nil {
		return []string{}
	}

	return evidence
}

// extractDockerConnectionInfo extracts connection info from Docker environment - to be overridden by subclasses
func (d *BaseDetector) extractDockerConnectionInfo(_ *DetectedDatabase, _ map[string]string) {
}

// extractEnvConnectionInfo extracts connection info from environment variables - to be overridden by subclasses
func (d *BaseDetector) extractEnvConnectionInfo(_ *DetectedDatabase, _, _ string) {
}
