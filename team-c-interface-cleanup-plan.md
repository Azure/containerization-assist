# Team C Interface Cleanup Plan

## Current Status
- Interface validation shows 5 errors (down from 7)
- Duplicate interfaces exist between pkg/mcp/interfaces.go and pkg/mcp/types/interfaces.go

## Interfaces to Remove from types/interfaces.go

1. **Transport** (lines 62-75)
2. **RequestHandler** (lines 77-80) 
3. **Tool** (lines 82-87)
4. **ToolOrchestrator** (lines 101-106)

## Additional Cleanup
- ToolRegistry duplicate in internal/orchestration/interfaces.go

## Action Plan

1. Remove duplicate interface definitions from types/interfaces.go
2. Keep only non-interface types (structs, type aliases) in types/interfaces.go
3. Update any remaining references to use the canonical definitions
4. Run validation to confirm 0 errors