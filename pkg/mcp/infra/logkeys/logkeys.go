package logkeys

import (
	"time"

	"github.com/rs/zerolog"
)

// Standard logging keys used throughout Container Kit
const (
	// Identity keys
	KeySession   = "session_id"
	KeyTraceID   = "trace_id"
	KeyRequestID = "request_id"
	KeyUserID    = "user_id"

	// Container keys
	KeyImage     = "image"
	KeyContainer = "container_id"
	KeyRegistry  = "registry"
	KeyTag       = "tag"

	// Kubernetes keys
	KeyCluster   = "cluster"
	KeyNamespace = "namespace"
	KeyPod       = "pod"
	KeyService   = "service"

	// Operation keys
	KeyOperation = "operation"
	KeyDuration  = "duration_ms"
	KeyError     = "error"
	KeyResult    = "result"

	// File/Path keys
	KeyPath      = "path"
	KeyFile      = "file"
	KeyDirectory = "directory"

	// Resource keys
	KeyCPU    = "cpu_usage"
	KeyMemory = "memory_usage"
	KeyDisk   = "disk_usage"

	// Tool keys
	KeyTool     = "tool"
	KeyVersion  = "version"
	KeyStatus   = "status"
	KeyExitCode = "exit_code"

	// Network keys
	KeyHost = "host"
	KeyPort = "port"
	KeyURL  = "url"

	// Validation keys
	KeyValidator = "validator"
	KeyField     = "field"
	KeyValue     = "value"
	KeyCode      = "code"
)

// SessionContext adds session ID to log context
func SessionContext(sessionID string) func(zerolog.Context) zerolog.Context {
	return func(c zerolog.Context) zerolog.Context {
		return c.Str(KeySession, sessionID)
	}
}

// OperationContext adds operation name and duration to log context
func OperationContext(op string, duration time.Duration) func(zerolog.Context) zerolog.Context {
	return func(c zerolog.Context) zerolog.Context {
		return c.Str(KeyOperation, op).
			Int64(KeyDuration, duration.Milliseconds())
	}
}

// ErrorContext adds error information to log context
func ErrorContext(err error) func(zerolog.Context) zerolog.Context {
	return func(c zerolog.Context) zerolog.Context {
		return c.Err(err)
	}
}

// ToolContext adds tool information to log context
func ToolContext(toolName, version string) func(zerolog.Context) zerolog.Context {
	return func(c zerolog.Context) zerolog.Context {
		return c.Str(KeyTool, toolName).
			Str(KeyVersion, version)
	}
}

// ContainerContext adds container information to log context
func ContainerContext(image, containerID string) func(zerolog.Context) zerolog.Context {
	return func(c zerolog.Context) zerolog.Context {
		return c.Str(KeyImage, image).
			Str(KeyContainer, containerID)
	}
}

// KubernetesContext adds Kubernetes resource information to log context
func KubernetesContext(namespace, pod string) func(zerolog.Context) zerolog.Context {
	return func(c zerolog.Context) zerolog.Context {
		return c.Str(KeyNamespace, namespace).
			Str(KeyPod, pod)
	}
}

// FileContext adds file path information to log context
func FileContext(filePath string) func(zerolog.Context) zerolog.Context {
	return func(c zerolog.Context) zerolog.Context {
		return c.Str(KeyFile, filePath)
	}
}

// ValidationContext adds validation information to log context
func ValidationContext(validator, field, code string) func(zerolog.Context) zerolog.Context {
	return func(c zerolog.Context) zerolog.Context {
		return c.Str(KeyValidator, validator).
			Str(KeyField, field).
			Str(KeyCode, code)
	}
}

// ResourceContext adds resource usage information to log context
func ResourceContext(cpu, memory float64) func(zerolog.Context) zerolog.Context {
	return func(c zerolog.Context) zerolog.Context {
		return c.Float64(KeyCPU, cpu).
			Float64(KeyMemory, memory)
	}
}
