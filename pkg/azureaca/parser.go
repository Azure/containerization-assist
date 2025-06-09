package azureaca

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/Azure/container-copilot/pkg/logger"
)

// ParseACAJSON loads an Azure Container App export (JSON) and converts it into
// an ACAConfig. The export can be produced with:
//
//	az containerapp export --name <app> --resource-group <rg> --output json
func ParseACAJSON(path string) (*ACAConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read ACA file %s: %w", path, err)
	}

	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("failed to parse ACA JSON: %w", err)
	}

	props, ok := raw["properties"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid ACA JSON: missing properties block")
	}
	template, ok := props["template"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid ACA JSON: missing template block")
	}
	containers, ok := template["containers"].([]interface{})
	if !ok || len(containers) == 0 {
		return nil, fmt.Errorf("invalid ACA JSON: no containers defined")
	}
	container, ok := containers[0].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid ACA JSON: container not an object")
	}

	// Env vars (only static values for now)
	envs := map[string]string{}
	if envList, ok := container["env"].([]interface{}); ok {
		for _, item := range envList {
			e := item.(map[string]interface{})
			name, _ := e["name"].(string)
			if val, ok := e["value"].(string); ok {
				envs[name] = val
			} else {
				logger.Warnf("env var %s is not a literal value – skipped", name)
			}
		}
	}

	// Replicas
	replicas := int32(1)
	if scale, ok := props["scale"].(map[string]interface{}); ok {
		if r, ok := scale["minReplicas"].(float64); ok {
			replicas = int32(r)
		}
	}

	cpu := "0.5"
	memory := "512Mi"
	if res, ok := container["resources"].(map[string]interface{}); ok {
		if cpuVal, ok := res["cpu"].(string); ok {
			cpu = cpuVal
		}
		if memVal, ok := res["memory"].(string); ok {
			memory = memVal
		}
	}

	port := int32(80)
	if ports, ok := container["ports"].([]interface{}); ok && len(ports) > 0 {
		if first, ok := ports[0].(map[string]interface{}); ok {
			if p, ok := first["port"].(float64); ok {
				port = int32(p)
			}
		}
	}

	return &ACAConfig{
		Name:          raw["name"].(string),
		Image:         container["image"].(string),
		Env:           envs,
		CPU:           cpu,
		Memory:        memory,
		Replicas:      replicas,
		Port:          port,
		Ingress:       true, // assume for now
		LivenessPath:  "/healthz",
		ReadinessPath: "/readyz",
	}, nil
}
