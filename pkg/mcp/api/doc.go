// Package api provides the canonical interface definitions for the MCP (Model Context Protocol) system.
//
// This package is the single source of truth for all public interfaces in the MCP ecosystem.
// It replaces multiple scattered interface definitions that previously caused import cycles
// and maintenance issues.
//
// # Architecture
//
// The api package follows these principles:
//
//   - Single Source of Truth: Each concept has exactly one interface definition
//   - No Implementation: This package contains only interfaces and types, no logic
//   - No Dependencies: Imports only standard library packages to avoid cycles
//   - Type Safety: Gradual migration from interface{} to generic types
//   - Backward Compatibility: Maintains compatibility during transition
//
// # Core Interfaces
//
// The package provides these primary interfaces:
//
//   - Tool: The base interface for all MCP tools
//   - Registry: Tool registration and management
//   - Orchestrator: High-level tool coordination
//   - Validator: Data validation framework
//   - SessionManager: Session state management
//   - Logger: Structured logging
//   - MetricsCollector: Metrics collection
//   - Tracer: Distributed tracing
//
// # Migration Guide
//
// To migrate from legacy interfaces:
//
//  1. Update imports:
//     // Old
//     import "github.com/Azure/container-kit/pkg/mcp/core"
//     tool := core.Tool
//
//     // New
//     import "github.com/Azure/container-kit/pkg/mcp/application/api"
//     tool := api.Tool
//
//  2. Use type aliases during transition:
//     // In your package
//     type Tool = api.Tool
//
//  3. Gradually update implementations to use new types
//
// # Future Enhancements
//
// Phase 2 will introduce generic interfaces:
//
//	type Tool[TInput, TOutput any] interface {
//	    Execute(ctx context.Context, input TInput) (TOutput, error)
//	}
//
// This will provide compile-time type safety and eliminate interface{} usage.
package api
