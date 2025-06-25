package utils

import (
	"strings"
	"testing"
)

func TestCreateDockerfilePreview(t *testing.T) {
	tests := []struct {
		name        string
		content     string
		opts        DockerfilePreviewOptions
		expectLines int
		expectTrunc bool
	}{
		{
			name:    "short dockerfile",
			content: "FROM node:18\nWORKDIR /app\nCOPY . .\nRUN npm install\nEXPOSE 3000\nCMD [\"npm\", \"start\"]",
			opts: DockerfilePreviewOptions{
				MaxLines:    15,
				ShowPreview: true,
			},
			expectLines: 6,
			expectTrunc: false,
		},
		{
			name:    "long dockerfile with truncation",
			content: strings.Repeat("RUN echo line\n", 20),
			opts: DockerfilePreviewOptions{
				MaxLines:    10,
				ShowPreview: true,
			},
			expectLines: 10,
			expectTrunc: true,
		},
		{
			name:    "default max lines",
			content: strings.Repeat("RUN echo line\n", 20),
			opts: DockerfilePreviewOptions{
				ShowPreview: true,
			},
			expectLines: 15, // Default
			expectTrunc: true,
		},
		{
			name:    "exact max lines",
			content: strings.Repeat("RUN echo line\n", 15),
			opts: DockerfilePreviewOptions{
				MaxLines:    15,
				ShowPreview: true,
			},
			expectLines: 15,
			expectTrunc: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			preview := CreateDockerfilePreview(tt.content, tt.opts)

			// Check total lines (adjust for trailing newline removal)
			lines := strings.Split(tt.content, "\n")
			if len(lines) > 0 && lines[len(lines)-1] == "" {
				lines = lines[:len(lines)-1]
			}
			expectedTotalLines := len(lines)
			if preview.TotalLines != expectedTotalLines {
				t.Errorf("Expected total lines %d, got %d", expectedTotalLines, preview.TotalLines)
			}

			// Check truncation
			if preview.Truncated != tt.expectTrunc {
				t.Errorf("Expected truncated %t, got %t", tt.expectTrunc, preview.Truncated)
			}

			// Check preview lines (approximately)
			previewLineCount := len(strings.Split(preview.Preview, "\n"))
			if tt.expectTrunc {
				// With truncation indicator, should have more lines
				if previewLineCount <= tt.expectLines {
					t.Errorf("Expected preview to have more than %d lines with truncation indicator, got %d", tt.expectLines, previewLineCount)
				}
			} else {
				// Without truncation, should match exactly
				if previewLineCount != tt.expectLines {
					t.Errorf("Expected preview to have %d lines, got %d", tt.expectLines, previewLineCount)
				}
			}

			// Check options are present
			if len(preview.Options) != 3 {
				t.Errorf("Expected 3 options, got %d", len(preview.Options))
			}

			expectedOptions := []string{"view_full", "modify", "continue"}
			for i, expected := range expectedOptions {
				if preview.Options[i].ID != expected {
					t.Errorf("Expected option ID %s, got %s", expected, preview.Options[i].ID)
				}
			}
		})
	}
}

func TestGeneratePreviewMessage(t *testing.T) {
	preview := &DockerfilePreview{
		Preview:    "FROM node:18\nWORKDIR /app\nCOPY . .\n\n... (5 more lines)",
		TotalLines: 8,
		Truncated:  true,
		Options: []Option{
			{ID: "view_full", Label: "View full Dockerfile", Description: "See the complete Dockerfile content"},
			{ID: "modify", Label: "Modify Dockerfile", Description: "Edit the Dockerfile before proceeding"},
			{ID: "continue", Label: "Continue with build", Description: "Proceed to build the Docker image"},
		},
	}

	filePath := "/app/Dockerfile"
	message := GeneratePreviewMessage(preview, filePath)

	// Check message contains key elements
	if !strings.Contains(message, "✅ **Dockerfile generated successfully!**") {
		t.Error("Message should contain success indicator")
	}

	if !strings.Contains(message, filePath) {
		t.Error("Message should contain file path")
	}

	if !strings.Contains(message, "```dockerfile") {
		t.Error("Message should contain dockerfile code block")
	}

	if !strings.Contains(message, preview.Preview) {
		t.Error("Message should contain preview content")
	}

	if !strings.Contains(message, "🔧 **What would you like to do next?**") {
		t.Error("Message should contain next actions")
	}

	// Check all options are listed
	for i, option := range preview.Options {
		expectedText := option.Label
		if !strings.Contains(message, expectedText) {
			t.Errorf("Message should contain option %d label: %s", i+1, expectedText)
		}
	}
}

