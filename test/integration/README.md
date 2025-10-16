# Integration Tests

This directory contains integration tests for the containerization-assist project. These tests verify complete workflows and real operations with Docker and Kubernetes.

## Test Categories

### 1. Workflow Tests (`workflows/`)

Tests complete containerization workflows by chaining tools together:

- **containerization-workflow.test.ts** - Full end-to-end containerization journey:
  - analyze-repo → generate-dockerfile → build-image → scan-image → tag-image → generate-k8s-manifests
  - Tests single-module Node.js and Python applications
  - Tests session persistence and state management
  - Tests error handling across the workflow

- **multi-module-workflow.test.ts** - Monorepo and multi-module application workflows:
  - Detects multiple modules in a single repository
  - Generates Dockerfiles for each module independently
  - Generates Kubernetes manifests for multi-service deployments
  - Tests module isolation and independence

### 2. Docker Operations Tests

- **docker-operations-integration.test.ts** - Real Docker operations:
  - Building images from Dockerfiles (Alpine, Node.js apps)
  - Tagging images with multiple tags
  - Scanning images for vulnerabilities (requires Trivy)
  - Proper cleanup and lifecycle management

### 3. Kubernetes Operations Tests

- **kubernetes-operations-integration.test.ts** - Real Kubernetes operations:
  - Preparing namespaces and cluster resources
  - Generating deployment manifests
  - Deploying applications to cluster
  - Verifying deployment status and health

### 4. Infrastructure Tests (`infrastructure/`)

- **docker/client-error-handling.test.ts** - Docker client error detection and handling
- Tests various error scenarios with meaningful error messages

### 5. Other Integration Tests

- **error-guidance-propagation.test.ts** - Error guidance flow through layers
- **orchestrator-routing.test.ts** - Tool routing and orchestration
- **health-check.test.ts** - Health check functionality via CLI
- **kubernetes-fast-fail.test.ts** - K8s configuration validation

## Prerequisites

### Required for All Tests
- Node.js 18+ installed
- Project built (`npm run build`)

### Required for Docker Tests
- Docker daemon running and accessible
- Sufficient disk space (tests build real images)
- Trivy installed (optional, for vulnerability scanning tests)

### Required for Kubernetes Tests
- Kubernetes cluster accessible (kind, minikube, or real cluster)
- kubectl configured with valid kubeconfig
- Sufficient cluster resources for test deployments

## Running Tests

### Run All Integration Tests
```bash
npm test
```

Note: Some integration tests are skipped by default due to ES module loading issues with @kubernetes/client-node. See "Known Issues" below.

### Run Specific Test Files

Due to the Kubernetes client ES module issue, workflow and operations tests need to be run directly via Node:

```bash
# Build first
npm run build

# Run workflow tests directly (requires manual execution)
# These tests are currently in the ignore list - see Known Issues below

# Run infrastructure tests (these work with jest)
npm test -- test/integration/infrastructure
```

### Run Tests in CI

The project includes smoke tests that run key workflows:

```bash
npm run smoke:journey  # End-to-end smoke test
```

## Test Structure

All integration tests follow this pattern:

```typescript
import { createApp } from '@/app';
import type { AppRuntime } from '@/types/runtime';

describe('Test Suite', () => {
  let runtime: AppRuntime;

  beforeAll(async () => {
    runtime = createApp({ logger });
    // Setup
  });

  afterAll(async () => {
    // Cleanup
    await runtime.stop();
  });

  it('should test workflow', async () => {
    const result = await runtime.execute('tool-name', {
      // parameters
    }, { sessionId: 'test-session' });

    expect(result.ok).toBe(true);
    // assertions
  });
});
```

## Test Data

Integration tests use fixtures from `test/__support__/fixtures/`:
- `node-express/` - Node.js Express application
- `python-flask/` - Python Flask application
- `java-spring-boot-maven/` - Java Spring Boot with Maven
- `dotnet-webapi/` - .NET Web API
- And more...

Tests may also create temporary fixtures dynamically using `createTestTempDir()`.

## Known Issues

### ES Module Loading Issue

Several integration tests are currently in the jest ignore list due to ES module loading issues with `@kubernetes/client-node`:

- `test/integration/workflows/containerization-workflow.test.ts`
- `test/integration/workflows/multi-module-workflow.test.ts`
- `test/integration/docker-operations-integration.test.ts`
- `test/integration/kubernetes-operations-integration.test.ts`

These tests:
1. Are structurally complete and follow best practices
2. Import tools which transitively import the Kubernetes client
3. Hit a jest/ES module compatibility issue at runtime
4. Can be run manually outside of jest or via end-to-end smoke tests

**Workarounds:**
1. Run via smoke tests: `npm run smoke:journey`
2. Run after fixing the @kubernetes/client-node ES module issue
3. Tests are designed to gracefully skip when Docker/K8s not available

## Skipping Tests Based on Environment

Tests automatically detect and skip based on availability:

```typescript
beforeAll(async () => {
  const healthCheck = await runtime.healthCheck();
  dockerAvailable = healthCheck.dependencies?.docker?.available || false;
  k8sAvailable = healthCheck.dependencies?.kubernetes?.available || false;
});

it('should test docker operation', async () => {
  if (!dockerAvailable) {
    console.warn('Skipping test: Docker not available');
    return;
  }
  // test code
});
```

## Test Timeouts

Integration tests have extended timeouts:
- Default: 30 seconds
- Workflow tests: 90-120 seconds (full end-to-end flows)
- Docker build tests: 60 seconds
- Kubernetes deploy tests: 90 seconds

## Cleanup and Resource Management

Tests use `DockerTestCleaner` for automatic cleanup:

```typescript
import { DockerTestCleaner } from '../__support__/utilities/docker-test-cleaner';

const testCleaner = new DockerTestCleaner(logger, dockerClient, {
  verifyCleanup: true
});

// Track resources
testCleaner.trackImage(imageId);
testCleaner.trackContainer(containerId);

// Cleanup in afterAll
await testCleaner.cleanup();
```

## Contributing

When adding new integration tests:

1. Place tests in the appropriate category directory
2. Use fixtures from `test/__support__/fixtures/` when possible
3. Include proper cleanup in `afterAll` hooks
4. Add graceful skipping when dependencies unavailable
5. Document any special prerequisites
6. Use meaningful session IDs for debugging
7. Set appropriate test timeouts

## Related Documentation

- [Developer Guide](../../docs/developer-guide.md) - Development setup
- [Tool Capabilities](../../docs/tool-capabilities.md) - Tool reference
- [Quality Gates](../../docs/quality-gates.md) - Quality assurance
- [Examples](../../docs/examples/) - Usage examples
