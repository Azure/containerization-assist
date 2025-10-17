/**
 * Parameterized Test Data Patterns
 * Data-driven testing utilities for comprehensive test coverage
 */

import { goldenFileLoader, loadGoldenFile } from './golden-file-loader';
import { EnvironmentCapabilities, detectEnvironment } from '../utilities/environment-detector';

export interface TestScenario<TInput, TExpected> {
  name: string;
  description?: string;
  input: TInput;
  expected: TExpected;
  tags?: string[];
  skip?: boolean | string; // Skip reason if string
  environment?: Array<keyof Omit<EnvironmentCapabilities, 'platform'>>; // Required env capabilities
  timeout?: number;
  variant?: string; // For golden file loading
}

export interface TestSuite<TInput, TExpected> {
  name: string;
  description: string;
  scenarios: TestScenario<TInput, TExpected>[];
  setup?: () => Promise<void> | void;
  teardown?: () => Promise<void> | void;
}

/**
 * Factory for creating parameterized test scenarios
 */
export class ParameterizedTestFactory {
  /**
   * Create test scenarios from golden files
   */
  static async createFromGoldenFiles<TInput, TExpected>(
    toolName: string,
    inputs: Record<string, TInput>,
    options: {
      variant?: string;
      tags?: string[];
      environment?: Array<keyof Omit<EnvironmentCapabilities, 'platform'>>;
    } = {},
  ): Promise<TestScenario<TInput, TExpected>[]> {
    const { variant, tags = [], environment = [] } = options;
    const scenarios: TestScenario<TInput, TExpected>[] = [];

    for (const [fixtureName, input] of Object.entries(inputs)) {
      const expected = await loadGoldenFile<TExpected>(toolName, fixtureName, variant);

      if (expected) {
        scenarios.push({
          name: `${toolName}-${fixtureName}${variant ? `-${variant}` : ''}`,
          description: `Test ${toolName} with ${fixtureName} fixture`,
          input,
          expected,
          tags: [toolName, fixtureName, ...tags],
          environment,
          variant,
        });
      }
    }

    return scenarios;
  }

  /**
   * Create test scenarios with variants
   */
  static createWithVariants<TInput, TExpected>(
    baseName: string,
    baseInput: TInput,
    variants: Array<{
      name: string;
      input: Partial<TInput>;
      expected: TExpected;
      description?: string;
      tags?: string[];
    }>,
  ): TestScenario<TInput, TExpected>[] {
    return variants.map((variant) => ({
      name: `${baseName}-${variant.name}`,
      description: variant.description || `${baseName} with ${variant.name} variant`,
      input: { ...baseInput, ...variant.input },
      expected: variant.expected,
      tags: [baseName, variant.name, ...(variant.tags || [])],
      variant: variant.name,
    }));
  }

  /**
   * Create environment-conditional scenarios
   */
  static createEnvironmentConditional<TInput, TExpected>(
    scenarios: TestScenario<TInput, TExpected>[],
    capabilities: EnvironmentCapabilities,
  ): TestScenario<TInput, TExpected>[] {
    return scenarios.map((scenario) => {
      if (!scenario.environment) {
        return scenario;
      }

      const missingCapabilities = scenario.environment.filter(
        (capability) => !capabilities[capability].available,
      );

      if (missingCapabilities.length > 0) {
        return {
          ...scenario,
          skip: `Missing required capabilities: ${missingCapabilities.join(', ')}`,
        };
      }

      return scenario;
    });
  }
}

/**
 * Pre-defined test scenarios for common tools
 */
export class CommonTestScenarios {
  /**
   * Analyze-repo tool scenarios
   */
  static async createAnalyzeRepoScenarios(): Promise<TestScenario<any, any>[]> {
    return ParameterizedTestFactory.createFromGoldenFiles(
      'analyze-repo',
      {
        'java-spring-boot-maven': {
          projectPath: './test/__support__/fixtures/java-spring-boot-maven',
          analysisType: 'full',
        },
        'node-express': {
          projectPath: './test/__support__/fixtures/node-express',
          analysisType: 'full',
        },
        'dotnet-webapi': {
          projectPath: './test/__support__/fixtures/dotnet-webapi',
          analysisType: 'full',
        },
        'python-flask': {
          projectPath: './test/__support__/fixtures/python-flask',
          analysisType: 'full',
        },
      },
      {
        tags: ['analysis', 'repository'],
        environment: [], // No special environment requirements
      },
    );
  }

  /**
   * Generate-dockerfile tool scenarios
   */
  static async createGenerateDockerfileScenarios(): Promise<TestScenario<any, any>[]> {
    const basicScenarios = await ParameterizedTestFactory.createFromGoldenFiles(
      'generate-dockerfile',
      {
        'java-maven': {
          projectType: 'java',
          buildTool: 'maven',
          projectPath: './test/__support__/fixtures/java-spring-boot-maven',
        },
        'node-npm': {
          projectType: 'nodejs',
          buildTool: 'npm',
          projectPath: './test/__support__/fixtures/node-express',
        },
        'dotnet-core': {
          projectType: 'dotnet',
          buildTool: 'dotnet',
          projectPath: './test/__support__/fixtures/dotnet-webapi',
        },
      },
    );

    // Add security-hardened variants
    const securityScenarios = await ParameterizedTestFactory.createFromGoldenFiles(
      'generate-dockerfile',
      {
        'java-maven': {
          projectType: 'java',
          buildTool: 'maven',
          projectPath: './test/__support__/fixtures/java-spring-boot-maven',
          securityHardened: true,
        },
        'node-npm': {
          projectType: 'nodejs',
          buildTool: 'npm',
          projectPath: './test/__support__/fixtures/node-express',
          securityHardened: true,
        },
      },
      {
        variant: 'security-hardened',
        tags: ['security', 'hardened'],
      },
    );

    return [...basicScenarios, ...securityScenarios];
  }

