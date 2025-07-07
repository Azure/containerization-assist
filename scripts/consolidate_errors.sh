#!/bin/bash

# Script to consolidate error packages in pkg/mcp

echo "=== Error Package Consolidation Plan ==="
echo
echo "Current structure:"
echo "- pkg/mcp/errors/ (public errors)"
echo "  - errors.go (261 lines) - MCPError type"
echo "  - rich_helpers.go (68 lines) - Helper functions"
echo "  - rich/ (1,322 lines total)"
echo "    - builder.go (249 lines)"
echo "    - constructors.go (424 lines)"
echo "    - generic.go (397 lines)"
echo "    - types.go (252 lines)"
echo
echo "- pkg/mcp/internal/errors/ (internal errors)"
echo "  - core_error.go (425 lines)"
echo "  - error_validation.go (250 lines)"
echo "  - orchestration.go (196 lines)"
echo "  - runtime.go (337 lines)"
echo "  - tool.go (139 lines)"
echo "  - types.go (450 lines) - Duplicate RichError!"
echo
echo "- pkg/mcp/types/config/errors.go (32 lines)"
echo
echo "Total: 14 files, 3,481 lines"
echo
echo "=== Proposed Consolidated Structure ==="
echo
echo "pkg/mcp/errors/"
echo "├── errors.go (300 lines) - Core error types and MCPError"
echo "├── rich.go (400 lines) - RichError type and builder"
echo "├── constructors.go (300 lines) - Error constructors"
echo "├── validation.go (200 lines) - Validation errors"
echo "├── runtime.go (250 lines) - Runtime errors"
echo "├── tool.go (150 lines) - Tool errors"
echo "└── helpers.go (100 lines) - Helper functions"
echo
echo "Total: 7 files, ~1,700 lines (51% reduction)"
echo
echo "=== Actions to take: ==="
echo "1. Merge rich/* into rich.go"
echo "2. Move internal/errors/* to pkg/mcp/errors/*"
echo "3. Remove duplicate RichError definitions"
echo "4. Consolidate error constructors"
echo "5. Remove generic.go complexity"
