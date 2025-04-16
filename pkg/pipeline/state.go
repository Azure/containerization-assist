package pipeline

import (
	"github.com/Azure/container-copilot/pkg/docker"
	"github.com/Azure/container-copilot/pkg/k8s"
)

// PipelineState holds state across steps and iterations
type PipelineState struct {
	RepoFileTree   string
	Dockerfile     docker.Dockerfile
	RegistryURL    string
	ImageName      string
	K8sObjects     map[string]*k8s.K8sObject
	Success        bool
	IterationCount int
	Metadata       map[string]interface{} //Flexible storage //Could store summary of changes that will get displayed to the user at the end
}
