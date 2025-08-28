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

// MockWorkflowOrchestrator implements workflow.WorkflowOrchestrator for testing
type MockWorkflowOrchestrator struct {
	mock.Mock
}