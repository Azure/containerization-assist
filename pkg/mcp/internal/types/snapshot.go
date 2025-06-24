package types

import (
	"time"
)

// SessionSnapshot represents a point-in-time snapshot of session state
type SessionSnapshot struct {
	Timestamp    time.Time              `json:"timestamp"`
	Manifests    map[string]K8sManifest `json:"manifests"`
	DeploymentID string                 `json:"deployment_id"`
}
