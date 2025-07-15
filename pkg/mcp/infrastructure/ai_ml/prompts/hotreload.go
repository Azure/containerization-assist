// Package prompts provides hot-reload functionality for template files
package prompts

import (
	"context"
	"log/slog"
	"path/filepath"
	"time"

	"github.com/fsnotify/fsnotify"
)

// HotReloadWatcher watches for file changes and reloads templates
type HotReloadWatcher struct {
	manager  *Manager
	watcher  *fsnotify.Watcher
	logger   *slog.Logger
	stopChan chan struct{}
	doneChan chan struct{}
}

// NewHotReloadWatcher creates a new hot-reload watcher
func NewHotReloadWatcher(manager *Manager, logger *slog.Logger) (*HotReloadWatcher, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	return &HotReloadWatcher{
		manager:  manager,
		watcher:  watcher,
		logger:   logger.With("component", "hotreload_watcher"),
		stopChan: make(chan struct{}),
		doneChan: make(chan struct{}),
	}, nil
}

// Start starts the hot-reload watcher
func (w *HotReloadWatcher) Start(ctx context.Context) error {
	// Add template directory to watch list
	if w.manager.config.TemplateDir != "" {
		if err := w.watcher.Add(w.manager.config.TemplateDir); err != nil {
			return err
		}
		w.logger.Info("Watching template directory for changes", "dir", w.manager.config.TemplateDir)
	}

	// Start watching in a goroutine
	go w.watchLoop(ctx)

	return nil
}

// Stop stops the hot-reload watcher
func (w *HotReloadWatcher) Stop() {
	close(w.stopChan)
	<-w.doneChan
	w.watcher.Close()
	w.logger.Info("Hot-reload watcher stopped")
}

// watchLoop is the main watch loop
func (w *HotReloadWatcher) watchLoop(ctx context.Context) {
	defer close(w.doneChan)

	// Debounce mechanism to avoid multiple reloads for rapid file changes
	var reloadTimer *time.Timer
	const debounceDelay = 500 * time.Millisecond

	for {
		select {
		case <-ctx.Done():
			w.logger.Debug("Watch loop stopped due to context cancellation")
			return
		case <-w.stopChan:
			w.logger.Debug("Watch loop stopped by stop signal")
			return
		case event, ok := <-w.watcher.Events:
			if !ok {
				w.logger.Debug("Watcher events channel closed")
				return
			}

			w.logger.Debug("File system event detected",
				"event", event.String(),
				"file", event.Name)

			// Only process template files
			if !w.isTemplateFile(event.Name) {
				continue
			}

			// Check if this is a relevant event
			if event.Op&fsnotify.Write == fsnotify.Write ||
				event.Op&fsnotify.Create == fsnotify.Create ||
				event.Op&fsnotify.Remove == fsnotify.Remove {

				w.logger.Info("Template file changed, scheduling reload",
					"file", event.Name,
					"operation", event.Op.String())

				// Cancel previous timer if exists
				if reloadTimer != nil {
					reloadTimer.Stop()
				}

				// Schedule reload with debounce
				reloadTimer = time.AfterFunc(debounceDelay, func() {
					w.reloadTemplates()
				})
			}

		case err, ok := <-w.watcher.Errors:
			if !ok {
				w.logger.Debug("Watcher errors channel closed")
				return
			}

			w.logger.Error("File watcher error", "error", err)
		}
	}
}

// isTemplateFile checks if a file is a template file
func (w *HotReloadWatcher) isTemplateFile(filename string) bool {
	ext := filepath.Ext(filename)
	return ext == ".yaml" || ext == ".yml"
}

// reloadTemplates reloads all templates with error handling
func (w *HotReloadWatcher) reloadTemplates() {
	w.logger.Info("Reloading templates due to file changes")

	start := time.Now()
	if err := w.manager.ReloadTemplates(); err != nil {
		w.logger.Error("Failed to reload templates", "error", err)
		return
	}

	duration := time.Since(start)
	w.logger.Info("Templates reloaded successfully",
		"duration", duration,
		"count", len(w.manager.templates))
}

// StartHotReload starts hot-reload functionality for the manager
func (m *Manager) StartHotReload(ctx context.Context) error {
	if !m.config.EnableHotReload {
		m.logger.Debug("Hot-reload is disabled")
		return nil
	}

	if m.config.TemplateDir == "" {
		m.logger.Debug("No template directory configured, skipping hot-reload")
		return nil
	}

	watcher, err := NewHotReloadWatcher(m, m.logger)
	if err != nil {
		return err
	}

	// Store watcher for cleanup
	m.watcher = watcher

	return watcher.Start(ctx)
}

// StopHotReload stops hot-reload functionality
func (m *Manager) StopHotReload() {
	if m.watcher != nil {
		m.watcher.Stop()
		m.watcher = nil
	}
}
