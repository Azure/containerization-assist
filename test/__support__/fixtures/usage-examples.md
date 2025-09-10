# Test Data Strategy - Usage Examples

This document demonstrates how to use the comprehensive test data strategy implemented for the containerization assist project.

## Overview

The test data strategy provides:
- **Golden Files**: Expected outputs for regression testing
- **Parameterized Testing**: Data-driven test scenarios
- **Environment-Aware Loading**: Dynamic fixture selection based on available infrastructure
- **Unified Mock Factory**: Comprehensive mocking system
- **Fixture Registry**: Centralized fixture management
- **Validation System**: Quality assurance for test data

## Quick Start

### Basic Setup

```typescript
// Import the complete system
import {
  createTestEnvironment,
  loadProjectWithEnvironment,
  runParameterizedTests,
  CommonTestScenarios
} from '@test/__support__/fixtures';

// Initialize test environment
const { capabilities, fixtures } = await createTestEnvironment({
  detectEnvironment: true,
  loadFixtures: true
});
```

### Environment Detection

```typescript
import { checkTestEnvironment } from '@test/__support__/fixtures';

// Get environment status
const { summary, capabilities, recommendations } = await checkTestEnvironment();

console.log(summary); // Detailed environment report
console.log(recommendations); // What to install for better testing
```

## Parameterized Testing Examples

### Tool Testing with Golden Files

```typescript
import { CommonTestScenarios, runParameterizedTests } from '@test/__support__/fixtures';

describe('Analyze Repository Tool', () => {
  runParameterizedTests(
    {
      name: 'Repository Analysis',
      description: 'Test analyze-repo tool with various project types',
      scenarios: await CommonTestScenarios.createAnalyzeRepoScenarios()
    },
    async (input, expected, scenario) => {
      // Execute the actual tool
      const result = await analyzeRepoTool.execute(input);
      
      // Compare with golden file data
      expect(result.success).toBe(true);
      expect(result.data).toMatchObject(expected);
    }
  );
});
```

### Custom Tool Scenarios

```typescript
import { createToolTestScenarios } from '@test/__support__/fixtures';

describe('Build Image Tool', () => {
  const scenarios = await createToolTestScenarios('build-image', {
    'basic-dockerfile': {
      dockerfilePath: './fixtures/Dockerfile.basic',
      context: './fixtures/java-app',
      imageTag: 'test:latest'
    },
    'multi-stage': {
      dockerfilePath: './fixtures/Dockerfile.multistage',
      context: './fixtures/node-app',
      imageTag: 'test:v1.0'
    }
  }, {
    environment: ['docker'] // Requires Docker
  });

  runParameterizedTests(
    { name: 'Build Image Scenarios', scenarios },
    async (input, expected) => {
      const result = await buildImageTool.execute(input);
      expect(result).toEqual(expected);
    }
  );
});
```

## Environment-Aware Testing

### Conditional Tests Based on Infrastructure

```typescript
import { describeWithEnv } from '@test/__support__/fixtures';

// Only run if Docker is available
describeWithEnv('Docker Integration Tests', ['docker'], () => {
  test('should build real Docker image', async () => {
    const dockerfile = await loadDockerWithEnvironment('basic-node');
    const result = await docker.build(dockerfile);
    expect(result.success).toBe(true);
  });
});

// Only run if Kubernetes cluster is available
describeWithEnv('Kubernetes Deployment Tests', ['kubernetes'], () => {
  test('should deploy to cluster', async () => {
    const manifests = await loadKubernetesWithEnvironment('web-app', 'development');
    const result = await kubectl.apply(manifests);
    expect(result.success).toBe(true);
  });
});
```

### Dynamic Fixture Loading

```typescript
import { environmentAwareLoader } from '@test/__support__/fixtures';

describe('Environment Adaptive Tests', () => {
  test('should load appropriate fixtures', async () => {
    // Automatically chooses real or mock fixtures based on environment
    const project = await environmentAwareLoader.loadProjectFixture('java');
    const docker = await environmentAwareLoader.loadDockerFixture('spring-boot');
    const k8s = await environmentAwareLoader.loadKubernetesFixture('web-app');
    
    // All fixtures are loaded appropriate to the current environment
    expect(project.success).toBe(true);
    expect(docker.success).toBe(true);
    expect(k8s.success).toBe(true);
  });
});
```

## Mock Factory Usage

### Infrastructure Mocks

```typescript
import { unifiedMockFactory } from '@test/__support__/fixtures';

describe('Mock Integration Tests', () => {
  test('should use Docker mocks when Docker unavailable', () => {
    const dockerMock = unifiedMockFactory.createDockerMock('success');
    const k8sMock = unifiedMockFactory.createKubernetesMock('kind');
    const trivyMock = unifiedMockFactory.createTrivyMock('clean');
    
    // Use mocks in your tests
    expect(dockerMock.buildImage).toBeDefined();
    expect(k8sMock.applyManifest).toBeDefined();
    expect(trivyMock.scanImage).toBeDefined();
  });
});
```

