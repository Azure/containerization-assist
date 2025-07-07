package transport

import (
	"encoding/json"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/application/api"
	"github.com/Azure/container-kit/pkg/mcp/domain/types/tools"
)

// ============================================================================
// HTTP Handler Type Extensions
// ============================================================================

// This file contains type extension methods for HTTP transport types,
// providing backward compatibility and conversion functionality.

// ============================================================================
// ErrorResponse Extensions
// ============================================================================

// SetDetailsFromInterface sets details from legacy interface{} for backward compatibility.
// This method converts untyped error details into the strongly typed ErrorDetails structure.
func (er *ErrorResponse) SetDetailsFromInterface(details interface{}) {
	if details == nil {
		er.Details = nil
		return
	}

	// Convert interface{} to TypedErrorDetails
	if detailsMap, ok := details.(map[string]interface{}); ok {
		typedDetails := &tools.TypedErrorDetails{}

		if field, ok := detailsMap["field"].(string); ok {
			typedDetails.Field = field
		}
		if value, ok := detailsMap["value"].(string); ok {
			typedDetails.Value = value
		}
		if constraint, ok := detailsMap["constraint"].(string); ok {
			typedDetails.Constraint = constraint
		}
		if context, ok := detailsMap["context"].(map[string]string); ok {
			typedDetails.Context = context
		}
		if stackTrace, ok := detailsMap["stack_trace"].([]string); ok {
			typedDetails.StackTrace = stackTrace
		}
		if metadata, ok := detailsMap["metadata"].(map[string]string); ok {
			typedDetails.Metadata = metadata
		}

		er.Details = typedDetails
	}
}

// SetDetails sets typed error details directly.
// This is the preferred method for setting error details.
func (er *ErrorResponse) SetDetails(details *tools.TypedErrorDetails) {
	er.Details = details
}

// GetDetails returns the typed error details.
// This method provides access to the strongly typed error details.
func (er *ErrorResponse) GetDetails() *tools.TypedErrorDetails {
	return er.Details
}

// ============================================================================
// ToolDescription Conversion Methods
// ============================================================================

// FromCoreToolMetadata converts api.ToolMetadata to ToolDescription.
// This method bridges the gap between internal tool metadata and HTTP API representation.
func (td *ToolDescription) FromCoreToolMetadata(metadata api.ToolMetadata) {
	td.Name = metadata.Name
	td.Description = metadata.Description
	td.Version = metadata.Version
	td.Category = string(metadata.Category)
	// Note: api.ToolMetadata doesn't have a Schema field that matches TypedJSONSchema
	// Schema needs to be set separately using SetSchemaFromMap if needed
}

// ToCoreToolMetadata converts ToolDescription to api.ToolMetadata.
// This method converts HTTP API representation back to internal tool metadata.
func (td *ToolDescription) ToCoreToolMetadata() api.ToolMetadata {
	return api.ToolMetadata{
		Name:         td.Name,
		Description:  td.Description,
		Version:      td.Version,
		Category:     api.ToolCategory(td.Category),
		Status:       api.ToolStatus("active"),
		Tags:         []string{},
		RegisteredAt: time.Now(),
		LastModified: time.Now(),
		Dependencies: []string{}, // ToolDescription doesn't have these fields
		Capabilities: []string{},
		Requirements: []string{},
	}
}

// SetSchemaFromMap sets schema from legacy map[string]interface{} for backward compatibility.
// This method converts untyped schema into the strongly typed structure.
func (td *ToolDescription) SetSchemaFromMap(schemaMap map[string]interface{}) {
	if schemaMap == nil {
		td.Schema = nil
		return
	}

	schema := &tools.JSONSchema{}

	if schemaType, ok := schemaMap["type"].(string); ok {
		schema.Type = schemaType
	}

	if description, ok := schemaMap["description"].(string); ok {
		schema.Description = description
	}

	if required, ok := schemaMap["required"].([]string); ok {
		schema.Required = required
	}

	if properties, ok := schemaMap["properties"].(map[string]interface{}); ok {
		schema.Properties = make(map[string]*tools.TypedSchemaProperty)
		for name, prop := range properties {
			if propMap, ok := prop.(map[string]interface{}); ok {
				property := &tools.TypedSchemaProperty{}
				if propType, ok := propMap["type"].(string); ok {
					property.Type = propType
				}
				if propDesc, ok := propMap["description"].(string); ok {
					property.Description = propDesc
				}
				schema.Properties[name] = property
			}
		}
	}

	td.Schema = schema
}

// GetSchemaAsMap returns schema as map[string]interface{} for backward compatibility.
// This method converts the typed schema back to an untyped map.
func (td *ToolDescription) GetSchemaAsMap() map[string]interface{} {
	if td.Schema == nil {
		return nil
	}
	return td.Schema.ToMap()
}

