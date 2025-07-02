package utils

import (
	"encoding/json"

	"github.com/invopop/jsonschema"
)

// RemoveCopilotIncompatibleFromSchema converts invopop jsonschema.Schema to map and removes incompatible fields
func RemoveCopilotIncompatibleFromSchema(schema *jsonschema.Schema) map[string]interface{} {
	// Marshal and unmarshal to get map format
	schemaBytes, err := json.Marshal(schema)
	if err != nil {
		return make(map[string]interface{})
	}

	var schemaMap map[string]interface{}
	if err := json.Unmarshal(schemaBytes, &schemaMap); err != nil {
		return make(map[string]interface{})
	}

	// Apply compatibility fixes
	RemoveCopilotIncompatible(schemaMap)

	return schemaMap
}

// AddMissingArrayItems recursively adds missing "items" fields for arrays
// that don't have them, which is required by MCP validation.
// It safely handles nested objects, arrays, and various JSON schema structures.
func AddMissingArrayItems(schema map[string]interface{}) {
	// Recursively process all map values
	for _, value := range schema {
		switch v := value.(type) {
		case map[string]interface{}:
			// Check if this is an array type definition without items
			if v["type"] == "array" {
				if _, hasItems := v["items"]; !hasItems {
					// Add default string items for array types
					// This is safe for most MCP array use cases
					v["items"] = map[string]interface{}{"type": "string"}
				}
			}
			// Recursively process nested objects (like "properties", "definitions", etc.)
			AddMissingArrayItems(v)

		case []interface{}:
			// Handle arrays of schema objects (like in "oneOf", "anyOf", etc.)
			for _, elem := range v {
				if m, ok := elem.(map[string]interface{}); ok {
					AddMissingArrayItems(m)
				}
			}
		}
	}
}

// RemoveCopilotIncompatible recursively strips meta-schema fields that
// Copilot's AJV-Draft-7 validator cannot handle.
func RemoveCopilotIncompatible(node map[string]any) {
	delete(node, "$schema")        // drop any draft URI
	delete(node, "$id")            // AJV rejects nested id
	delete(node, "$dynamicRef")    // draft-2020 keyword
	delete(node, "$dynamicAnchor") // draft-2020 keyword

	// draft-2020 unevaluatedProperties is also unsupported
	delete(node, "unevaluatedProperties")

	for _, v := range node { // walk children
		switch child := v.(type) {
		case map[string]any:
			RemoveCopilotIncompatible(child)
		case []any:
			for _, elem := range child {
				if m, ok := elem.(map[string]any); ok {
					RemoveCopilotIncompatible(m)
				}
			}
		}
	}
}
