package prompt

import (
	"fmt"
	"os"
	"path/filepath"
)

// Manager is an interface for managing prompt templates
type Manager interface {
	// GetTemplate retrieves a prompt template by name
	GetTemplate(name string) (string, error)

	// ListTemplates returns all available template names
	ListTemplates() ([]string, error)
}

// FileSystemManager implements Manager by loading templates from the filesystem
type FileSystemManager struct {
	templatesDir string
}

// NewFileSystemManager creates a new FileSystemManager with the given templates directory
func NewFileSystemManager(templatesDir string) *FileSystemManager {
	return &FileSystemManager{
		templatesDir: templatesDir,
	}
}

// GetTemplate retrieves a prompt template by name from the filesystem
func (m *FileSystemManager) GetTemplate(name string) (string, error) {
	filePath := filepath.Join(m.templatesDir, name)
	content, err := os.ReadFile(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to read template %s: %w", name, err)
	}
	return string(content), nil
}

// ListTemplates returns all available template names from the filesystem
func (m *FileSystemManager) ListTemplates() ([]string, error) {
	files, err := os.ReadDir(m.templatesDir)
	if err != nil {
		return nil, fmt.Errorf("failed to list templates: %w", err)
	}

	var templates []string
	for _, file := range files {
		if !file.IsDir() {
			templates = append(templates, file.Name())
		}
	}
	return templates, nil
}
