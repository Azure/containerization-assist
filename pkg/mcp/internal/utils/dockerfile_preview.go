package utils

import (
	"fmt"
	"strings"
)

// DockerfilePreviewOptions defines options for generating Dockerfile previews
type DockerfilePreviewOptions struct {
	MaxLines    int  `json:"max_lines"`    // Maximum number of lines to show in preview
	ShowPreview bool `json:"show_preview"` // Whether to include preview in response
}

// DockerfilePreview represents a preview of a Dockerfile with user options
type DockerfilePreview struct {
	Preview     string   `json:"preview"`                // First N lines of the Dockerfile
	TotalLines  int      `json:"total_lines"`            // Total number of lines in the Dockerfile
	Truncated   bool     `json:"truncated"`              // Whether the preview was truncated
	Options     []Option `json:"options"`                // Available user actions
	FullContent string   `json:"full_content,omitempty"` // Full content (optional)
}

// Option represents a user action option
type Option struct {
	ID          string `json:"id"`
	Label       string `json:"label"`
	Description string `json:"description"`
}

// CreateDockerfilePreview creates a preview of the Dockerfile content
func CreateDockerfilePreview(content string, opts DockerfilePreviewOptions) *DockerfilePreview {
	if opts.MaxLines <= 0 {
		opts.MaxLines = 15 // Default to 15 lines
	}

	lines := strings.Split(content, "\n")

	// Remove empty line at the end if content ends with newline
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}

	totalLines := len(lines)

	// Determine how many lines to show
	previewLines := opts.MaxLines
	if totalLines <= previewLines {
		previewLines = totalLines
	}

	// Create preview
	preview := strings.Join(lines[:previewLines], "\n")
	truncated := totalLines > opts.MaxLines

	// Add truncation indicator if needed
	if truncated {
		preview += fmt.Sprintf("\n\n... (%d more lines)", totalLines-previewLines)
	}

	// Create user options
	options := []Option{
		{
			ID:          "view_full",
			Label:       "View full Dockerfile",
			Description: "See the complete Dockerfile content",
		},
		{
			ID:          "modify",
			Label:       "Modify Dockerfile",
			Description: "Edit the Dockerfile before proceeding",
		},
		{
			ID:          "continue",
			Label:       "Continue with build",
			Description: "Proceed to build the Docker image",
		},
	}

	result := &DockerfilePreview{
		Preview:    preview,
		TotalLines: totalLines,
		Truncated:  truncated,
		Options:    options,
	}

	// Include full content if requested (for view_full option)
	if !opts.ShowPreview {
		result.FullContent = content
	}

	return result
}

// GeneratePreviewMessage creates a user-friendly message with the Dockerfile preview
func GeneratePreviewMessage(preview *DockerfilePreview, filePath string) string {
	var message strings.Builder

	message.WriteString("✅ **Dockerfile generated successfully!**\n\n")

	if filePath != "" {
		message.WriteString(fmt.Sprintf("📄 **File location:** `%s`\n\n", filePath))
	}

	message.WriteString("📝 **Dockerfile preview:**\n")
	message.WriteString("```dockerfile\n")
	message.WriteString(preview.Preview)
	message.WriteString("\n```\n\n")

	if preview.Truncated {
		message.WriteString(fmt.Sprintf("📊 **Total lines:** %d (showing first %d lines)\n\n",
			preview.TotalLines, preview.TotalLines-strings.Count(preview.Preview, "\n")))
	}

	message.WriteString("🔧 **What would you like to do next?**\n")
	for i, option := range preview.Options {
		message.WriteString(fmt.Sprintf("%d. **%s** - %s\n", i+1, option.Label, option.Description))
	}

	return message.String()
}

// FormatDockerfileResponse formats the response for the generate_dockerfile tool with preview
func FormatDockerfileResponse(content, filePath, template string, sessionID string, dryRun bool, includePreview bool) map[string]interface{} {
	response := map[string]interface{}{
		"success":    true,
		"session_id": sessionID,
		"dry_run":    dryRun,
	}

	if filePath != "" {
		response["dockerfile_path"] = filePath
	}

	if template != "" {
		response["template"] = template
	}

	// Always include full content for backward compatibility
	response["dockerfile_content"] = content

	// Add preview if requested
	if includePreview && content != "" {
		opts := DockerfilePreviewOptions{
			MaxLines:    15,
			ShowPreview: true,
		}

		preview := CreateDockerfilePreview(content, opts)
		response["dockerfile_preview"] = preview
		response["preview_message"] = GeneratePreviewMessage(preview, filePath)
	}

	return response
}
