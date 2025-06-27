package errors

import (
	"fmt"
	"time"
)

// Example usage of the unified CoreError system
// This file demonstrates how to create and use CoreError in different scenarios

// ExampleBuildError demonstrates creating a build error with full context
func ExampleBuildError() *CoreError {
	// Create a build error with comprehensive information
	err := Build("dockerfile", "Docker build failed due to missing base image")

	// Add context and diagnostics
	err = err.WithContext("dockerfile_path", "/app/Dockerfile").
		WithContext("base_image", "ubuntu:20.04").
		WithDiagnostics(&ErrorDiagnostics{
			RootCause:    "Base image 'ubuntu:20.04' not found in registry",
			ErrorPattern: "image_not_found",
			Symptoms:     []string{"pull access denied", "repository does not exist"},
			Checks: []DiagnosticCheck{
				{Name: "image_exists", Status: "fail", Details: "Image not found in registry"},
				{Name: "registry_access", Status: "pass", Details: "Registry accessible"},
			},
		}).
		WithResolution(&ErrorResolution{
			ImmediateSteps: []ResolutionStep{
				{Step: 1, Action: "Check image name spelling", Description: "Verify the base image name is correct"},
				{Step: 2, Action: "Try alternative base image", Command: "docker pull ubuntu:22.04", Expected: "Image pulled successfully"},
			},
			Alternatives: []Alternative{
				{Approach: "Use alpine base", Description: "Switch to Alpine Linux base image", Effort: "low", Risk: "low"},
				{Approach: "Build from scratch", Description: "Create custom base image", Effort: "high", Risk: "medium"},
			},
			RetryStrategy: &RetryStrategy{
				Retryable:   true,
				MaxAttempts: 3,
				BackoffMs:   5000,
				Conditions:  []string{"registry_available", "network_stable"},
			},
		})

	return err
}

// ExampleNetworkError demonstrates creating a network error
func ExampleNetworkError() *CoreError {
	err := Network("api_client", "Failed to connect to remote API")

	return err.WithContext("endpoint", "https://api.example.com").
		WithContext("timeout", "30s").
		WithSystemState(&SystemState{
			DockerAvailable: true,
			K8sConnected:    false,
			DiskSpaceMB:     15000,
			MemoryMB:        8192,
			LoadAverage:     0.8,
		}).
		WithDiagnostics(&ErrorDiagnostics{
			RootCause: "Network connectivity issue",
			Symptoms:  []string{"connection timeout", "DNS resolution failed"},
			Checks: []DiagnosticCheck{
				{Name: "dns_resolution", Status: "fail", Details: "Cannot resolve api.example.com"},
				{Name: "internet_access", Status: "pass", Details: "Can reach external sites"},
			},
		})
}

// ExampleSecurityError demonstrates creating a security error
func ExampleSecurityError() *CoreError {
	err := Security("vulnerability_scanner", "Critical vulnerability detected in dependencies")

	return err.WithContext("vulnerability_count", 3).
		WithContext("severity", "critical").
		WithFiles([]string{
			"/app/package.json",
			"/app/go.mod",
		}).
		WithDiagnostics(&ErrorDiagnostics{
			RootCause:    "Outdated dependencies with known vulnerabilities",
			ErrorPattern: "cve_detected",
			Symptoms:     []string{"npm audit warnings", "go mod vulnerabilities"},
		}).
		WithResolution(&ErrorResolution{
			ImmediateSteps: []ResolutionStep{
				{Step: 1, Action: "Update dependencies", Command: "npm audit fix", Expected: "Vulnerabilities resolved"},
				{Step: 2, Action: "Run security scan", Command: "npm audit", Expected: "0 vulnerabilities"},
			},
			ManualSteps: []string{
				"Review each vulnerability manually",
				"Check if updates break functionality",
				"Consider alternative packages if updates unavailable",
			},
		})
}

