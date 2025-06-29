# GitHub Actions Workflows

This directory contains GitHub Actions workflows for Container Kit CI/CD.

## Reusable Workflow: `reusable-go-build.yml`

The `reusable-go-build.yml` workflow provides a standardized way to build, test, and lint Go packages across different configurations. It reduces duplication and ensures consistency across all CI jobs.

### Features

- ✅ **Go build with optional build tags**
- ✅ **Unit tests with race detector**
- ✅ **golangci-lint integration**
- ✅ **Binary building with artifact upload**
- ✅ **Coverage reporting with Codecov upload**
- ✅ **Configurable Go version and packages**
- ✅ **Cross-platform support**
- ✅ **Build summary with outputs**

### Usage

```yaml
jobs:
  my-build:
    name: My Build Job
    uses: ./.github/workflows/reusable-go-build.yml
    with:
      go-version: '1.24'           # Go version (default: 1.24)
      build-tags: 'mcp'           # Build tags (default: '')
      packages: './pkg/mcp/...'   # Packages to build (default: './...')
      run-tests: true             # Run tests (default: true)
      run-race-tests: true        # Run race tests (default: true)
      run-lint: true              # Run linting (default: false)
      build-binary: true          # Build binary (default: false)
      binary-output: 'my-app'     # Binary name (default: '')
      coverage: true              # Generate coverage (default: false)
```

### Input Parameters

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `go-version` | string | `'1.24'` | Go version to use |
| `build-tags` | string | `''` | Build tags (comma-separated) |
| `packages` | string | `'./...'` | Packages to build/test (space-separated) |
| `enable-cache` | boolean | `true` | Enable Go modules cache |
| `run-tests` | boolean | `true` | Run tests |
| `run-race-tests` | boolean | `true` | Run tests with race detector |
| `run-lint` | boolean | `false` | Run golangci-lint |
| `lint-args` | string | `'--timeout=5m'` | Additional lint arguments |
| `build-binary` | boolean | `false` | Build binary |
| `binary-output` | string | `''` | Output path for binary |
| `binary-main` | string | `'./main.go'` | Main package for binary |
| `coverage` | boolean | `false` | Generate coverage report |
| `upload-coverage` | boolean | `false` | Upload coverage to Codecov |
| `coverage-flags` | string | `''` | Codecov flags |
| `runner-os` | string | `'ubuntu-latest'` | Runner OS |

### Outputs

| Output | Type | Description |
|--------|------|-------------|
| `build-success` | boolean | Whether build succeeded |
| `test-success` | boolean | Whether tests succeeded |
| `coverage-percentage` | string | Test coverage percentage |

### Examples

#### 1. Basic MCP Build

```yaml
mcp-build:
  uses: ./.github/workflows/reusable-go-build.yml
  with:
    build-tags: 'mcp'
    packages: './pkg/mcp/...'
    run-lint: true
    coverage: true
```

#### 2. CLI Build with Binary

```yaml
cli-build:
  uses: ./.github/workflows/reusable-go-build.yml
  with:
    build-tags: 'cli'
    build-binary: true
    binary-output: 'container-kit'
    binary-main: './main.go'
```

#### 3. Cross-Platform Matrix

```yaml
cross-platform:
  strategy:
    matrix:
      os: [ubuntu-latest, windows-latest, macos-latest]
  uses: ./.github/workflows/reusable-go-build.yml
  with:
    runner-os: ${{ matrix.os }}
    build-binary: true
    binary-output: 'app-${{ matrix.os }}'
```

#### 4. Using Outputs

```yaml
build:
  uses: ./.github/workflows/reusable-go-build.yml
  with:
    coverage: true

summary:
  needs: build
  runs-on: ubuntu-latest
  steps:
    - name: Check results
      run: |
        echo "Build: ${{ needs.build.outputs.build-success }}"
        echo "Tests: ${{ needs.build.outputs.test-success }}"
        echo "Coverage: ${{ needs.build.outputs.coverage-percentage }}%"
```

### Migration Guide

To migrate existing workflows to use the reusable workflow:

#### Before (Duplicated Code)

```yaml
jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      - name: Set up Go
        uses: actions/setup-go@v4
        with:
          go-version: '1.24'
      - name: Cache Go modules
        uses: actions/cache@v3
        with:
          path: ~/go/pkg/mod
          key: ${{ runner.os }}-go-${{ hashFiles('**/go.sum') }}
      - name: Download dependencies
        run: go mod download
      - name: Test MCP packages
        run: go test -tags=mcp ./pkg/mcp/...
      - name: Test with race detector
        run: go test -race -tags=mcp ./pkg/mcp/...
```

#### After (Reusable Workflow)

```yaml
jobs:
  test:
    uses: ./.github/workflows/reusable-go-build.yml
    with:
      build-tags: 'mcp'
      packages: './pkg/mcp/...'
      run-race-tests: true
```

### Benefits

1. **Reduced Duplication**: Eliminates ~50-100 lines of repeated YAML per workflow
2. **Consistency**: Ensures all builds use the same patterns and best practices
3. **Maintainability**: Changes to build logic only need to be made in one place
4. **Flexibility**: Configurable inputs support different use cases
5. **Observability**: Standardized outputs and summaries across all jobs
6. **Artifacts**: Automatic binary artifact uploads when building binaries

## Current Workflows

