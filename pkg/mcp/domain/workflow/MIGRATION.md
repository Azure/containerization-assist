# Workflow Orchestrator Migration Guide

## Overview

The workflow orchestrator architecture has been refactored to use a single base implementation with decorators, replacing the previous 3-level inheritance hierarchy.

## Changes

### Before
```
Orchestrator
    └── EventOrchestrator (inherits from Orchestrator)
            └── SagaOrchestrator (inherits from EventOrchestrator)
```

### After
```
BaseOrchestrator (single implementation)
    └── Enhanced via decorators:
        - WithEvents(base) → EventAwareOrchestrator
        - WithSaga(eventAware) → SagaAwareOrchestrator
```

## Migration Steps

### 1. Replace Orchestrator Usage

**Before:**
```go
orchestrator := workflow.NewOrchestrator(logger)
```

**After:**
```go
factory := workflow.NewStepFactory(stepProvider, optimizer, optimizedStep, logger)
baseOrchestrator := workflow.NewBaseOrchestrator(factory, progressFactory, logger, middlewares...)
```

### 2. Replace EventOrchestrator Usage

**Before:**
```go
eventOrchestrator := workflow.NewEventOrchestrator(logger, eventPublisher)
```

**After:**
```go
eventAwareOrchestrator := workflow.WithEvents(baseOrchestrator, eventPublisher)
```

### 3. Replace SagaOrchestrator Usage

**Before:**
```go
sagaOrchestrator := workflow.NewSagaOrchestrator(logger, eventPublisher, sagaCoordinator)
```

**After:**
```go
sagaAwareOrchestrator := workflow.WithSaga(eventAwareOrchestrator, sagaCoordinator, logger)
// Or with dependencies:
sagaAwareOrchestrator := workflow.WithSagaAndDependencies(
    eventAwareOrchestrator, 
    sagaCoordinator, 
    containerManager, 
    deploymentManager, 
    logger
)
```

## Removed Files

- `event_orchestrator.go` - Functionality moved to `middleware_event.go` and `decorators.go`
- `saga_orchestrator.go` - Functionality moved to `middleware_saga.go` and `decorators.go`

## Key Benefits

1. **Reduced Code Duplication**: ~1000 lines removed
2. **Better Separation of Concerns**: Each middleware handles one aspect
3. **Easier Testing**: Middleware can be tested in isolation
4. **More Flexible**: New capabilities can be added via middleware without modifying core

## Wire Configuration

Update your wire providers:

```go
// Old
workflow.ProvideOrchestrator(...)
workflow.ProvideEventOrchestrator(...)
workflow.ProvideSagaOrchestrator(...)

// New
workflow.ProvideBaseOrchestrator(...)
workflow.ProvideEventOrchestrator(...)  // Now uses decorator
workflow.ProvideSagaOrchestrator(...)   // Now uses decorator
```