// ExampleErrorMigration demonstrates migrating from old error types
func ExampleErrorMigration() {
	// Example of migrating from different error types

	// Generic error
	genericErr := fmt.Errorf("something went wrong")
	coreErr := WrapError(genericErr, "example", "migration_demo")
	fmt.Printf("Generic error migrated: %v\n", coreErr.Error())

	// Check error properties
	fmt.Printf("Is retryable: %v\n", IsRetryable(coreErr))
	fmt.Printf("Category: %v\n", GetErrorCategory(coreErr))
	fmt.Printf("Severity: %v\n", GetErrorSeverity(coreErr))
}

// ExampleErrorChaining demonstrates error chaining and wrapping
func ExampleErrorChaining() *CoreError {
	// Create a chain of errors
	originalErr := fmt.Errorf("file not found: config.yaml")

	// Chain with original error
	chainedErr := Wrap(originalErr, "application", "startup failed")

	// The wrapped error preserves the original error chain
	fmt.Printf("Chained error: %v\n", chainedErr.Error())
	fmt.Printf("Original cause: %v\n", chainedErr.Unwrap())

	return chainedErr
}

// ExampleErrorWithSystemContext demonstrates adding system context
func ExampleErrorWithSystemContext() *CoreError {
	err := Resource("disk_manager", "Insufficient disk space for operation")

	return err.WithSystemState(&SystemState{
		DockerAvailable: true,
		K8sConnected:    true,
		DiskSpaceMB:     100, // Low disk space
		MemoryMB:        4096,
		LoadAverage:     2.5, // High load
	}).
		WithResourceUsage(&ResourceUsage{
			CPUPercent:     85.5,
			MemoryMB:       3800,
			DiskUsageMB:    95000,
			NetworkBytesTx: 1024000,
			NetworkBytesRx: 2048000,
		}).
		WithDiagnostics(&ErrorDiagnostics{
			RootCause: "Disk usage exceeded 95% threshold",
			Symptoms:  []string{"slow I/O operations", "temp file creation failing"},
			Checks: []DiagnosticCheck{
				{Name: "disk_space", Status: "fail", Details: "Only 100MB available"},
				{Name: "disk_health", Status: "pass", Details: "No hardware issues detected"},
			},
		}).
		WithResolution(&ErrorResolution{
			ImmediateSteps: []ResolutionStep{
				{Step: 1, Action: "Clean temporary files", Command: "find /tmp -type f -atime +7 -delete", Expected: "Free space increased"},
				{Step: 2, Action: "Remove old logs", Command: "journalctl --vacuum-time=7d", Expected: "Log files cleaned"},
			},
			Alternatives: []Alternative{
				{Approach: "Expand disk", Description: "Increase disk size", Effort: "medium", Risk: "low"},
				{Approach: "Move to external storage", Description: "Use network storage", Effort: "high", Risk: "medium"},
			},
		})
}

// ExampleErrorWithLogs demonstrates adding log context
func ExampleErrorWithLogs() *CoreError {
	err := Analysis("code_analyzer", "Static analysis failed")

	return err.WithLogs([]LogEntry{
		{
			Timestamp: time.Now().Add(-5 * time.Minute),
			Level:     "ERROR",
			Message:   "Syntax error in main.go:45",
			Source:    "linter",
		},
		{
			Timestamp: time.Now().Add(-3 * time.Minute),
			Level:     "WARN",
			Message:   "Unused variable 'result' in utils.go:12",
			Source:    "linter",
		},
		{
			Timestamp: time.Now().Add(-1 * time.Minute),
			Level:     "ERROR",
			Message:   "Analysis terminated due to syntax errors",
			Source:    "analyzer",
		},
	})
}

// ExampleErrorSerialization demonstrates JSON serialization
func ExampleErrorSerialization() {
	err := ExampleBuildError()

	// Serialize to JSON
	jsonData, jsonErr := err.ToJSON()
	if jsonErr != nil {
		fmt.Printf("Serialization error: %v\n", jsonErr)
		return
	}

	fmt.Printf("Error as JSON: %s\n", string(jsonData))
}
