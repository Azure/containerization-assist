package core

import (
	"fmt"
	"strings"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/core/config"
	errors "github.com/Azure/container-kit/pkg/mcp/errors"
)

// Value object types to prevent parameter confusion and add type safety
// These replace primitive string/int parameters in public APIs

// Identity types
type (
	SessionID   string
	RequestID   string
	UserID      string
	TraceID     string
	OperationID string
)

// Container types
type (
	ImageTag    string
	ImageName   string
	ContainerID string
	RegistryURL string
	Dockerfile  string
)

// Kubernetes types
type (
	ClusterName string
	Namespace   string
	PodName     string
	ServiceName string
	ConfigName  string
	SecretName  string
)

// Resource types
type (
	Bytes        int64
	Seconds      int64
	Milliseconds int64
	CPUCores     float64
	MemoryMB     int64
)

// File/Path types
type (
	FilePath      string
	DirectoryPath string
	AbsolutePath  string
	RelativePath  string
)

// Network types
type (
	Port int32
	Host string
	URL  string
)

// Validation methods for SessionID
func (s SessionID) Validate() error {
	if len(s) == 0 {
		return errors.NewTypedValidation("types", "session_id cannot be empty")
	}
	if len(s) > 64 {
		return errors.NewTypedValidation("types", "session ID too long (max 64 characters)")
	}
	// Session IDs should be alphanumeric with dashes/underscores
	for _, char := range s {
		if !((char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z') ||
			(char >= '0' && char <= '9') || char == '-' || char == '_') {
			return errors.NewTypedValidation("types", "session ID contains invalid characters")
		}
	}
	return nil
}

func (s SessionID) String() string {
	return string(s)
}

// Validation methods for ImageTag
func (t ImageTag) Validate() error {
	if len(t) == 0 {
		return errors.NewTypedValidation("types", "image_tag cannot be empty")
	}
	if len(t) > config.MaxPasswordLength {
		return errors.NewTypedValidation("types", "image tag too long (max 128 characters)")
	}
	if strings.Contains(string(t), " ") {
		return errors.NewTypedValidation("types", "image tag cannot contain spaces")
	}
	if strings.Contains(string(t), "..") {
		return errors.NewTypedValidation("types", "image tag cannot contain consecutive dots")
	}
	return nil
}

func (t ImageTag) String() string {
	return string(t)
}

// Validation methods for ImageName
func (n ImageName) Validate() error {
	if len(n) == 0 {
		return errors.NewTypedValidation("types", "image_name cannot be empty")
	}
	if len(n) > config.MaxNameLength {
		return errors.NewTypedValidation("types", "image name too long (max 255 characters)")
	}
	// Docker image names should be lowercase
	if strings.ToLower(string(n)) != string(n) {
		return errors.NewTypedValidation("types", "image name must be lowercase")
	}
	return nil
}

func (n ImageName) String() string {
	return string(n)
}

// Validation methods for Namespace
func (ns Namespace) Validate() error {
	if len(ns) == 0 {
		return errors.NewTypedValidation("types", "namespace cannot be empty")
	}
	if len(ns) > 63 {
		return errors.NewTypedValidation("types", "namespace too long (max 63 characters)")
	}
	// Kubernetes namespace names must be valid DNS labels
	if !isValidDNSLabel(string(ns)) {
		return errors.NewTypedValidation("types", "namespace must be a valid DNS label")
	}
	return nil
}

func (ns Namespace) String() string {
	return string(ns)
}

// Validation methods for ClusterName
func (c ClusterName) Validate() error {
	if len(c) == 0 {
		return errors.NewTypedValidation("types", "cluster_name cannot be empty")
	}
	if len(c) > 100 {
		return errors.NewTypedValidation("types", "cluster name too long (max 100 characters)")
	}
	return nil
}

func (c ClusterName) String() string {
	return string(c)
}

// Validation methods for Port
func (p Port) Validate() error {
	if p <= 0 {
		return errors.NewTypedValidation("types", "port must be positive")
	}
	if p > 65535 {
		return errors.NewTypedValidation("types", "port must be <= 65535")
	}
	return nil
}

func (p Port) String() string {
	return fmt.Sprintf("%d", p)
}

// Validation methods for FilePath
func (fp FilePath) Validate() error {
	if len(fp) == 0 {
		return errors.NewTypedValidation("types", "file_path cannot be empty")
	}
	if len(fp) > 4096 {
		return errors.NewTypedValidation("types", "file path too long (max 4096 characters)")
	}
	// Check for dangerous patterns
	pathStr := string(fp)
	if strings.Contains(pathStr, "..") {
		return errors.NewTypedValidation("types", "file path cannot contain '..'")
	}
	return nil
}

func (fp FilePath) String() string {
	return string(fp)
}

// Pretty printing methods for resource types

// String formats bytes in human-readable format
func (b Bytes) String() string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
		TB = GB * 1024
	)

	bytes := int64(b)
	switch {
	case bytes >= TB:
		return fmt.Sprintf("%.1fTB", float64(bytes)/TB)
	case bytes >= GB:
		return fmt.Sprintf("%.1fGB", float64(bytes)/GB)
	case bytes >= MB:
		return fmt.Sprintf("%.1fMB", float64(bytes)/MB)
	case bytes >= KB:
		return fmt.Sprintf("%.1fKB", float64(bytes)/KB)
	default:
		return fmt.Sprintf("%d bytes", bytes)
	}
}

// String formats seconds in human-readable duration
func (s Seconds) String() string {
	duration := time.Duration(s) * time.Second
	return duration.String()
}

// String formats milliseconds in human-readable duration
func (ms Milliseconds) String() string {
	duration := time.Duration(ms) * time.Millisecond
	return duration.String()
}

// String formats CPU cores with appropriate precision
func (cpu CPUCores) String() string {
	cores := float64(cpu)
	if cores >= 1.0 {
		return fmt.Sprintf("%.1f cores", cores)
	}
	return fmt.Sprintf("%.0fm", cores*1000) // millicores
}

// String formats memory in MB
func (mem MemoryMB) String() string {
	return fmt.Sprintf("%dMB", int64(mem))
}

// Helper functions

// isValidDNSLabel checks if a string is a valid DNS label according to RFC 1123
func isValidDNSLabel(label string) bool {
	if len(label) == 0 || len(label) > 63 {
		return false
	}

	for i, char := range label {
		if !((char >= 'a' && char <= 'z') || (char >= '0' && char <= '9') || char == '-') {
			return false
		}
		// Cannot start or end with hyphen
		if char == '-' && (i == 0 || i == len(label)-1) {
			return false
		}
	}

	return true
}

// Constructor functions for common patterns

// NewSessionID creates a validated SessionID
func NewSessionID(id string) (SessionID, error) {
	sessionID := SessionID(id)
	if err := sessionID.Validate(); err != nil {
		return "", err
	}
	return sessionID, nil
}

// NewImageTag creates a validated ImageTag
func NewImageTag(tag string) (ImageTag, error) {
	imageTag := ImageTag(tag)
	if err := imageTag.Validate(); err != nil {
		return "", err
	}
	return imageTag, nil
}

// NewNamespace creates a validated Namespace
func NewNamespace(ns string) (Namespace, error) {
	namespace := Namespace(ns)
	if err := namespace.Validate(); err != nil {
		return "", err
	}
	return namespace, nil
}

// NewPort creates a validated Port
func NewPort(port int32) (Port, error) {
	p := Port(port)
	if err := p.Validate(); err != nil {
		return 0, err
	}
	return p, nil
}
