package customizer

import (
	"fmt"
)

// updateNestedValue updates a nested value in a YAML structure
func updateNestedValue(obj interface{}, value interface{}, path ...interface{}) error {
	if len(path) == 0 {
		return fmt.Errorf("path cannot be empty")
	}

	current := obj
	// Navigate to the parent of the final key
	for i := 0; i < len(path)-1; i++ {
		switch curr := current.(type) {
		case map[string]interface{}:
			keyStr, ok := path[i].(string)
			if !ok {
				return fmt.Errorf("non-string key at position %d", i)
			}
			next, exists := curr[keyStr]
			if !exists {
				// Create intermediate maps as needed
				curr[keyStr] = make(map[string]interface{})
				next = curr[keyStr]
			}
			current = next
		case []interface{}:
			keyInt, ok := path[i].(int)
			if !ok {
				return fmt.Errorf("non-integer key at position %d for array", i)
			}
			if keyInt >= len(curr) {
				return fmt.Errorf("array index %d out of bounds at position %d", keyInt, i)
			}
			current = curr[keyInt]
		default:
			return fmt.Errorf("cannot navigate through non-map/non-array at position %d", i)
		}
	}

	// Set the final value
	finalKey := path[len(path)-1]
	switch curr := current.(type) {
	case map[string]interface{}:
		keyStr, ok := finalKey.(string)
		if !ok {
			return fmt.Errorf("non-string final key")
		}
		curr[keyStr] = value
	case []interface{}:
		keyInt, ok := finalKey.(int)
		if !ok {
			return fmt.Errorf("non-integer final key for array")
		}
		if keyInt < len(curr) {
			curr[keyInt] = value
		} else {
			return fmt.Errorf("array index %d out of bounds for final key", keyInt)
		}
	default:
		return fmt.Errorf("cannot set value on non-map/non-array")
	}

	return nil
}

// updateLabelsInManifest updates labels in any Kubernetes manifest
func updateLabelsInManifest(manifest map[string]interface{}, labels map[string]string) error {
	if len(labels) == 0 {
		return nil
	}

	// Get existing metadata
	metadata, exists := manifest["metadata"]
	if !exists {
		metadata = make(map[string]interface{})
		manifest["metadata"] = metadata
	}

	metadataMap, ok := metadata.(map[string]interface{})
	if !ok {
		return fmt.Errorf("metadata is not a map")
	}

	// Get existing labels
	existingLabels, exists := metadataMap["labels"]
	if !exists {
		existingLabels = make(map[string]interface{})
		metadataMap["labels"] = existingLabels
	}

	labelsMap, ok := existingLabels.(map[string]interface{})
	if !ok {
		labelsMap = make(map[string]interface{})
		metadataMap["labels"] = labelsMap
	}

	// Add new labels
	for k, v := range labels {
		labelsMap[k] = v
	}

	return nil
}
