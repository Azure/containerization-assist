// Package service_test provides unit tests for the service layer dependency providers.
// These tests verify the simplified dependency injection approach.
package service

import (
	"github.com/stretchr/testify/mock"
)

// MockEventPublisher implements domainevents.Publisher for testing
type MockEventPublisher struct {
	mock.Mock
}

// MockEventPublisher methods removed as dead code

// MockWorkflowOrchestrator implements workflow.WorkflowOrchestrator for testing
type MockWorkflowOrchestrator struct {
	mock.Mock
}

// MockWorkflowOrchestrator.Execute method removed as dead code

// TestEventOrchestratorAdapter_PublishWorkflowEvent was removed
// The eventOrchestratorAdapter was part of over-engineered patterns that were simplified
// Event publishing is now handled directly through the simplified architecture