func TestFormatDockerfileResponse(t *testing.T) {
	content := "FROM node:18\nWORKDIR /app\nCOPY . .\nRUN npm install\nEXPOSE 3000\nCMD [\"npm\", \"start\"]"
	filePath := "/app/Dockerfile"
	template := "dockerfile-node"
	sessionID := "test-session"

	tests := []struct {
		name           string
		includePreview bool
		dryRun         bool
	}{
		{
			name:           "with preview",
			includePreview: true,
			dryRun:         false,
		},
		{
			name:           "without preview",
			includePreview: false,
			dryRun:         false,
		},
		{
			name:           "dry run with preview",
			includePreview: true,
			dryRun:         true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			response := FormatDockerfileResponse(content, filePath, template, sessionID, tt.dryRun, tt.includePreview)

			// Check required fields
			if response["success"] != true {
				t.Error("Response should indicate success")
			}

			if response["session_id"] != sessionID {
				t.Errorf("Expected session_id %s, got %v", sessionID, response["session_id"])
			}

			if response["dry_run"] != tt.dryRun {
				t.Errorf("Expected dry_run %t, got %v", tt.dryRun, response["dry_run"])
			}

			if response["dockerfile_path"] != filePath {
				t.Errorf("Expected dockerfile_path %s, got %v", filePath, response["dockerfile_path"])
			}

			if response["dockerfile_content"] != content {
				t.Errorf("Expected dockerfile_content to match")
			}

			if response["template"] != template {
				t.Errorf("Expected template %s, got %v", template, response["template"])
			}

			// Check preview fields
			if tt.includePreview {
				if response["dockerfile_preview"] == nil {
					t.Error("Response should include dockerfile_preview when requested")
				}

				if response["preview_message"] == nil {
					t.Error("Response should include preview_message when requested")
				}

				// Verify preview structure
				preview, ok := response["dockerfile_preview"].(*DockerfilePreview)
				if !ok {
					t.Error("dockerfile_preview should be of type *DockerfilePreview")
				} else {
					if preview.TotalLines != 6 {
						t.Errorf("Expected 6 total lines, got %d", preview.TotalLines)
					}
					if len(preview.Options) != 3 {
						t.Errorf("Expected 3 options, got %d", len(preview.Options))
					}
				}
			} else {
				if response["dockerfile_preview"] != nil {
					t.Error("Response should not include dockerfile_preview when not requested")
				}

				if response["preview_message"] != nil {
					t.Error("Response should not include preview_message when not requested")
				}
			}
		})
	}
}

func TestDockerfilePreviewOptions(t *testing.T) {
	content := "FROM alpine\nRUN echo hello"

	// Test default options
	opts := DockerfilePreviewOptions{}
	preview := CreateDockerfilePreview(content, opts)

	if opts.MaxLines <= 0 && preview.TotalLines <= 15 {
		// Should not be truncated with default max lines
		if preview.Truncated {
			t.Error("Short content should not be truncated with default options")
		}
	}

	// Test explicit options
	opts = DockerfilePreviewOptions{
		MaxLines:    1,
		ShowPreview: true,
	}
	preview = CreateDockerfilePreview(content, opts)

	if !preview.Truncated {
		t.Error("Content should be truncated with MaxLines=1")
	}

	// Check that full content is included when ShowPreview is false
	opts = DockerfilePreviewOptions{
		MaxLines:    1,
		ShowPreview: false,
	}
	preview = CreateDockerfilePreview(content, opts)

	if preview.FullContent != content {
		t.Error("FullContent should be set when ShowPreview is false")
	}
}
