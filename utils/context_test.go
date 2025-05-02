package utils

import (
	"testing"
)

func TestGrabContentBetweenTags(t *testing.T) {
	tests := []struct {
		name        string
		content     string
		tag         string
		expected    string
		expectError bool
	}{
		{
			name:        "Simple content",
			content:     "<DOCKERFILE>Test Dockerfile Content</DOCKERFILE>",
			tag:         "DOCKERFILE",
			expected:    "Test Dockerfile Content",
			expectError: false,
		},
		{
			name:        "Multiple Tags",
			content:     "<DOCKERFILE>Test Dockerfile Content<DOCKERFILE>Another Content</DOCKERFILE>",
			tag:         "DOCKERFILE",
			expected:    "Another Content",
			expectError: false,
		},
		{
			name:        "Content with nested tags",
			content:     "<outer>This is <inner>nested</inner> content</outer>",
			tag:         "outer",
			expected:    "This is <inner>nested</inner> content",
			expectError: false,
		},
		{
			name:        "Empty content",
			content:     "<empty></empty>",
			tag:         "empty",
			expected:    "",
			expectError: false,
		},
		{
			name:        "Missing tags",
			content:     "No tags here",
			tag:         "tag",
			expected:    "",
			expectError: true,
		},
		{
			name:        "Mismatched tags",
			content:     "<start>Content</end>",
			tag:         "start",
			expected:    "",
			expectError: true,
		},
		{
			name:        "Missing closing tag",
			content:     "<MANIFEST>Content",
			tag:         "MANIFEST",
			expected:    "",
			expectError: true,
		},
		{
			name:        "Missing opening tag",
			content:     "Content</MANIFEST>",
			tag:         "MANIFEST",
			expected:    "",
			expectError: true,
		},
		{
			name:        "Extra tag",
			content:     "<MANIFEST>Some Manifest Content</MANIFEST><extra>Extra Content</extra>",
			tag:         "MANIFEST",
			expected:    "Some Manifest Content",
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := GrabContentBetweenTags(tt.content, tt.tag)

			if tt.expectError && err == nil {
				t.Errorf("expected an error but got nil")
			}
			if !tt.expectError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			if !tt.expectError && result != tt.expected {
				t.Errorf("expected: %q, got: %q", tt.expected, result)
			}
		})
	}
}
