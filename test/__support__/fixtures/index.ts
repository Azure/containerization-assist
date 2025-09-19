/**
 * Test Data Strategy - Comprehensive Fixture Management System
 * 
 * This module provides a complete solution for managing test fixtures, golden files,
 * parameterized test data, and environment-aware loading for the containerization
 * assist project.
 */

// Core fixture management
export {
  FixtureRegistry,
  FixtureMetadata,
  FixtureSearchCriteria,
  fixtureRegistry,
  findFixtures,
  loadFixture
} from './fixture-registry';

// Golden file management
export {
  GoldenFileLoader,
  GoldenFileMetadata,
  ToolGoldenFileInfo,
  WorkflowGoldenFileInfo,
  GoldenFileLoadOptions,
  goldenFileLoader,
  loadGoldenFile,
  loadWorkflowGoldenFile,
  saveGoldenFile
} from './golden-file-loader';

// Parameterized test data
export {
  TestScenario,
  TestSuite,
  ParameterizedTestFactory,
  CommonTestScenarios,
  ParameterizedTestRunner,
  createTestScenarios,
  runParameterizedTests,
  describeWithEnv
} from './parameterized-test-data';

// Environment-aware loading
export {
  EnvironmentAwareFixtureLoader,
  LoaderConfiguration,
  FixtureLoadOptions,
  ProjectFixture,
  environmentAwareLoader,
  loadProjectWithEnvironment,
  loadDockerWithEnvironment,
  loadKubernetesWithEnvironment
} from './environment-aware-loader';

// Fixture validation
export {
  FixtureValidator,
  ValidationRule,
  ValidationRuleResult,
  FixtureValidationReport,
  ValidationOptions,
  fixtureValidator,
  validateFixture,
  validateAllGoldenFiles,
  validateFixturesByType
} from './fixture-validation';

// Unified mock factory (from mocks directory)
export {
  MockFactory,
  MockBehavior,
  SecurityFindingsLevel,
  MockScenario,
  mockFactory,
  mockDocker,
  mockKubernetes,
  mockTrivy,
  mockTool,
  mockEnvironment
} from '../mocks/mock-factory';

// Environment detection utilities (from utilities directory)
export {
  EnvironmentCapabilities,
  DetectionOptions,
  detectEnvironment,
  createEnvironmentReport,
  createConditionalDescribe
} from '../utilities/environment-detector';

/**
 * Quick Start Examples and Patterns
 */

/**
 * Example 1: Load a project fixture with environment awareness
 * 
 * ```typescript
 * import { loadProjectWithEnvironment } from '@test/__support__/fixtures';
 * 
 * // Loads real project if available, falls back to mock if needed
 * const project = await loadProjectWithEnvironment('java', 'spring-boot');
 * ```
 */

/**
 * Example 2: Create parameterized tests with golden files
 * 
 * ```typescript
 * import { CommonTestScenarios, runParameterizedTests } from '@test/__support__/fixtures';
 * 
 * const scenarios = await CommonTestScenarios.createAnalyzeRepoScenarios();
 * 
 * runParameterizedTests(
 *   { name: 'Analyze Repository', scenarios },
 *   async (input, expected) => {
 *     const result = await analyzeRepoTool.execute(input);
 *     expect(result).toEqual(expected);
 *   }
 * );
 * ```
 */

/**
 * Example 3: Environment-conditional testing
 * 
 * ```typescript
 * import { describeWithEnv } from '@test/__support__/fixtures';
 * 
 * describeWithEnv('Docker Integration Tests', ['docker'], () => {
 *   test('should build image', async () => {
 *     // This test only runs if Docker is available
 *   });
 * });
 * ```
 */

/**
 * Example 4: Mock factory usage
 * 
 * ```typescript
 * import { mockFactory } from '@test/__support__/fixtures';
 * 
 * const dockerMock = mockFactory.createDockerMock('success');
 * const toolMock = mockFactory.createBuildImageMock('failure');
 * ```
 */

/**
 * Example 5: Fixture validation
 * 
 * ```typescript
 * import { validateAllGoldenFiles } from '@test/__support__/fixtures';
 * 
 * const reports = await validateAllGoldenFiles();
 * const failedFixtures = reports.filter(r => !r.valid);
 * ```
 */

/**
 * High-level convenience functions for common patterns
 */

import { TestScenario } from './parameterized-test-data';
import { EnvironmentCapabilities } from '../utilities/environment-detector';

/**
 * Create a complete test environment setup
 */
export async function createTestEnvironment(options: {
  detectEnvironment?: boolean;
  loadFixtures?: boolean;
  validateFixtures?: boolean;
} = {}) {
  const { detectEnvironment: detect = true, loadFixtures = true, validateFixtures = false } = options;
  
  const setup: {
    capabilities?: EnvironmentCapabilities;
    fixtures?: any;
    validation?: any;
  } = {};

  if (detect) {
    const { detectEnvironment } = await import('../utilities/environment-detector');
    setup.capabilities = await detectEnvironment();
  }

  if (loadFixtures) {
    const { fixtureRegistry } = await import('./fixture-registry');
    await fixtureRegistry.initialize();
    setup.fixtures = fixtureRegistry;
  }

  if (validateFixtures) {
    const { validateAllGoldenFiles } = await import('./fixture-validation');
    setup.validation = await validateAllGoldenFiles();
  }

  return setup;
}

/**
 * Create tool test scenarios for any tool
 */
export async function createToolTestScenarios<TInput, TExpected>(
  toolName: string,
  inputs: Record<string, TInput>,
  options: {
    variant?: string;
    environment?: Array<keyof Omit<EnvironmentCapabilities, 'platform'>>;
  } = {}
): Promise<TestScenario<TInput, TExpected>[]> {
  const { ParameterizedTestFactory } = await import('./parameterized-test-data');
  return ParameterizedTestFactory.createFromGoldenFiles(toolName, inputs, options);
}

/**
 * Quick fixture finder with common filters
 */
export async function findTestData(query: {
  type?: 'project' | 'golden' | 'k8s' | 'docker';
  category?: string;
  tags?: string[];
}) {
  const { fixtureRegistry } = await import('./fixture-registry');
  await fixtureRegistry.initialize();
  return fixtureRegistry.find(query);
}

/**
 * Environment status checker
 */
export async function checkTestEnvironment(): Promise<{
  summary: string;
  capabilities: EnvironmentCapabilities;
  recommendations: string[];
}> {
  const { detectEnvironment, createEnvironmentReport } = await import('../utilities/environment-detector');
  
  const capabilities = await detectEnvironment();
  const summary = createEnvironmentReport(capabilities);
  
  const recommendations: string[] = [];
  
  if (!capabilities.docker.available) {
    recommendations.push('Install Docker for container-related tests');
  }
  
  if (!capabilities.kubernetes.available) {
    recommendations.push('Set up local Kubernetes (kind/minikube) for deployment tests');
  }
  
  if (!capabilities.trivy.available) {
    recommendations.push('Install Trivy for security scanning tests');
  }

  return { summary, capabilities, recommendations };
}