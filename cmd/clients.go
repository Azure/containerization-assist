package cmd

import (
	"github.com/Azure/containerization-assist/pkg/ai"
	"github.com/Azure/containerization-assist/pkg/core/docker"
	"github.com/Azure/containerization-assist/pkg/core/kind"
	"github.com/Azure/containerization-assist/pkg/core/kubernetes"
	"github.com/Azure/containerization-assist/pkg/pipeline"
)

// Clients holds all the client implementations for the CLI
type Clients struct {
	AzOpenAIClient ai.LLMClient
	Docker         docker.DockerClient
	Kind           kind.KindRunner
	Kube           kubernetes.KubeRunner
}

// Ensure Clients implements the pipeline interface
var _ pipeline.AllStageClients = (*Clients)(nil)

// GetAIClient returns the AI client
func (c *Clients) GetAIClient() ai.LLMClient {
	return c.AzOpenAIClient
}

// GetDockerClient returns the Docker client
func (c *Clients) GetDockerClient() docker.DockerClient {
	return c.Docker
}

// GetKubeClient returns the Kubernetes client
func (c *Clients) GetKubeClient() kubernetes.KubeRunner {
	return c.Kube
}

// SetAIClient replaces the AI client (used for tracking)
func (c *Clients) SetAIClient(client ai.LLMClient) {
	c.AzOpenAIClient = client
}
