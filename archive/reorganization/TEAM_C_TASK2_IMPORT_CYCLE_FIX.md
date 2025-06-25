# Task 2: Fix Import Cycle Issues

## Problem
After removing duplicate interfaces, several packages have import cycle issues because they need to use interfaces from pkg/mcp but pkg/mcp imports from internal packages.

## Import Cycle Analysis
```
pkg/mcp -> pkg/mcp/internal/core -> pkg/mcp/internal/transport -> pkg/mcp (CYCLE)
```

## Current Build Errors
1. **Transport packages**: http.go, stdio.go need RequestHandler, Transport interfaces
2. **Deploy strategies**: Need ProgressReporter interface
3. **Progress adapter**: Needs ProgressReporter, ProgressStage types
4. **Test files**: interfaces_test.go needs ProgressReporter

## Solution Strategy
To break import cycles while maintaining functionality:

1. **Define minimal local interfaces** in internal packages that need them
2. **Use type aliases or interface embedding** to maintain compatibility  
3. **Avoid importing pkg/mcp from any pkg/mcp/internal/** packages
4. **Keep canonical interfaces in pkg/mcp/interfaces.go** as the authoritative source

## Action Plan
1. Add minimal RequestHandler interface to transport package
2. Add minimal ProgressReporter interface to deploy_strategies package
3. Fix progress adapter with local types
4. Fix test file imports
5. Clean up unused imports
6. Verify build passes

## Expected Result
- Build passes cleanly
- No import cycles
- Interface validation still passes (0 errors)
- Functionality preserved