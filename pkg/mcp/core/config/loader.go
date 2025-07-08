// Package config - Configuration loader and manager
// This file provides unified configuration loading from multiple sources
package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/errors"
	"gopkg.in/yaml.v3"
)

// Config represents the complete MCP server configuration
// This consolidates all configuration types into a single structure
type Config struct {
	// Server configuration
	Server *ServerConfig `json:"server" yaml:"server"`

	// Build configuration
	Build *BuildConfig `json:"build" yaml:"build"`

	// Deployment configuration
	Deploy *Deploy `json:"deploy" yaml:"deploy"`

	// Security scanning configuration
	Scan *ScanConfig `json:"scan" yaml:"scan"`
}

// ConfigLoader handles loading configuration from multiple sources
type ConfigLoader struct {
	// Configuration sources priority (first wins)
	configPaths []string
	envPrefix   string
}

// NewConfigLoader creates a new configuration loader
func NewConfigLoader() *ConfigLoader {
	return &ConfigLoader{
		configPaths: []string{
			"./config.yaml",
			"./config.yml",
			"./config.json",
			"~/.container-kit/config.yaml",
			"/etc/container-kit/config.yaml",
		},
		envPrefix: "MCP_",
	}
}

// WithConfigPaths sets custom configuration file paths
func (l *ConfigLoader) WithConfigPaths(paths ...string) *ConfigLoader {
	l.configPaths = paths
	return l
}

// WithEnvPrefix sets a custom environment variable prefix
func (l *ConfigLoader) WithEnvPrefix(prefix string) *ConfigLoader {
	l.envPrefix = prefix
	return l
}

// Load loads configuration from all available sources
func (l *ConfigLoader) Load() (*Config, error) {
	// Start with default configuration
	config := DefaultConfig()

	// Try to load from configuration files
	if err := l.loadFromFiles(config); err != nil {
		return nil, errors.NewError().Message("failed to load from config files").Cause(err).WithLocation().Build()
	}

	if err := l.loadFromEnv(config); err != nil {
		return nil, errors.NewError().Message("failed to load from environment").Cause(err).WithLocation().Build()
	}

	if err := config.Validate(); err != nil {
		return nil, errors.NewError().Message("configuration validation failed").Cause(err).WithLocation().Build()
	}

	return config, nil
}

// loadFromFiles attempts to load configuration from available files
func (l *ConfigLoader) loadFromFiles(config *Config) error {
	for _, path := range l.configPaths {
		// Expand home directory
		if strings.HasPrefix(path, "~/") {
			homeDir, err := os.UserHomeDir()
			if err != nil {
				continue
			}
			path = filepath.Join(homeDir, path[2:])
		}

		// Check if file exists
		if _, err := os.Stat(path); os.IsNotExist(err) {
			continue
		}

		// Load the file
		if err := l.loadFromFile(config, path); err != nil {
			return errors.NewError().Messagef("failed to load config from %s", path).Cause(err).WithLocation().Build()
		}

		break
	}

	return nil
}

// loadFromFile loads configuration from a specific file
func (l *ConfigLoader) loadFromFile(config *Config, filePath string) error {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return errors.NewError().Message("failed to read config file").Cause(err).WithLocation().Build()
	}

	ext := strings.ToLower(filepath.Ext(filePath))
	switch ext {
	case ".yaml", ".yml":
		return yaml.Unmarshal(data, config)
	case ".json":
		return json.Unmarshal(data, config)
	default:
		// Try YAML first, then JSON
		if err := yaml.Unmarshal(data, config); err != nil {
			if jsonErr := json.Unmarshal(data, config); jsonErr != nil {
				return errors.NewError().Messagef("unsupported config file format: %s", ext).WithLocation().Build()
			}
		}
	}

	return nil
}

// loadFromEnv loads configuration from environment variables
func (l *ConfigLoader) loadFromEnv(config *Config) error {
	return l.setFieldsFromEnv(reflect.ValueOf(config).Elem(), "")
}

// setFieldsFromEnv recursively sets struct fields from environment variables
func (l *ConfigLoader) setFieldsFromEnv(v reflect.Value, prefix string) error {
	t := v.Type()

	for i := 0; i < v.NumField(); i++ {
		field := v.Field(i)
		fieldType := t.Field(i)

		// Skip unexported fields
		if !field.CanSet() {
			continue
		}

		// Get environment variable name
		envTag := fieldType.Tag.Get("env")
		if envTag == "" {
			// Generate env var name from field name
			envName := l.envPrefix + strings.ToUpper(fieldType.Name)
			if prefix != "" {
				envName = l.envPrefix + prefix + "_" + strings.ToUpper(fieldType.Name)
			}
			envTag = envName
		}

		// Handle nested structs
		if field.Kind() == reflect.Struct {
			newPrefix := prefix
			if newPrefix != "" {
				newPrefix += "_"
			}
			newPrefix += strings.ToUpper(fieldType.Name)

			if err := l.setFieldsFromEnv(field, newPrefix); err != nil {
				return err
			}
			continue
		}

		// Handle pointer to struct
		if field.Kind() == reflect.Ptr && field.Type().Elem().Kind() == reflect.Struct {
			if field.IsNil() {
				field.Set(reflect.New(field.Type().Elem()))
			}

			newPrefix := prefix
			if newPrefix != "" {
				newPrefix += "_"
			}
			newPrefix += strings.ToUpper(fieldType.Name)

			if err := l.setFieldsFromEnv(field.Elem(), newPrefix); err != nil {
				return err
			}
			continue
		}

		// Get environment variable value
		envValue := os.Getenv(envTag)
		if envValue == "" {
			continue
		}

		// Set field value based on type
		if err := l.setFieldFromString(field, envValue, fieldType.Tag); err != nil {
			return errors.NewError().Messagef("failed to set field %s from env %s", fieldType.Name, envTag).Cause(err).WithLocation().Build()
		}
	}

	return nil
}