// SetExampleFromInterface sets example from legacy interface{} for backward compatibility.
// This method converts untyped examples into the strongly typed structure.
func (td *ToolDescription) SetExampleFromInterface(example interface{}) {
	if example == nil {
		td.Example = nil
		return
	}

	// Convert interface{} to TypedToolExample
	if exampleMap, ok := example.(map[string]interface{}); ok {
		typedExample := &tools.TypedToolExample{}

		if name, ok := exampleMap["name"].(string); ok {
			typedExample.Name = name
		}
		if desc, ok := exampleMap["description"].(string); ok {
			typedExample.Description = desc
		}
		if input, exists := exampleMap["input"]; exists {
			// Convert to JSON for TypedToolExample
			if inputBytes, err := json.Marshal(input); err == nil {
				typedExample.Input = inputBytes
			}
		}
		if output, exists := exampleMap["output"]; exists {
			// Convert to JSON for TypedToolExample
			if outputBytes, err := json.Marshal(output); err == nil {
				typedExample.Output = outputBytes
			}
		}

		td.Example = typedExample
	}
}

// GetExampleAsInterface returns example as interface{} for backward compatibility.
// This method converts the typed example back to an untyped interface.
func (td *ToolDescription) GetExampleAsInterface() interface{} {
	if td.Example == nil {
		return nil
	}
	return td.Example.ToInterface()
}

// ============================================================================
// SessionInfo Metadata Methods
// ============================================================================

// SetMetadataFromMap sets metadata from a map for backward compatibility.
// This method converts untyped metadata into the strongly typed structure.
func (si *SessionInfo) SetMetadataFromMap(metadata map[string]interface{}) {
	if metadata == nil {
		si.Metadata = nil
		return
	}

	// Convert to typed metadata
	typedMetadata := &tools.TypedSessionMetadata{
		CustomFields: make(map[string]string),
	}

	// Extract known fields
	if userID, ok := metadata["user_id"].(string); ok {
		typedMetadata.UserID = userID
	}
	if source, ok := metadata["source"].(string); ok {
		typedMetadata.Source = source
	}
	if env, ok := metadata["environment"].(string); ok {
		typedMetadata.Environment = env
	}
	if lastTool, ok := metadata["last_tool"].(string); ok {
		typedMetadata.LastTool = lastTool
	}
	if toolCount, ok := metadata["tool_count"].(int); ok {
		typedMetadata.ToolCount = toolCount
	}
	if lastActivity, ok := metadata["last_activity"].(string); ok {
		if parsed, err := time.Parse(time.RFC3339, lastActivity); err == nil {
			typedMetadata.LastActivity = parsed
		}
	}
	if tags, ok := metadata["tags"].([]interface{}); ok {
		typedMetadata.Tags = make([]string, 0, len(tags))
		for _, tag := range tags {
			if tagStr, ok := tag.(string); ok {
				typedMetadata.Tags = append(typedMetadata.Tags, tagStr)
			}
		}
	}

	// Store remaining fields in custom map
	for k, v := range metadata {
		switch k {
		case "user_id", "source", "environment", "last_tool", "tool_count", "last_activity", "tags":
			// Already handled above
		default:
			if strVal, ok := v.(string); ok {
				typedMetadata.CustomFields[k] = strVal
			}
		}
	}

	si.Metadata = typedMetadata
}

// SetMetadata sets typed metadata directly.
// This is the preferred method for setting session metadata.
func (si *SessionInfo) SetMetadata(metadata *tools.TypedSessionMetadata) {
	si.Metadata = metadata
}

// GetMetadata returns the typed metadata.
// This method provides access to the strongly typed session metadata.
func (si *SessionInfo) GetMetadata() *tools.TypedSessionMetadata {
	return si.Metadata
}

// GetMetadataAsMap returns metadata as a map for backward compatibility.
// This method converts the typed metadata back to an untyped map.
func (si *SessionInfo) GetMetadataAsMap() map[string]interface{} {
	if si.Metadata == nil {
		return nil
	}

	result := make(map[string]interface{})

	// Add known fields
	if si.Metadata.UserID != "" {
		result["user_id"] = si.Metadata.UserID
	}
	if si.Metadata.Source != "" {
		result["source"] = si.Metadata.Source
	}
	if si.Metadata.Environment != "" {
		result["environment"] = si.Metadata.Environment
	}
	if si.Metadata.LastTool != "" {
		result["last_tool"] = si.Metadata.LastTool
	}
	if si.Metadata.ToolCount > 0 {
		result["tool_count"] = si.Metadata.ToolCount
	}
	if !si.Metadata.LastActivity.IsZero() {
		result["last_activity"] = si.Metadata.LastActivity.Format(time.RFC3339)
	}
	if len(si.Metadata.Tags) > 0 {
		result["tags"] = si.Metadata.Tags
	}

	// Add custom fields
	for k, v := range si.Metadata.CustomFields {
		result[k] = v
	}

	return result
}
