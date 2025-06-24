package core

import (
	"time"
)

// Version constants for schema evolution
const (
	CurrentSchemaVersion = "v1.0.0"
	ToolAPIVersion       = "2024.12.17"
)

// BaseToolResponse provides common response structure for all tools
type BaseToolResponse struct {
	Version   string    `json:"version"`    // Schema version (e.g., "v1.0.0")
	Tool      string    `json:"tool"`       // Tool name for correlation
	Timestamp time.Time `json:"timestamp"`  // Execution timestamp
	SessionID string    `json:"session_id"` // Session correlation
	DryRun    bool      `json:"dry_run"`    // Whether this was a dry-run
}

// BaseToolArgs provides common arguments for all tools
type BaseToolArgs struct {
	DryRun    bool   `json:"dry_run,omitempty" description:"Preview changes without executing"`
	SessionID string `json:"session_id,omitempty" description:"Session ID for state correlation"`
}

// ImageReference provides normalized image referencing across tools
type ImageReference struct {
	Registry   string `json:"registry,omitempty"`
	Repository string `json:"repository"`
	Tag        string `json:"tag"`
	Digest     string `json:"digest,omitempty"`
}

func (ir ImageReference) String() string {
	result := ir.Repository
	if ir.Registry != "" {
		result = ir.Registry + "/" + result
	}
	if ir.Tag != "" {
		result += ":" + ir.Tag
	}
	if ir.Digest != "" {
		result += "@" + ir.Digest
	}
	return result
}

// ResourceRequests defines Kubernetes resource requirements
type ResourceRequests struct {
	CPURequest    string `json:"cpu_request,omitempty"`
	MemoryRequest string `json:"memory_request,omitempty"`
	CPULimit      string `json:"cpu_limit,omitempty"`
	MemoryLimit   string `json:"memory_limit,omitempty"`
}

// SecretRef defines references to secrets in Kubernetes manifests
type SecretRef struct {
	Name string `json:"name"`
	Key  string `json:"key"`
	Env  string `json:"env"`
}

// PortForward defines port forwarding for Kind cluster testing
type PortForward struct {
	LocalPort  int    `json:"local_port"`
	RemotePort int    `json:"remote_port"`
	Service    string `json:"service,omitempty"`
	Pod        string `json:"pod,omitempty"`
}

// ResourceUtilization tracks system resource usage
type ResourceUtilization struct {
	CPU         float64 `json:"cpu_percent"`
	Memory      float64 `json:"memory_percent"`
	Disk        float64 `json:"disk_percent"`
	DiskFree    int64   `json:"disk_free_bytes"`
	LoadAverage float64 `json:"load_average"`
}

// ServiceHealth tracks health of external services
type ServiceHealth struct {
	Status       string        `json:"status"`
	LastCheck    time.Time     `json:"last_check"`
	ResponseTime time.Duration `json:"response_time,omitempty"`
	Error        string        `json:"error,omitempty"`
}

// NewBaseResponse creates a base response with current metadata
func NewBaseResponse(tool, sessionID string, dryRun bool) BaseToolResponse {
	return BaseToolResponse{
		Version:   CurrentSchemaVersion,
		Tool:      tool,
		Timestamp: time.Now(),
		SessionID: sessionID,
		DryRun:    dryRun,
	}
}
