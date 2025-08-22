// Package prompts provides hot-reload functionality test suite
package prompts

import (
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHotReloadWatcher(t *testing.T) {
	// Create a temporary directory for test templates
	tempDir, err := os.MkdirTemp("", "test-templates")
	require.NoError(t, err)
	defer func() {
		if err := os.RemoveAll(tempDir); err != nil {
			t.Logf("Failed to remove temp dir: %v", err)
		}
	}()

	// Create a logger for testing
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))

	// Create a manager with hot-reload enabled
	config := ManagerConfig{
		TemplateDir:     tempDir,
		EnableHotReload: true,
		AllowOverride:   true,
	}

	manager, err := NewManager(logger, config)
	require.NoError(t, err)
	defer manager.StopHotReload()

	// Create a test template file
	templateContent := `
id: test-template
name: Test Template
description: A test template
category: test
version: 1.0.0
template: |
  Hello {{.name}}!
parameters:
  - name: name
    type: string
    required: true
    description: The name to greet
max_tokens: 100
temperature: 0.3
`

	templatePath := filepath.Join(tempDir, "test-template.yaml")
	err = os.WriteFile(templatePath, []byte(templateContent), 0644)
	require.NoError(t, err)

	// Wait for hot-reload to detect the file
	time.Sleep(1 * time.Second)

	// Verify the template was loaded
	template, err := manager.GetTemplate("test-template")
	require.NoError(t, err)
	assert.Equal(t, "Test Template", template.Name)
	assert.Equal(t, "test", template.Category)

	// Modify the template
	modifiedContent := `
id: test-template
name: Modified Test Template
description: A modified test template
category: test
version: 1.1.0
template: |
  Hello {{.name}}, welcome!
parameters:
  - name: name
    type: string
    required: true
    description: The name to greet
max_tokens: 150
temperature: 0.5
`

	err = os.WriteFile(templatePath, []byte(modifiedContent), 0644)
	require.NoError(t, err)

	// Wait for hot-reload to detect the change
	time.Sleep(1 * time.Second)

	// Verify the template was updated
	template, err = manager.GetTemplate("test-template")
	require.NoError(t, err)
	assert.Equal(t, "Modified Test Template", template.Name)
	assert.Equal(t, "1.1.0", template.Version)
	assert.Equal(t, int32(150), template.MaxTokens)
	assert.Equal(t, float32(0.5), template.Temperature)

	// Remove the template file
	err = os.Remove(templatePath)
	require.NoError(t, err)

	// Wait for hot-reload to detect the removal
	time.Sleep(1 * time.Second)

	// Verify the template was removed (should fall back to embedded templates)
	_, err = manager.GetTemplate("test-template")
	assert.Error(t, err)
}

func TestHotReloadWatcherDisabled(t *testing.T) {
	// Create a temporary directory for test templates
	tempDir, err := os.MkdirTemp("", "test-templates")
	require.NoError(t, err)
	defer func() {
		if err := os.RemoveAll(tempDir); err != nil {
			t.Logf("Failed to remove temp dir: %v", err)
		}
	}()

	// Create a logger for testing
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))

	// Create a manager with hot-reload disabled
	config := ManagerConfig{
		TemplateDir:     tempDir,
		EnableHotReload: false,
		AllowOverride:   true,
	}

	manager, err := NewManager(logger, config)
	require.NoError(t, err)

	// Verify that no watcher was created
	assert.Nil(t, manager.watcher)

	// Create a test template file after manager creation
	templateContent := `
id: test-template
name: Test Template
description: A test template
category: test
version: 1.0.0
template: |
  Hello {{.name}}!
parameters:
  - name: name
    type: string
    required: true
    description: The name to greet
max_tokens: 100
temperature: 0.3
`

	templatePath := filepath.Join(tempDir, "test-template.yaml")
	err = os.WriteFile(templatePath, []byte(templateContent), 0644)
	require.NoError(t, err)

	// Wait a bit to ensure no hot-reload happens
	time.Sleep(1 * time.Second)

	// Verify the template was not loaded (hot-reload is disabled)
	_, err = manager.GetTemplate("test-template")
	assert.Error(t, err)
}

func TestHotReloadWatcherStart(t *testing.T) {
	// Create a temporary directory for test templates
	tempDir, err := os.MkdirTemp("", "test-templates")
	require.NoError(t, err)
	defer func() {
		if err := os.RemoveAll(tempDir); err != nil {
			t.Logf("Failed to remove temp dir: %v", err)
		}
	}()

	// Create a logger for testing
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))

	// Create a manager with hot-reload enabled
	config := ManagerConfig{
		TemplateDir:     tempDir,
		EnableHotReload: true,
		AllowOverride:   true,
	}

	manager, err := NewManager(logger, config)
	require.NoError(t, err)
	defer manager.StopHotReload()

	// Verify watcher was created
	assert.NotNil(t, manager.watcher)
}

func TestHotReloadTemplateFileDetection(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))

	// Create a dummy manager for testing
	manager := &Manager{
		templates: make(map[string]*Template),
		logger:    logger,
		config:    ManagerConfig{},
	}

	// Create a watcher for testing
	watcher := &HotReloadWatcher{
		manager: manager,
		logger:  logger,
	}

	// Test template file detection
	assert.True(t, watcher.isTemplateFile("test.yaml"))
	assert.True(t, watcher.isTemplateFile("test.yml"))
	assert.False(t, watcher.isTemplateFile("test.txt"))
	assert.False(t, watcher.isTemplateFile("test.json"))
	assert.False(t, watcher.isTemplateFile("test"))
}
