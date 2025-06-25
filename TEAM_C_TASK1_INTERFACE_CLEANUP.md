# Task 1: Fix Duplicate Interface Definitions

## Problem
Interface validation shows 7 duplicate definitions between:
- `pkg/mcp/interfaces.go` (canonical)
- `pkg/mcp/types/interfaces.go` (duplicates to remove)

## Duplicates Found
1. `Tool` interface
2. `ToolRegistry` interface  
3. `ProgressReporter` interface
4. `ToolArgs` interface
5. `RequestHandler` interface
6. `ToolResult` interface
7. `Transport` interface

## Solution
Remove duplicate interface definitions from `pkg/mcp/types/interfaces.go` while preserving:
- All non-interface types (structs, constants, etc.)
- Type aliases for compatibility
- Supporting types that don't conflict

## Action Plan
1. Remove lines 24-38: ToolArgs and Tool interfaces
2. Remove lines 62-68: ProgressReporter interface  
3. Remove lines 91-101: RequestHandler and Transport interfaces
4. Remove lines 103-108: ToolRegistry interface
5. Keep all supporting types and constants
6. Validate build passes
7. Run interface validation tool

## Expected Result
- Interface validation: 7 errors → 0 errors
- Build continues to pass
- All imports remain functional via canonical interfaces.go