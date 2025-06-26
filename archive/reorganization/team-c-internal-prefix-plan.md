# Team C - Internal Prefix Strategy Plan

## Current Status
- Interface validation errors reduced from 7 to 4
- Remaining errors are due to local interface definitions created to avoid import cycles:
  - Transport (in pkg/mcp/interfaces.go and pkg/mcp/internal/core/interfaces.go)
  - RequestHandler (in pkg/mcp/interfaces.go and pkg/mcp/internal/core/interfaces.go)
  - ToolRegistry (in pkg/mcp/interfaces.go and pkg/mcp/internal/orchestration/interfaces.go)
  - ToolOrchestrator (in pkg/mcp/internal/orchestration/interfaces.go and pkg/mcp/internal/runtime/conversation/interfaces.go)

## Team A's Internal Prefix Strategy
According to REORG.md, Team A successfully used an "Internal" prefix for internal types to avoid naming conflicts and import cycles.

## Action Plan

### 1. Rename Internal Interfaces
- In `pkg/mcp/internal/core/interfaces.go`:
  - `Transport` → `InternalTransport`
  - `RequestHandler` → `InternalRequestHandler`
  
- In `pkg/mcp/internal/orchestration/interfaces.go`:
  - `ToolRegistry` → `InternalToolRegistry`
  - `ToolOrchestrator` → `InternalToolOrchestrator`
  
- In `pkg/mcp/internal/runtime/conversation/interfaces.go`:
  - `ToolOrchestrator` → `InternalToolOrchestrator`

### 2. Update All References
Update all files that use these local interfaces to use the new Internal-prefixed names.

### 3. Verify No Import Cycles
After renaming, ensure no import cycles are introduced.

### 4. Run Interface Validation
Re-run the validation tool to confirm 0 errors.

## Expected Outcome
- Interface validation passes with 0 errors
- CI/CD pipeline unblocked
- No import cycles
- Clear distinction between public interfaces (in pkg/mcp) and internal interfaces (with Internal prefix)