  /**
   * Build-image tool scenarios
   */
  static async createBuildImageScenarios(): Promise<TestScenario<any, any>[]> {
    return ParameterizedTestFactory.createFromGoldenFiles(
      'build-image',
      {
        'dockerfile-basic': {
          dockerfilePath: './test/__support__/fixtures/dockerfiles/basic.Dockerfile',
          context: './test/__support__/fixtures/java-spring-boot-maven',
          imageTag: 'test-app:latest',
        },
        'dockerfile-multi-stage': {
          dockerfilePath: './test/__support__/fixtures/dockerfiles/multi-stage.Dockerfile',
          context: './test/__support__/fixtures/node-express',
          imageTag: 'test-node:latest',
        },
      },
      {
        tags: ['build', 'docker'],
        environment: ['docker'], // Requires Docker
      },
    );
  }

  /**
   * Scan tool scenarios
   */
  static async createScanScenarios(): Promise<TestScenario<any, any>[]> {
    const scenarios = [
      {
        name: 'clean-image',
        input: { imageId: 'alpine:3.18' },
        expected: await loadGoldenFile('scan-image', 'image-clean'),
        tags: ['security', 'clean'],
      },
      {
        name: 'vulnerable-image',
        input: { imageId: 'node:14' }, // Older version with known vulnerabilities
        expected: await loadGoldenFile('scan-image', 'image-vulnerable'),
        tags: ['security', 'vulnerable'],
      },
    ].filter((s) => s.expected); // Only include scenarios with golden files

    return scenarios.map((s) => ({
      name: `scan-${s.name}`,
      description: `Security scan of ${s.input.imageId}`,
      input: s.input,
      expected: s.expected,
      tags: ['scan-image', ...s.tags],
      environment: ['trivy' as keyof Omit<EnvironmentCapabilities, 'platform'>],
    }));
  }

  /**
   * Generate-k8s-manifests tool scenarios
   */
  static async createGenerateK8sManifestsScenarios(): Promise<TestScenario<any, any>[]> {
    return ParameterizedTestFactory.createFromGoldenFiles(
      'generate-k8s-manifests',
      {
        'web-app': {
          appName: 'test-web-app',
          imageTag: 'test-app:latest',
          port: 8080,
          environment: 'development',
        },
        microservices: {
          appName: 'test-microservice',
          imageTag: 'test-service:latest',
          port: 3000,
          environment: 'production',
          replicas: 3,
        },
      },
      {
        tags: ['kubernetes', 'manifests'],
        environment: [], // Can run without K8s cluster
      },
    );
  }
}

/**
 * Test runner utilities for parameterized tests
 */
export class ParameterizedTestRunner {
  /**
   * Run a test suite with environment detection
   */
  static runSuite<TInput, TExpected>(
    suite: TestSuite<TInput, TExpected>,
    testFunction: (
      input: TInput,
      expected: TExpected,
      scenario: TestScenario<TInput, TExpected>,
    ) => Promise<void> | void,
    capabilities?: EnvironmentCapabilities,
  ): void {
    describe(suite.name, () => {
      if (suite.setup) {
        beforeAll(suite.setup);
      }

      if (suite.teardown) {
        afterAll(suite.teardown);
      }

      const scenarios = capabilities
        ? ParameterizedTestFactory.createEnvironmentConditional(suite.scenarios, capabilities)
        : suite.scenarios;

      scenarios.forEach((scenario) => {
        const testRunner = scenario.skip
          ? typeof scenario.skip === 'string'
            ? test.skip
            : test.skip
          : test;

        const testName = scenario.description || scenario.name;
        const testTimeout = scenario.timeout;

        testRunner(
          testName,
          async () => {
            await testFunction(scenario.input, scenario.expected, scenario);
          },
          testTimeout,
        );

        // Log skip reason if provided
        if (typeof scenario.skip === 'string') {
          console.log(`Skipping ${testName}: ${scenario.skip}`);
        }
      });
    });
  }

  /**
   * Create a describe block with environment checks
   */
  static describeWithEnvironment(
    name: string,
    requirements: Array<keyof Omit<EnvironmentCapabilities, 'platform'>>,
    fn: () => void,
    capabilities?: EnvironmentCapabilities,
  ): void {
    if (!capabilities) {
      // Runtime environment detection
      describe(name, () => {
        let envCapabilities: EnvironmentCapabilities;

        beforeAll(async () => {
          envCapabilities = await detectEnvironment({ timeout: 3000 });
        });

        const shouldSkip = () => {
          if (!envCapabilities) return true;
          return requirements.some((req) => !envCapabilities[req].available);
        };

        test('environment requirements', () => {
          const missing = requirements.filter((req) => !envCapabilities[req].available);
          if (missing.length > 0) {
            console.log(`Skipping ${name} - Missing: ${missing.join(', ')}`);
          }
          expect(missing.length).toBe(0);
        });

        if (!shouldSkip()) {
          fn();
        }
      });
    } else {
      // Use provided capabilities
      const missing = requirements.filter((req) => !capabilities[req].available);

      if (missing.length > 0) {
        console.log(`Skipping ${name} - Missing: ${missing.join(', ')}`);
        describe.skip(name, fn);
      } else {
        describe(name, fn);
      }
    }
  }
}

/**
 * Export convenience functions
 */
export const createTestScenarios = ParameterizedTestFactory;
export const runParameterizedTests = ParameterizedTestRunner.runSuite;
export const describeWithEnv = ParameterizedTestRunner.describeWithEnvironment;
