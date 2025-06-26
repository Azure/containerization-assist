package clients

import (
	"github.com/Azure/container-kit/pkg/ai"
	"github.com/Azure/container-kit/pkg/docker"
	"github.com/Azure/container-kit/pkg/k8s"
	"github.com/Azure/container-kit/pkg/kind"
)

type Clients struct {
	AzOpenAIClient ai.LLMClient
	Docker         docker.DockerClient
	Kind           kind.KindRunner
	Kube           k8s.KubeRunner
}
