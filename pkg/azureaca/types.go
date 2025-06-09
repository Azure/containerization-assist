package azureaca

// ACAConfig is a normalized subset of an Azure Container App export, used
// to generate equivalent Kubernetes resources.
// Dockerfile generation is deliberately out-of-scope – the running image in
// the Container App is reused verbatim in the resulting Deployment.
//
// NOTE: This struct purposefully mirrors the fields consumed by
// k8s helper functions and the acatransform pipeline stage. Extend as needed.

type ACAConfig struct {
	Name          string            `json:"name"`
	Image         string            `json:"image"`
	Env           map[string]string `json:"env"`
	CPU           string            `json:"cpu"`
	Memory        string            `json:"memory"`
	Replicas      int32             `json:"replicas"`
	Port          int32             `json:"port"`
	Ingress       bool              `json:"ingress"`
	LivenessPath  string            `json:"liveness_path"`
	ReadinessPath string            `json:"readiness_path"`
}
