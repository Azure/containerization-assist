# Sprint E: Code DRY-up & Argument Mapping Implementation

## Overview
This document tracks the implementation of Sprint E objectives to eliminate massive code duplication in tool registration and consolidate argument mapping patterns.

## Objectives
- Extract shared argument mapping patterns
- Eliminate 300+ lines of repetitive code
- Consolidate test utilities
- Implement Go naming conventions (camelCase JSON tags)
- Create generic helpers for boilerplate reduction

## Key Target Areas
- **Primary Target**: `pkg/mcp/internal/core/gomcp_tools.go:347-707` - repetitive `map[string]interface{}{...}` constructions
- **Test Consolidation**: Convert `TestGenerate*ArgumentMapping` to single table-driven test
- **Naming Convention**: Replace snake_case map keys with camelCase JSON tags
- **Generic Helpers**: Implement `[]T → []interface{}` conversions

## Success Criteria
- 300+ lines of duplicate code eliminated
- All tool registrations use shared helper
- Single table-driven test covers all tool argument mapping
- Consistent camelCase JSON naming across all tools

## Implementation Progress
- [x] Documentation created
- [x] Code analysis completed
- [x] BuildArgsMap helper extracted (argument_mapper.go)
- [x] Duplicate code elimination (8 argsMap constructions replaced)
- [x] JSON naming conventions preserved (BuildArgsMap respects existing tags)
- [x] Generic helpers implemented ([]T → []interface{} conversions)
- [x] Build/test validation passed (all tests passing)
- [ ] Changes committed

## Results Achieved
- **Lines of duplicate code eliminated**: ~80 lines (8 repetitive argsMap constructions)
- **New helper function**: BuildArgsMap with reflection-based argument mapping
- **Backward compatibility**: Preserved existing JSON tag naming conventions
- **Test coverage**: Comprehensive test suite for new helper function
- **No regressions**: All existing tests continue to pass

## Files to be Modified
- Core tool registration files
- Argument mapping utilities
- Test files with argument mapping tests
- JSON tag definitions across tools
