package azureaca

import "github.com/Azure/container-copilot/pkg/k8s"

// GenerateK8sObjects converts an ACAConfig into basic Kubernetes objects.
func GenerateK8sObjects(cfg *ACAConfig) map[string]*k8s.K8sObject {
	deployment := k8s.NewDeployment(cfg.Name, cfg.Image, cfg.Port, cfg.Replicas, cfg.Env, cfg.CPU, cfg.Memory, cfg.LivenessPath, cfg.ReadinessPath)
	service := k8s.NewService(cfg.Name, cfg.Port, cfg.Ingress)
	configMap := k8s.NewConfigMap(cfg.Name, cfg.Env)

	objs := map[string]*k8s.K8sObject{
		"deployment": deployment,
		"service":    service,
	}
	if len(cfg.Env) > 0 {
		objs["configmap"] = configMap
	}
	return objs
}
