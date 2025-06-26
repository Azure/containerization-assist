# Coverage Baseline Report for Container Copilot

## Executive Summary

Generated on: 2025-06-25

This report provides a baseline assessment of test coverage across the Container Copilot codebase. The analysis reveals significant opportunities for improvement, with most core packages having coverage well below the target of 80%.

## Coverage Summary by Package Category

### Core Packages (Target: ≥80%)

| Package | Current Coverage | Gap to 80% | Priority |
|---------|-----------------|------------|----------|
| pkg/pipeline/databasedetectionstage | 83.5% | ✅ Exceeds | Low |
| pkg/mcp/utils | 63.2% | 16.8% | Medium |
| pkg/pipeline | 59.6% | 20.4% | High |
| pkg/pipeline/repoanalysisstage | 56.5% | 23.5% | High |
| pkg/mcp/internal/pipeline | 43.1% | 36.9% | High |
| pkg/mcp/internal/transport | 39.2% | 40.8% | High |
| pkg/mcp/internal/types | 34.5% | 45.5% | Medium |
| pkg/mcp/internal/utils | 33.8% | 46.2% | Medium |
| pkg/mcp/internal/observability | 32.1% | 47.9% | High |
| pkg/core/git | 23.7% | 56.3% | High |
| pkg/mcp/internal/customizer | 23.8% | 56.2% | Medium |
| pkg/mcp/internal/core | 18.5% | 61.5% | Critical |
| pkg/pipeline/dockerstage | 17.9% | 62.1% | High |
| pkg/core/docker | 17.7% | 62.3% | Critical |
| pkg/pipeline/manifeststage | 17.7% | 62.3% | High |
| pkg/mcp/internal/runtime/conversation | 17.2% | 62.8% | Critical |
| pkg/mcp/internal/testutil | 17.0% | 63.0% | Low |
| pkg/mcp/internal/analyze | 9.1% | 70.9% | Critical |
| pkg/core/kubernetes | 8.9% | 71.1% | Critical |
| pkg/mcp/internal/session | 8.4% | 71.6% | Critical |
| pkg/mcp/internal/build | 7.3% | 72.7% | Critical |
| pkg/mcp/internal/orchestration | 6.1% | 73.9% | Critical |
| pkg/mcp/internal/deploy | 5.1% | 74.9% | Critical |
| cmd/mcp-server | 1.3% | 78.7% | Critical |
| pkg/mcp/internal/runtime | 0.4% | 79.6% | Critical |

### Packages with 0% Coverage (Need Immediate Attention)

- pkg/ai
- pkg/clients
- pkg/core/analysis
- pkg/deps
- pkg/docker
- pkg/filetree
- pkg/k8s
- pkg/kind
- pkg/logger
- pkg/mcp (main package)
- pkg/mcp/internal
- pkg/mcp/internal/conversation
- pkg/mcp/internal/orchestration/testutil
- pkg/mcp/internal/profiling/testutil
- pkg/mcp/internal/registry
- pkg/mcp/internal/scan
- pkg/mcp/internal/server
- pkg/mcp/internal/workflow
- pkg/mcp/testing
- pkg/mcp/types
- pkg/runner
- pkg/templating
- pkg/utils
- cmd
- cmd/tool-generator

## Key Findings

1. **Critical Coverage Gaps**:
   - Core MCP functionality has only 18.5% coverage
   - Essential Docker and Kubernetes packages are below 20%
   - Command-line tools have minimal coverage (1.3% for mcp-server)

2. **High-Performing Packages**:
   - Only 1 package (databasedetectionstage) meets the 80% target
   - 4 packages are above 50% coverage
   - Most packages are below 35% coverage

3. **Zero Coverage Concerns**:
   - 26 packages have 0% test coverage
   - Many of these are core functionality (ai, clients, logger)
   - Includes critical components like registry and server

## Recommendations

### Immediate Actions (Sprint 2)

1. **Focus on Critical Core Packages**:
   - pkg/mcp/internal/core (61.5% gap)
   - pkg/core/docker (62.3% gap)
   - pkg/core/kubernetes (71.1% gap)
   - pkg/mcp/internal/runtime (79.6% gap)

2. **Address Zero-Coverage Packages**:
   - Start with packages that other components depend on
   - Prioritize pkg/clients, pkg/logger, pkg/utils

3. **Quick Wins**:
   - pkg/mcp/utils (16.8% gap) - already at 63.2%
   - pkg/pipeline (20.4% gap) - already at 59.6%

### Testing Strategy

1. **Table-Driven Tests**: Implement for all public functions
2. **Integration Tests**: Focus on MCP server and pipeline execution
3. **Mock Strategy**: Create comprehensive mocks for external dependencies
4. **Coverage Enforcement**: Set up CI to fail if coverage drops below current baseline

## Next Steps

1. Set up automated coverage tracking in CI/CD
2. Create detailed test plans for each critical package
3. Establish team ownership for coverage improvements
4. Schedule regular coverage reviews

## Metrics for Success

- Core packages achieve ≥80% coverage
- No package with 0% coverage
- Average coverage across all packages ≥60%
- All new code requires tests (enforced by CI)
