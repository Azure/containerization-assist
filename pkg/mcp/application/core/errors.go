package core

import "errors"

// Common errors for core services
var (
	// ErrToolNotFound indicates the requested tool was not found
	ErrToolNotFound = errors.New("tool not found")

	// ErrOrchestratorNotAvailable indicates the tool orchestrator is not available
	ErrOrchestratorNotAvailable = errors.New("tool orchestrator not available")

	// ErrInvalidServerMode indicates an invalid server mode
	ErrInvalidServerMode = errors.New("invalid server mode")

	// ErrSessionNotFound indicates the requested session was not found
	ErrSessionNotFound = errors.New("session not found")
)
