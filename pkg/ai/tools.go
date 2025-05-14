package ai

import (
	"encoding/json"

	"github.com/Azure/azure-sdk-for-go/sdk/ai/azopenai"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
)

// Helper to JSON‑marshal a schema
func mkSchema(v any) []byte {
	b, _ := json.Marshal(v)
	return b
}

// CreateReadFileTool creates a tool definition for reading files
func CreateReadFileTool() azopenai.ChatCompletionsFunctionToolDefinition {
	return azopenai.ChatCompletionsFunctionToolDefinition{
		Type: to.Ptr("function"),
		Function: &azopenai.ChatCompletionsFunctionToolDefinitionFunction{
			Name:        to.Ptr("read_file"),
			Description: to.Ptr("Read file contents from the repository"),
			Parameters: mkSchema(map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"filePath": map[string]interface{}{
						"type":        "string",
						"description": "Path to file, relative to repo root",
					},
				},
				"required": []string{"filePath"},
			}),
		},
	}
}

// CreateListDirectoryTool creates a tool definition for listing directory contents
func CreateListDirectoryTool() azopenai.ChatCompletionsFunctionToolDefinition {
	return azopenai.ChatCompletionsFunctionToolDefinition{
		Type: to.Ptr("function"),
		Function: &azopenai.ChatCompletionsFunctionToolDefinitionFunction{
			Name:        to.Ptr("list_directory"),
			Description: to.Ptr("List files in a directory"),
			Parameters: mkSchema(map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"dirPath": map[string]interface{}{
						"type":        "string",
						"description": "Directory path, relative to repo root",
					},
				},
				"required": []string{"dirPath"},
			}),
		},
	}
}

// CreateFileExistsTool creates a tool definition for checking if a file exists
func CreateFileExistsTool() azopenai.ChatCompletionsFunctionToolDefinition {
	return azopenai.ChatCompletionsFunctionToolDefinition{
		Type: to.Ptr("function"),
		Function: &azopenai.ChatCompletionsFunctionToolDefinitionFunction{
			Name:        to.Ptr("file_exists"),
			Description: to.Ptr("Check if a file exists"),
			Parameters: mkSchema(map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"filePath": map[string]interface{}{
						"type":        "string",
						"description": "File path, relative to repo root",
					},
				},
				"required": []string{"filePath"},
			}),
		},
	}
}

// GetFileSystemTools returns all file system related tools
func GetFileSystemTools() []azopenai.ChatCompletionsToolDefinitionClassification {
	readTool := CreateReadFileTool()
	listTool := CreateListDirectoryTool()
	existsTool := CreateFileExistsTool()

	return []azopenai.ChatCompletionsToolDefinitionClassification{
		&readTool, &listTool, &existsTool,
	}
}
