package manifests

// checkContainerReferences checks if container references exist
func (v *ManifestValidator) checkContainerReferences(container map[string]interface{}, configMaps, secrets map[string]bool, result *ValidationResult) {
	// Check environment from ConfigMap/Secret
	if envFrom, ok := container["envFrom"].([]interface{}); ok {
		for _, env := range envFrom {
			if envMap, ok := env.(map[string]interface{}); ok {
				// Check ConfigMapRef
				if cmRef, ok := envMap["configMapRef"].(map[string]interface{}); ok {
					if name, ok := cmRef["name"].(string); ok && !configMaps[name] {
						result.Warnings = append(result.Warnings, "Referenced ConfigMap '"+name+"' not found in manifests")
					}
				}

				// Check SecretRef
				if secRef, ok := envMap["secretRef"].(map[string]interface{}); ok {
					if name, ok := secRef["name"].(string); ok && !secrets[name] {
						result.Warnings = append(result.Warnings, "Referenced Secret '"+name+"' not found in manifests")
					}
				}
			}
		}
	}

	// Check individual environment variables
	if env, ok := container["env"].([]interface{}); ok {
		for _, envVar := range env {
			if envMap, ok := envVar.(map[string]interface{}); ok {
				if valueFrom, ok := envMap["valueFrom"].(map[string]interface{}); ok {
					// Check ConfigMapKeyRef
					if cmKeyRef, ok := valueFrom["configMapKeyRef"].(map[string]interface{}); ok {
						if name, ok := cmKeyRef["name"].(string); ok && !configMaps[name] {
							result.Warnings = append(result.Warnings, "Referenced ConfigMap '"+name+"' not found in manifests")
						}
					}

					// Check SecretKeyRef
					if secKeyRef, ok := valueFrom["secretKeyRef"].(map[string]interface{}); ok {
						if name, ok := secKeyRef["name"].(string); ok && !secrets[name] {
							result.Warnings = append(result.Warnings, "Referenced Secret '"+name+"' not found in manifests")
						}
					}
				}
			}
		}
	}

	// Check volume mounts
	if volumeMounts, ok := container["volumeMounts"].([]interface{}); ok {
		// Would need to cross-reference with volumes in pod spec
		// This is a simplified check
		if len(volumeMounts) > 0 {
			result.Warnings = append(result.Warnings, "Container has volume mounts - ensure volumes are properly defined")
		}
	}
}