// setFieldFromString sets a field value from a string representation
func (l *ConfigLoader) setFieldFromString(field reflect.Value, value string, tag reflect.StructTag) error {
	switch field.Kind() {
	case reflect.String:
		field.SetString(value)

	case reflect.Bool:
		b, err := strconv.ParseBool(value)
		if err != nil {
			return err
		}
		field.SetBool(b)

	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		// Handle time.Duration specially
		if field.Type() == reflect.TypeOf(time.Duration(0)) {
			d, err := time.ParseDuration(value)
			if err != nil {
				return err
			}
			field.SetInt(int64(d))
		} else {
			i, err := strconv.ParseInt(value, 10, 64)
			if err != nil {
				return err
			}
			field.SetInt(i)
		}

	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		u, err := strconv.ParseUint(value, 10, 64)
		if err != nil {
			return err
		}
		field.SetUint(u)

	case reflect.Float32, reflect.Float64:
		f, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return err
		}
		field.SetFloat(f)

	case reflect.Slice:
		// Handle string slices
		if field.Type().Elem().Kind() == reflect.String {
			values := strings.Split(value, ",")
			for i, v := range values {
				values[i] = strings.TrimSpace(v)
			}
			field.Set(reflect.ValueOf(values))
		}

	case reflect.Map:
		// Handle map[string]string
		if field.Type().Key().Kind() == reflect.String && field.Type().Elem().Kind() == reflect.String {
			m := make(map[string]string)
			pairs := strings.Split(value, ",")
			for _, pair := range pairs {
				kv := strings.SplitN(strings.TrimSpace(pair), "=", 2)
				if len(kv) == 2 {
					m[strings.TrimSpace(kv[0])] = strings.TrimSpace(kv[1])
				}
			}
			field.Set(reflect.ValueOf(m))
		}
	}

	return nil
}

// DefaultConfig returns a configuration with all default values
func DefaultConfig() *Config {
	return &Config{
		Server: DefaultServerConfig(),
		Build:  DefaultBuildConfig(),
		Deploy: DefaultDeploy(),
		Scan:   DefaultScanConfig(),
	}
}

// Validate validates the entire configuration
func (c *Config) Validate() error {
	if c.Server != nil {
		if err := c.Server.Validate(); err != nil {
			return errors.NewError().Message("server config validation failed").Cause(err).WithLocation().Build()
		}
	}

	if c.Build != nil {
		if err := c.Build.Validate(); err != nil {
			return errors.NewError().Message("build config validation failed").Cause(err).WithLocation().Build()
		}
	}

	if c.Deploy != nil {
		if err := c.Deploy.Validate(); err != nil {
			return errors.NewError().Message("deploy config validation failed").Cause(err).WithLocation().Build()
		}
	}

	if c.Scan != nil {
		if err := c.Scan.Validate(); err != nil {
			return errors.NewError().Message("scan config validation failed").Cause(err).WithLocation().Build()
		}
	}

	return nil
}

// SaveToFile saves the configuration to a file
func (c *Config) SaveToFile(filePath string) error {
	// Determine format based on extension
	ext := strings.ToLower(filepath.Ext(filePath))

	var data []byte
	var err error

	switch ext {
	case ".yaml", ".yml":
		data, err = yaml.Marshal(c)
	case ".json":
		data, err = json.MarshalIndent(c, "", "  ")
	default:
		return errors.NewError().Messagef("unsupported file format: %s", ext).WithLocation().Build()
	}

	if err != nil {
		return errors.NewError().Message("failed to marshal config").Cause(err).WithLocation().Build()
	}

	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return errors.NewError().Message("failed to create config directory").Cause(err).WithLocation().Build()
	}

	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return errors.NewError().Message("failed to write config file").Cause(err).WithLocation().Build()
	}

	return nil
}

func (c *Config) GetServerConfig() *ServerConfig {
	if c.Server == nil {
		c.Server = DefaultServerConfig()
	}
	return c.Server
}

// GetBuildConfig returns the build configuration with defaults
func (c *Config) GetBuildConfig() *BuildConfig {
	if c.Build == nil {
		c.Build = DefaultBuildConfig()
	}
	return c.Build
}

// GetDeployConfig returns the deploy configuration with defaults
func (c *Config) GetDeployConfig() *Deploy {
	if c.Deploy == nil {
		c.Deploy = DefaultDeploy()
	}
	return c.Deploy
}

// GetScanConfig returns the scan configuration with defaults
func (c *Config) GetScanConfig() *ScanConfig {
	if c.Scan == nil {
		c.Scan = DefaultScanConfig()
	}
	return c.Scan
}
