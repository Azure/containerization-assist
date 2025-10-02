# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- Comprehensive structured response documentation in `docs/tool-capabilities.md` with examples for tool developers
- Type safety guard tests for tool registration to prevent invalid tool objects from being registered
- Behavioural CLI tests using Commander directly instead of string-grep source code tests
- Manual test exclusion in Jest configuration for `bootstrap-manual-test.ts`

### Changed
- CLI tests now exercise Commander's API directly for more reliable validation of argument parsing
- Test suite improved with better coverage of type safety guarantees
- Manual integration tests now clearly marked and excluded from automated test runs

### Technical Improvements
- **MCP Response Formatting**: Tools can now return structured data with optional `summary` field for human-readable output
  - Objects with `summary` field emit two text blocks (summary + data)
  - Objects without `summary` emit single JSON text block
  - Primitive values emit single text block
  - See `docs/tool-capabilities.md` for examples
- **Type Safety**: `widenToolType` helper in `src/app/index.ts` safely handles tool registration with proper documentation
- **Test Quality**: Replaced brittle string-grep tests with behavioural tests that validate actual CLI behavior

### Developer Experience
- Tool developers can now use the `summary` field pattern for better user-facing responses
- Tests are more maintainable and reliable with behavioural approach
- Type safety is enforced at registration time with compile-time and runtime checks

## [1.5.0] - Previous Release
See git history for earlier changes.
