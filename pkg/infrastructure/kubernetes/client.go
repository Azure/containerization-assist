package kubernetes

import (
	"context"
	"time"
)

// Client interface defines kubernetes operations
type Client interface {
	// Deploy applies manifests to the cluster
	Deploy(ctx context.Context, manifests []byte, namespace string) error

	// Delete removes resources from the cluster
	Delete(ctx context.Context, manifests []byte, namespace string) error

	// Get retrieves resource information
	Get(ctx context.Context, resource, name, namespace string) (interface{}, error)

	// List lists resources in a namespace
	List(ctx context.Context, resource, namespace string) ([]interface{}, error)

	// GetStatus gets the status of a resource
	GetStatus(ctx context.Context, resource, name, namespace string) (*ResourceStatus, error)

	// WaitForReady waits for a resource to be ready
	WaitForReady(ctx context.Context, resource, name, namespace string, timeout time.Duration) error

	// GetLogs retrieves pod logs
	GetLogs(ctx context.Context, podName, namespace string, options LogOptions) ([]byte, error)

	// Rollback performs a rollback operation
	Rollback(ctx context.Context, resource, name, namespace string, revision int) error

	// Scale scales a resource
	Scale(ctx context.Context, resource, name, namespace string, replicas int) error

	// Validate validates kubernetes manifests
	Validate(ctx context.Context, manifests []byte) error
}

// ResourceStatus represents the status of a kubernetes resource
type ResourceStatus struct {
	Name       string                 `json:"name"`
	Namespace  string                 `json:"namespace"`
	Kind       string                 `json:"kind"`
	Ready      bool                   `json:"ready"`
	Status     string                 `json:"status"`
	Conditions []ResourceCondition    `json:"conditions,omitempty"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
}

// ResourceCondition represents a condition of a kubernetes resource
type ResourceCondition struct {
	Type    string `json:"type"`
	Status  string `json:"status"`
	Reason  string `json:"reason,omitempty"`
	Message string `json:"message,omitempty"`
}

// LogOptions configures log retrieval
type LogOptions struct {
	Container    string     `json:"container,omitempty"`
	Previous     bool       `json:"previous,omitempty"`
	Follow       bool       `json:"follow,omitempty"`
	Timestamps   bool       `json:"timestamps,omitempty"`
	SinceSeconds *int64     `json:"since_seconds,omitempty"`
	SinceTime    *time.Time `json:"since_time,omitempty"`
	TailLines    *int64     `json:"tail_lines,omitempty"`
	LimitBytes   *int64     `json:"limit_bytes,omitempty"`
}
