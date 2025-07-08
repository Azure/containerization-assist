# Container Kit MCP Quality Standards

## Automated Quality Gates

All code changes must pass the following automated quality gates:

### 1. Interface Count Limit
- **Standard**: ≤50 interfaces total across pkg/mcp/
- **Current**: Monitored by `scripts/interface-counter`
- **Enforcement**: CI monitoring (warning mode during refactoring)
- **Target**: Achieve ≤50 interfaces through refactoring efforts

### 2. File Size Limit
- **Standard**: ≤800 lines per Go file
- **Rationale**: Maintainability and readability
- **Enforcement**: CI monitoring (warning mode during refactoring)
- **Current Violations**: 6 files exceed limit (largest: 1840 lines)

### 3. Function Complexity
- **Standard**: ≤20 cyclomatic complexity per function
- **Rationale**: Testability and maintainability
- **Enforcement**: CI monitoring (warning mode during refactoring)
- **Current Violations**: 7 functions exceed limit

### 4. Test Coverage
- **Standard**: Minimum coverage per package (varies by package)
- **API packages**: 30% minimum coverage (target: 70%)
- **Core packages**: 35% minimum coverage (target: 75%)
- **Tool packages**: 20% minimum coverage (target: 60%)
- **Enforcement**: CI monitoring with gradual threshold increases

### 5. Architecture Boundaries
- **Standard**: Zero violations of layer boundaries
- **Import depth**: ≤3 levels maximum
- **Circular dependencies**: Zero tolerance
- **Enforcement**: CI monitoring with architecture compliance tests

### 6. Code Quality
- **Linting**: Must pass golangci-lint with project configuration
- **Formatting**: Must pass gofmt and goimports
- **Build**: Must compile without errors or warnings

## Quality Gate Implementation

### Current Status: Monitoring Mode
During the refactoring process (EPSILON workstream), quality gates are in **monitoring mode**:
- All metrics are tracked and reported
- Violations are logged as warnings, not failures
- CI provides visibility into quality trends
- Enforcement will be strengthened post-refactoring

### CI Integration
- **GitHub Actions**: Comprehensive quality workflow with detailed reporting
- **Coverage Reports**: Automated coverage analysis with artifact upload
- **Performance Monitoring**: Benchmark tracking with regression detection
- **Quality Metrics**: Interface count, file size, complexity tracking

### Local Development
```bash
# Run all quality gates locally
scripts/quality_gates.sh

# Run individual checks
scripts/interface-counter pkg/mcp/
scripts/check_file_size.sh
scripts/complexity-checker pkg/mcp/
scripts/coverage.sh
scripts/regression_test.sh
```

### Quality Tools
1. **Interface Counter**: `tools/interface-counter` - Tracks interface proliferation
2. **File Size Checker**: `scripts/check_file_size.sh` - Enforces file size limits
3. **Complexity Analyzer**: `tools/complexity-checker` - Measures cyclomatic complexity
4. **Coverage Tracker**: `scripts/coverage.sh` - Monitors test coverage
5. **Performance Monitor**: `scripts/regression_test.sh` - Detects performance regressions
6. **Architecture Validator**: Architecture compliance tests

## Quality Improvement Strategy

### Phase 1: Infrastructure (EPSILON) - Current
- ✅ Quality gate infrastructure implemented
- ✅ Monitoring and measurement tools deployed
- ✅ CI integration with comprehensive reporting
- ✅ Baseline metrics established

### Phase 2: Refactoring Support (ALPHA, BETA, GAMMA, DELTA)
- 🔄 Quality gates provide visibility during refactoring
- 🔄 Metrics track improvement progress
- 🔄 Prevent new quality debt accumulation
- 🔄 Support architectural cleanup efforts

### Phase 3: Enforcement (Post-Refactoring)
- 🎯 Gradually strengthen quality thresholds
- 🎯 Convert warnings to blocking failures
- 🎯 Achieve target metrics (≤50 interfaces, 0 violations)
- 🎯 Establish continuous quality improvement

## Current Baseline Metrics

### Interface Count
- **Current**: 149 interfaces
- **Target**: ≤50 interfaces
- **Reduction Needed**: 99 interfaces (66% reduction)

### File Size Violations
- **Current**: 6 files exceed 800 lines
- **Largest**: 1840 lines (pkg/mcp/domain/containerization/scan/tools.go)
- **Target**: 0 files exceed limit

### Function Complexity
- **Current**: 7 functions exceed complexity limit
- **Highest**: 28 complexity (ValidateTaggedStruct)
- **Target**: 0 functions exceed limit

### Test Coverage
- **Current**: Variable across packages
- **Range**: 0% to 77.2% per package
- **Target**: +5% improvement per package

## Long-term Quality Benefits

### Developer Experience
- **Faster Onboarding**: Clear quality standards and automated feedback
- **Reduced Debugging**: Higher test coverage and code quality
- **Consistent Standards**: Automated enforcement reduces variation
- **Quality Confidence**: Monitoring prevents quality regressions

### Maintainability
- **Technical Debt Prevention**: Quality gates block debt accumulation
- **Refactoring Safety**: Coverage enables confident changes
- **Architecture Preservation**: Boundary monitoring maintains design
- **Performance Stability**: Regression testing prevents degradation

### Project Health
- **Quality Visibility**: Continuous monitoring and reporting
- **Trend Analysis**: Historical quality metrics tracking
- **Risk Mitigation**: Early detection of quality issues
- **Stakeholder Confidence**: Demonstrable quality improvements

## Quality Gate Configuration

### CI Workflow: `.github/workflows/quality-gates.yml`
- Runs on all Go code changes
- Comprehensive quality analysis
- Artifact upload for reports
- Detailed quality summaries

### Scripts Directory: `scripts/`
- `quality_gates.sh` - Master quality gate runner
- `interface-counter` - Interface count monitoring
- `check_file_size.sh` - File size enforcement
- `complexity-checker` - Function complexity analysis
- `coverage.sh` - Test coverage tracking
- `performance_baseline.sh` - Performance baseline establishment
- `regression_test.sh` - Performance regression detection

### Tools Directory: `tools/`
- `interface-counter/` - Go-based interface counting tool
- `complexity-checker/` - Go-based complexity analysis tool

## Troubleshooting

### Quality Gate Failures
1. **Interface Count Exceeded**: Work with architecture team to consolidate interfaces
2. **File Size Violations**: Break large files into focused modules
3. **Complexity Issues**: Refactor complex functions into smaller helpers
4. **Coverage Below Threshold**: Add missing unit tests
5. **Performance Regression**: Investigate and optimize slow operations

### Local Development Issues
1. **Scripts Not Executable**: Run `chmod +x scripts/*.sh`
2. **Tools Not Built**: Run build commands in `tools/` directories
3. **Missing Dependencies**: Ensure `bc` and `make` are installed
4. **Go Module Issues**: Run `go mod tidy` to clean up dependencies

## Contributing to Quality Standards

### Proposing Changes
1. Create RFC document for significant standard changes
2. Discuss impact on development velocity
3. Consider backward compatibility during refactoring
4. Update thresholds gradually, not abruptly

### Improving Tools
1. Quality tools are open for contribution
2. Add new metrics or improve existing analysis
3. Enhance CI integration and reporting
4. Optimize tool performance and accuracy

---

**Note**: This quality infrastructure provides the foundation for maintaining and improving code quality throughout the Container Kit MCP refactoring process and beyond. The monitoring approach during refactoring ensures visibility without blocking progress, while establishing the foundation for strict enforcement post-refactoring.