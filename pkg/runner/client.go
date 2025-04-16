package runner

import (
	"github.com/Azure/container-copilot/pkg/ai"
)

type Clients struct {
	AzOpenAIClient *ai.AzOpenAIClient
	Docker         DockerRunner
	Kind           KindRunner
	Kube           KubeRunner
}
