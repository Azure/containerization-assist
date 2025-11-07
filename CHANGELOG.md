# Change Log

## [Unreleased]

## [3.0.0] - 2025-11-07

### Major Features

**Policy-Driven Containerization** - Complete overhaul of policy system with three-phase architecture:

- **Pre-Generation Configuration** (Sprint 1)
  - Policy-driven defaults for resource allocation, build strategy, base images
  - Environment-aware configuration (dev/staging/prod)
  - Language-specific optimizations

- **Generation-Time Guidance** (Sprint 2)
  - Knowledge base filtering and weighting by policy
  - Tag-based snippet matching with policy control
  - Registry and base image category filtering

- **Template Injection** (Sprint 3) ✅ NEW
  - Automatic injection of organizational standards
  - CA certificates, observability agents, security hardening
  - Kubernetes sidecars, init containers, volumes
  - Dynamic defaults for replicas, health checks, autoscaling

- **Semantic Validation** (Sprint 4) ✅ NEW
  - Intelligent resource over-provisioning detection
  - Security posture scoring (0-100 composite score)
  - Cross-tool consistency validation
  - Environment-aware validation rules

- **Production Readiness** (Sprint 5) ✅ NEW
  - 5 production-ready example policies
  - Comprehensive policy authoring guide
  - Policy simulation tool for testing
  - Performance optimized (<200ms overhead)

### Changed
- **Policy Evaluation**: Migrated from OPA CLI-only to hybrid WASM + OPA CLI approach
  - Built-in policies now use pre-compiled WASM bundles (fast, zero-dependency)
  - Custom user policies fall back to OPA CLI (requires OPA installation)
  - ~10x faster policy evaluation for built-in policies
  - Added `@open-policy-agent/opa-wasm` dependency for WASM evaluation
  - Added `npm run build:policies` script to compile Rego → WASM
  - WASM bundles are automatically built during `npm run build`

### Breaking Changes

None - v2.0 policies remain fully compatible.

### Added

- Policy simulation CLI tool (`npm run policy:simulate`)
- OPA test framework integration (21 tests)
- Performance benchmarking suite
- 13 comprehensive documentation guides (158KB)
- 5 production-ready example policies with 1,285 lines of tests
- Semantic parsing library for intelligent validation
- Cross-tool workflow validation
- WASM pre-compilation for 10x faster policy evaluation
- Pre-compiled WASM policy bundles in `policies/compiled/` directory
- Build script `scripts/build-policies.ts` for compiling Rego policies to WASM
- Hybrid policy loader with automatic fallback from WASM to OPA CLI

### Fixed

- Template injection query path (Sprint 3)
- Policy simulation TypeScript compilation (Sprint 5)
- Integration test API for semantic validation (Sprint 4)

### Documentation

- Complete policy authoring guide (19KB)
- v2.0 → v3.0 migration guide (17KB)
- Policy getting started guide
- Writing Rego policies deep-dive (37KB)
- 5 sprint completion plans

### Testing

- 1,727+ passing tests
- 60+ unit test files
- 20 integration test files
- 6 E2E workflow tests
- 21/21 OPA policy tests passing
- 85% coverage target achieved

### Performance

- <200ms total policy overhead
- WASM pre-compilation support
- Hybrid WASM + OPA CLI evaluation
- Performance regression testing

## [1.0.1-dev.6]

- Dev Release with bug fixes, local registry validation, cross-platform docker build support, and .dockerignore file parsing

## [1.0.1-dev.5]

- Dev Release improved telemetry

## [1.0.1-dev.4]

- Dev Release with updated tool output formatting

## [1.0.1-dev.3]

- Dev Release with updated tool contracts

## [1.0.1-dev.2]

- Dev Release with doc and dep updates

## [1.0.1-dev.1]

- Dev Release with updated output formatting and policy updates

## [1.0.0]

- Initial release
