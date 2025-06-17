package clients

import (
	"github.com/Azure/container-copilot/pkg/ai"
	"github.com/Azure/container-copilot/pkg/docker"
	"github.com/Azure/container-copilot/pkg/k8s"
	"github.com/Azure/container-copilot/pkg/kind"
)

type Clients struct {
	AzOpenAIClient ai.LLMClient
	Docker         docker.DockerClient
	Kind           kind.KindRunner
	Kube           k8s.KubeRunner
}
