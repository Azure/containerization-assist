// Package events provides domain event definitions and publishing for Containerization Assist MCP.
package events

// DomainEvent represents a domain event that occurred within the system.
// Events are used to decouple components and enable reactive behaviors.
type DomainEvent interface {
	// EventType returns the type name of this event
	EventType() string
}

// Note: Concrete event types (WorkflowStartedEvent, WorkflowCompletedEvent, etc.)
// have been removed as they were unused. If events are needed in the future,
// create new event types that implement the DomainEvent interface.
