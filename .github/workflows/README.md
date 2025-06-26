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

| Workflow | Purpose | Status |
|----------|---------|---------|
| `unit-test.yml` | Unit tests for core and MCP packages | ✅ Active |
| `mcp-integration-tests.yml` | MCP integration tests with Kind | ✅ Active |
| `build-tags-matrix.yml` | Build tag validation matrix | ✅ Active |
| `reusable-go-build.yml` | **Reusable build workflow** | 🆕 **New** |
| `example-reusable-usage.yml` | Examples of reusable workflow usage | 📝 Example |

## Next Steps

1. **Migrate existing workflows** to use `reusable-go-build.yml`
2. **Update documentation** when patterns change
3. **Add more reusable workflows** for Docker, security scanning, etc.
4. **Pin action versions** to commit SHAs for security