### **Primary Workflows**
| Workflow | Purpose | Triggers | Status |
|----------|---------|----------|---------|
| `unit-test.yml` | Unit tests for all packages + binary builds | PR | ✅ Active |
| `code-quality.yml` | **Consolidated linting & quality checks** | PR, Daily, Main push | 🆕 **New** |
| `quality-gates.yml` | Comprehensive quality metrics & gates | PR, Main push | ✅ Active |
| `security-scan.yml` | Security scanning (Trivy, GitLeaks) | PR, Main push | ✅ Active |
| `core-coverage-enforcement.yml` | Core package coverage requirements | PR | ✅ Active |
| `ci-status-consolidator.yml` | **Aggregates CI results into single comment** | After other workflows | 🆕 **New** |
| `adapter-elimination-check.yml` | **Verifies adapter elimination** | PR, Main push | 🆕 **New** |
| `architecture-metrics.yml` | **Tracks architecture quality metrics** | PR, Main push | 🆕 **New** |

### **Integration & Release**
| Workflow | Purpose | Triggers | Status |
|----------|---------|----------|---------|
| `mcp-integration-tests.yml` | MCP integration tests with Kind | PR | ✅ Active |
| `integration-test.yml` | Large-scale external repo testing | PR | ✅ Active |
| `release.yml` | Release management with GoReleaser | Manual | ✅ Active |
| `schema-export.yml` | MCP tool schema generation | Manual | ✅ Active |

### **Infrastructure & Examples**
| Workflow | Purpose | Status |
|----------|---------|---------|
| `reusable-go-build.yml` | **Reusable build workflow** | 🔧 **Infrastructure** |
| `example-reusable-usage.yml` | Examples of reusable workflow usage | 📝 Example |
| `coverage-ratchet.yml` | Global coverage tracking | ✅ Active |

### **Recently Consolidated** 🎯
| Replaced Workflows | Replaced By | Lines Saved |
|-------------------|-------------|-------------|
| `build-tags-matrix.yml` | Extended `unit-test.yml` | ~400 lines |
| `lint-dashboard.yml` + `lint-strict.yml` + `lint-with-budget.yml` | `code-quality.yml` | ~300 lines |

**Total Reduction**: 16 → 12 workflows (25% fewer files, 700+ lines saved)

## CI Status Consolidation

To reduce PR comment noise, we use a consolidated CI status approach:

### Individual Workflows
- **security-scan.yml** - Runs security scans, uploads artifacts
- **quality-gates.yml** - Runs quality checks, uploads artifacts
- **core-coverage-enforcement.yml** - Runs coverage checks, posts coverage comment (kept separate due to detailed reporting needs)
- **lint-dashboard.yml** - Runs lint checks, uploads artifacts

### Consolidation
- **ci-status-consolidator.yml** - Triggered after individual workflows complete
- Downloads artifacts from all related workflows for the PR
- Creates/updates a single consolidated CI status comment
- Provides overview of all CI results in one place

### Benefits
- Single CI status comment per PR instead of 4+ separate comments
- Clear overview of all CI results
- Automatically updates as workflows complete
- Maintains detailed individual workflow logs and artifacts

### Coverage Exception
The core coverage workflow retains its individual PR comment because:
- It provides detailed per-package coverage breakdown
- Users need granular coverage information for decision making
- The detailed table format doesn't fit well in the consolidated summary

## Key Improvements Made

### **Phase 1 Consolidation Complete** ✅
1. **Deleted `build-tags-matrix.yml`** - 400+ lines of duplicate build logic eliminated
2. **Consolidated 3 linting workflows** into single `code-quality.yml`
3. **Standardized Go versions** to '1.24' across all workflows
4. **Implemented CI comment consolidation** to reduce PR noise

### **Benefits Achieved**
- **25% fewer workflow files** (16 → 12)
- **700+ lines of code eliminated**
- **Consistent Go version management**
- **Single CI status comment per PR**
- **Easier maintenance and updates**

## Architecture Quality Enforcement

### **Adapter Elimination Checks**

The CI pipeline now enforces architecture quality standards from the adapter elimination project:

#### **Canary Validation Checks**
Added to `ci-pipeline.yml` canary phase:
- **No Adapter Files**: Ensures all adapter patterns have been eliminated
- **No Wrapper Files**: Verifies wrapper consolidation (except `docker_operation.go`)

#### **Dedicated Architecture Workflows**

1. **`adapter-elimination-check.yml`**
   - Verifies zero adapter files exist
   - Confirms wrapper consolidation
   - Checks for import cycles
   - Reports interface unification progress

2. **`architecture-metrics.yml`**
   - Tracks architecture metrics over time
   - Posts PR comments with quality indicators
   - Measures build performance
   - Reports on simplification progress

### **Metrics Tracked**

| Metric | Target | Enforcement |
|--------|--------|-------------|
| Adapter Files | 0 | ✅ Hard fail in CI |
| Wrapper Files | 0 | ✅ Hard fail in CI |
| Import Cycles | 0 | ✅ Build verification |
| Tool Interfaces | 1 | ⚠️ Tracked, not enforced |
| Build Time | <2s | 📊 Tracked |
| Lines of Code | Decreasing | 📊 Tracked |

## Next Steps

### **Phase 2 Opportunities** (Future improvements)
1. **Merge integration test workflows** - Combine `integration-test.yml` + `mcp-integration-tests.yml`
2. **Consolidate coverage workflows** - Merge `core-coverage-enforcement.yml` + `coverage-ratchet.yml`
3. **Extract common composite actions** - For golangci-lint installation, error budget checking
4. **Create reusable security workflow** - Extract security scanning patterns
5. **Add interface unification enforcement** - When complete (currently at 31, target 1)

### **Maintenance**
1. **Monitor new consolidation opportunities** as workflows evolve
2. **Pin action versions** to commit SHAs for security
3. **Update documentation** when patterns change
4. **Track architecture metrics** to ensure continued simplification