### Environment-Based Mocking

```typescript
import { mockEnvironment } from '@test/__support__/fixtures';

describe('Environment Mock Tests', () => {
  test('should create appropriate mocks for environment', async () => {
    const capabilities = await detectEnvironment();
    const mocks = mockEnvironment(capabilities);
    
    // Mocks automatically match your environment capabilities
    expect(mocks.docker).toBeDefined();
    expect(mocks.kubernetes).toBeDefined();
    expect(mocks.trivy).toBeDefined();
  });
});
```

## Golden File Management

### Loading Golden Files

```typescript
import { loadGoldenFile } from '@test/__support__/fixtures';

describe('Golden File Tests', () => {
  test('should match expected output', async () => {
    const expectedOutput = await loadGoldenFile('analyze-repo', 'spring-boot-maven');
    
    const actualResult = await analyzeRepo('./fixtures/spring-boot-project');
    
    expect(actualResult).toEqual(expectedOutput);
  });
});
```

### Updating Golden Files

```typescript
import { saveGoldenFile } from '@test/__support__/fixtures';

// When tool output changes, update golden files
const newOutput = await analyzeRepo('./fixtures/spring-boot-project');
await saveGoldenFile('analyze-repo', 'spring-boot-maven', newOutput);
```

## Fixture Validation

### Validating Test Data Quality

```typescript
import { validateAllGoldenFiles, fixtureValidator } from '@test/__support__/fixtures';

describe('Fixture Quality Assurance', () => {
  test('should validate all golden files', async () => {
    const reports = await validateAllGoldenFiles();
    const failures = reports.filter(r => !r.valid);
    
    if (failures.length > 0) {
      console.warn('Invalid fixtures found:', failures.map(f => f.fixtureId));
    }
    
    expect(failures.length).toBe(0);
  });
  
  test('should validate specific fixture', async () => {
    const report = await fixtureValidator.validateFixture('project-java-spring-boot');
    
    expect(report.success).toBe(true);
    expect(report.data.valid).toBe(true);
    expect(report.data.score).toBeGreaterThan(80);
  });
});
```

## Advanced Patterns

### Custom Validation Rules

```typescript
import { fixtureValidator } from '@test/__support__/fixtures';

// Add custom validation rule
fixtureValidator.addRule({
  name: 'security-scan-format',
  description: 'Validate security scan result format',
  validate: (data, metadata) => {
    if (metadata?.category === 'scan') {
      const scanData = data as any;
      if (!scanData.summary || !scanData.vulnerabilities) {
        return {
          passed: false,
          message: 'Security scan must have summary and vulnerabilities',
          severity: 'error'
        };
      }
    }
    return { passed: true, severity: 'info' };
  }
});
```

### Custom Test Scenarios

```typescript
import { ParameterizedTestFactory } from '@test/__support__/fixtures';

const customScenarios = ParameterizedTestFactory.createWithVariants(
  'dockerfile-generation',
  { projectPath: './fixtures/base-project' },
  [
    {
      name: 'basic',
      input: { securityHardened: false },
      expected: await loadGoldenFile('generate-dockerfile', 'basic'),
      description: 'Basic Dockerfile generation'
    },
    {
      name: 'security-hardened',
      input: { securityHardened: true },
      expected: await loadGoldenFile('generate-dockerfile', 'security-hardened'),
      description: 'Security-hardened Dockerfile generation'
    }
  ]
);
```

### Finding Fixtures Dynamically

```typescript
import { findTestData } from '@test/__support__/fixtures';

// Find all Java project fixtures
const javaProjects = await findTestData({
  type: 'project',
  category: 'java'
});

// Find all golden files for security tools
const securityGoldenFiles = await findTestData({
  type: 'golden',
  tags: ['security', 'scan']
});

// Find all Kubernetes manifests
const k8sManifests = await findTestData({
  type: 'k8s'
});
```

## Best Practices

1. **Use Environment-Aware Loading**: Let the system choose appropriate fixtures
2. **Validate Regularly**: Run fixture validation in CI/CD
3. **Update Golden Files**: Keep expected outputs current with tool changes
4. **Tag Appropriately**: Use descriptive tags for easy fixture discovery
5. **Leverage Parameterized Tests**: Cover multiple scenarios efficiently
6. **Mock Gracefully**: Fall back to mocks when real infrastructure unavailable

## Migration from Old System

The new system maintains backward compatibility with existing fixtures while providing enhanced capabilities:

```typescript
// Old approach
const expectedOutput = require('../fixtures/expected-outputs/analyze-result.json');

// New approach (backward compatible)
const expectedOutput = await loadGoldenFile('analyze-repo', 'spring-boot-maven');

// Enhanced new approach
const scenarios = await CommonTestScenarios.createAnalyzeRepoScenarios();
runParameterizedTests({ name: 'Analysis Tests', scenarios }, testFunction);
```

This comprehensive system provides robust, maintainable, and environment-aware test data management for the containerization assist project.