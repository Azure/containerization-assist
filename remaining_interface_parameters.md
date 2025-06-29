# Remaining map[string]interface{} Parameters Documentation

## Overview
During the Day 1 interface consolidation work, I found multiple structs that still use `map[string]interface{}` for their Parameters field. These are **different** from the ToolMetadata struct we consolidated and appear to serve different purposes in the codebase.

## Occurrences Found

### 1. Test Files
- `pkg/mcp/internal/server/unified_server_test.go` - Test data using old format
- `pkg/mcp/internal/orchestration/types_test.go` - Test data using old format

### 2. Error Handling Structures
Location: `pkg/mcp/internal/orchestration/error_types.go`
- `ErrorContext` struct - Parameters field
- `ErrorAction` struct - Parameters field
- `RecoveryStrategy` struct - Parameters field

Location: `pkg/mcp/internal/orchestration/error_redirection.go`
- Creating ErrorAction with empty map[string]interface{} Parameters

Location: `pkg/mcp/internal/orchestration/error_router.go`
- Setting action.Parameters to empty map[string]interface{}

Location: `pkg/mcp/internal/orchestration/error_recovery.go`
- Multiple RecoveryStrategy instances with Parameters containing various configs

### 3. Tool Input/Execution Structures
Location: `pkg/mcp/internal/orchestration/tool_types.go`
- `ToolInput` struct - Parameters field (this is for runtime tool execution, not metadata)

### 4. Runtime Analysis
Location: `pkg/mcp/internal/runtime/analyzer.go`
- `AnalysisContext` struct - Parameters field

### 5. Retry/Fix Mechanisms
Location: `pkg/mcp/internal/retry/fix_providers.go`
- Multiple FixAttempt structs with Parameters containing fix configurations

Location: `pkg/mcp/internal/retry/coordinator.go`
- `FixAttempt` struct - Parameters field

### 6. Runtime Validation
Location: `pkg/mcp/internal/runtime/validator.go`
- `ValidationContext` struct - Parameters field

### 7. Conversation State
Location: `pkg/mcp/internal/runtime/conversation/conversation_state.go`
- `ConversationContext` struct - Parameters field

### 8. Helper Functions
Location: `pkg/mcp/internal/server/unified_server.go`
- `convertParametersMapToString()` function - still exists but may be unused after our changes

## Analysis

These occurrences fall into distinct categories:

1. **Runtime Execution Parameters**: These are for passing dynamic arguments during tool execution, not static metadata
2. **Error Handling Context**: Used for error recovery and retry mechanisms with flexible data
3. **Test Data**: Test files that may need updating by Workstream D
4. **Conversation/Analysis Context**: Used for passing contextual information through the system

## Recommendation

These uses of `map[string]interface{}` appear to be **intentionally flexible** for runtime data passing, unlike the ToolMetadata which is static structural information. They likely should NOT be converted to `map[string]string` as they need to handle various data types at runtime.

The only change needed is in the test files where ToolMetadata is being tested with the old format - this is Workstream D's responsibility according to the project plan.