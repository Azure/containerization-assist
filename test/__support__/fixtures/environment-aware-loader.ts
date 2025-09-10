/**
 * Environment-Aware Fixture Loader
 * Dynamic fixture selection based on available infrastructure capabilities
 */

import { EnvironmentCapabilities, detectEnvironment } from '../utilities/environment-detector';
import { fixtureRegistry, FixtureMetadata } from './fixture-registry';
import { unifiedMockFactory } from '../mocks/unified-mock-factory';
import { goldenFileLoader } from './golden-file-loader';
import { Result, Success, Failure } from '../../../src/types';

export interface LoaderConfiguration {
  preferReal: boolean; // Prefer real infrastructure over mocks when available
  fallbackToMocks: boolean; // Fall back to mocks when real infrastructure unavailable
  cacheResults: boolean; // Cache environment detection results
  timeout: number; // Environment detection timeout
}

export interface FixtureLoadOptions {
  variant?: string;
  mockBehavior?: 'success' | 'failure' | 'timeout' | 'partial';
  forceReal?: boolean; // Force real infrastructure usage
  forceMock?: boolean; // Force mock usage
}

export interface ProjectFixture {
  path: string;
  files: string[];
  metadata: {
    projectType: string;
    buildTool: string;
    dependencies?: any[];
  };
}

/**
 * Environment-aware fixture loader that adapts to available infrastructure
 */
export class EnvironmentAwareFixtureLoader {
  private capabilities?: EnvironmentCapabilities;
  private config: LoaderConfiguration;
  private initialized = false;

  constructor(config: Partial<LoaderConfiguration> = {}) {
    this.config = {
      preferReal: true,
      fallbackToMocks: true,
      cacheResults: true,
      timeout: 5000,
      ...config
    };
  }

  /**
   * Initialize the loader with environment detection
   */
  async initialize(): Promise<Result<EnvironmentCapabilities>> {
    if (this.initialized && this.config.cacheResults && this.capabilities) {
      return Success(this.capabilities);
    }

    try {
      this.capabilities = await detectEnvironment({ timeout: this.config.timeout });
      
      // Initialize fixture registry
      await fixtureRegistry.initialize();
      
      this.initialized = true;
      return Success(this.capabilities);
    } catch (error) {
      return Failure(`Failed to initialize environment-aware loader: ${error instanceof Error ? error.message : 'Unknown error'}`);
    }
  }

  /**
   * Load project fixture with environment adaptation
   */
  async loadProjectFixture(
    type: 'java' | 'node' | 'dotnet' | 'python',
    variant?: string,
    options: FixtureLoadOptions = {}
  ): Promise<Result<ProjectFixture>> {
    await this.initialize();

    if (!this.capabilities) {
      return Failure('Environment not initialized');
    }

    try {
      // Find appropriate project fixture
      const fixtures = fixtureRegistry.find({
        type: 'project',
        category: type,
        tags: variant ? [variant] : undefined
      });

      if (fixtures.length === 0) {
        return Failure(`No project fixture found for type: ${type}`);
      }

      const fixture = fixtures[0]; // Use first match
      const result = await fixtureRegistry.load<ProjectFixture>(fixture.id);
      
      if (!result.success) {
        return result;
      }

      // Enhance with metadata based on project type
      const enhanced: ProjectFixture = {
        ...result.data,
        metadata: {
          projectType: type,
          buildTool: this.inferBuildTool(type),
          ...result.data.metadata
        }
      };

      return Success(enhanced);
    } catch (error) {
      return Failure(`Failed to load project fixture: ${error instanceof Error ? error.message : 'Unknown error'}`);
    }
  }

  /**
   * Load Docker-related fixtures based on Docker availability
   */
  async loadDockerFixture(
    fixtureName: string,
    options: FixtureLoadOptions = {}
  ): Promise<Result<any>> {
    await this.initialize();

    if (!this.capabilities) {
      return Failure('Environment not initialized');
    }

    const { forceReal, forceMock, mockBehavior = 'success' } = options;

    // Determine if we should use real Docker or mocks
    const useReal = this.shouldUseReal('docker', forceReal, forceMock);

    if (useReal) {
      // Load real Docker fixture (e.g., Dockerfile)
      const fixtures = fixtureRegistry.find({
        type: 'docker',
        tags: [fixtureName]
      });

      if (fixtures.length > 0) {
        return await fixtureRegistry.load(fixtures[0].id);
      } else {
        // Fallback to mock if no real fixture found
        if (this.config.fallbackToMocks) {
          return Success(unifiedMockFactory.createDockerMock(mockBehavior));
        } else {
          return Failure(`Docker fixture not found: ${fixtureName}`);
        }
      }
    } else {
      // Use mock Docker
      return Success(unifiedMockFactory.createDockerMock(mockBehavior));
    }
  }

  /**
   * Load Kubernetes fixtures based on cluster availability
   */
  async loadKubernetesFixture(
    fixtureName: string,
    environment: string = 'default',
    options: FixtureLoadOptions = {}
  ): Promise<Result<any>> {
    await this.initialize();

    if (!this.capabilities) {
      return Failure('Environment not initialized');
    }

    const { forceReal, forceMock, mockBehavior = 'success' } = options;
    const useReal = this.shouldUseReal('kubernetes', forceReal, forceMock);

    if (useReal) {
      // Load real K8s manifests
      const fixtures = fixtureRegistry.find({
        type: 'k8s',
        category: environment,
        tags: [fixtureName]
      });

      if (fixtures.length > 0) {
        return await fixtureRegistry.load(fixtures[0].id);
      } else {
        if (this.config.fallbackToMocks) {
          const clusterType = this.capabilities!.kubernetes.type as any;
          return Success(unifiedMockFactory.createKubernetesMock(clusterType || 'kind'));
        } else {
          return Failure(`Kubernetes fixture not found: ${fixtureName}`);
        }
      }
    } else {
      // Use mock Kubernetes
      const clusterType = this.capabilities!.kubernetes.available 
        ? (this.capabilities!.kubernetes.type as any)
        : 'unavailable';
      return Success(unifiedMockFactory.createKubernetesMock(clusterType));
    }
  }

  /**
   * Load security scanning fixtures based on Trivy availability
   */
  async loadSecurityFixture(
    scenario: 'clean' | 'vulnerable' | 'critical',
    options: FixtureLoadOptions = {}
  ): Promise<Result<any>> {
    await this.initialize();

    if (!this.capabilities) {
      return Failure('Environment not initialized');
    }

    const { forceReal, forceMock } = options;
    const useReal = this.shouldUseReal('trivy', forceReal, forceMock);

    if (useReal && this.capabilities!.trivy.available) {
      // Load real Trivy scan results (golden files)
      const goldenFile = await goldenFileLoader.loadToolGoldenFile('scan', `image-${scenario}`);
      if (goldenFile.success && goldenFile.data) {
        return Success(goldenFile.data);
      }
    }

    // Use mock security scanner
    const findingsLevel = this.mapScenarioToFindings(scenario);
    return Success(unifiedMockFactory.createTrivyMock(findingsLevel));
  }

  /**
   * Load golden file with environment awareness
   */
  async loadGoldenFile<T>(
    toolName: string,
    fixture: string,
    options: FixtureLoadOptions = {}
  ): Promise<Result<T | null>> {
    await this.initialize();

    // Check if the tool requires specific environment capabilities
    const toolRequirements = this.getToolRequirements(toolName);
    const canRunReal = toolRequirements.every(req => 
      this.capabilities![req].available
    );

    if (canRunReal || options.forceReal) {
      // Load real golden file
      return await goldenFileLoader.loadToolGoldenFile<T>(toolName, fixture, options);
    } else {
      // Tool can't run in this environment, return null or mock data
      if (this.config.fallbackToMocks) {
        // Generate mock golden data based on tool
        const mockData = this.generateMockGoldenData(toolName, fixture);
        return Success(mockData as T);
      } else {
        return Success(null);
      }
    }
  }

  /**
   * Get available fixtures for current environment
   */
  async getAvailableFixtures(): Promise<Result<FixtureMetadata[]>> {
    await this.initialize();

    if (!this.capabilities) {
      return Failure('Environment not initialized');
    }

    return Success(fixtureRegistry.getAvailableFixtures(this.capabilities));
  }

  /**
   * Get environment report
   */
  getEnvironmentReport(): string {
    if (!this.capabilities) {
      return 'Environment not initialized';
    }

    const { createEnvironmentReport } = require('../utilities/environment-detector');
    return createEnvironmentReport(this.capabilities);
  }

  // ================================
  // Private Helper Methods  
  // ================================

  private shouldUseReal(
    service: keyof Omit<EnvironmentCapabilities, 'platform'>,
    forceReal?: boolean,
    forceMock?: boolean
  ): boolean {
    if (forceMock) return false;
    if (forceReal) return true;

    if (!this.capabilities) return false;
    
    return this.config.preferReal && this.capabilities[service].available;
  }

  private inferBuildTool(projectType: string): string {
    switch (projectType) {
      case 'java': return 'maven';
      case 'node': return 'npm';
      case 'dotnet': return 'dotnet';
      case 'python': return 'pip';
      default: return 'unknown';
    }
  }

  private getToolRequirements(toolName: string): Array<keyof Omit<EnvironmentCapabilities, 'platform'>> {
    const requirements: Record<string, Array<keyof Omit<EnvironmentCapabilities, 'platform'>>> = {
      'build-image': ['docker'],
      'push-image': ['docker'],
      'tag-image': ['docker'],
      'deploy': ['kubernetes'],
      'verify-deployment': ['kubernetes'],
      'prepare-cluster': ['kubernetes'],
      'scan': ['trivy'],
      'generate-k8s-manifests': [], // Can run without cluster
      'analyze-repo': [], // No external requirements
      'generate-dockerfile': [], // No external requirements
      'fix-dockerfile': [], // No external requirements
      'workflow': [], // Depends on constituent tools
    };

    return requirements[toolName] || [];
  }

  private mapScenarioToFindings(scenario: string): 'none' | 'low' | 'medium' | 'high' | 'critical' {
    switch (scenario) {
      case 'clean': return 'none';
      case 'vulnerable': return 'medium';
      case 'critical': return 'critical';
      default: return 'none';
    }
  }

  private generateMockGoldenData(toolName: string, fixture: string): unknown {
    // Generate reasonable mock data based on tool type
    const mockDataGenerators: Record<string, (fixture: string) => unknown> = {
      'analyze-repo': (f) => ({
        projectType: f.includes('java') ? 'java' : f.includes('node') ? 'nodejs' : 'unknown',
        buildTool: 'mock',
        dependencies: [],
        recommendations: ['Mock recommendation'],
        ports: [8080],
      }),
      'build-image': (f) => ({
        imageId: `sha256:mock-${f}`,
        tags: [`${f}:latest`],
        success: true,
        logs: ['Mock build log'],
      }),
      'scan': (f) => ({
        summary: { total: 0, critical: 0, high: 0, medium: 0, low: 0 },
        vulnerabilities: [],
      }),
    };

    const generator = mockDataGenerators[toolName];
    return generator ? generator(fixture) : { mock: true, tool: toolName, fixture };
  }
}

/**
 * Global environment-aware loader instance
 */
export const environmentAwareLoader = new EnvironmentAwareFixtureLoader();

/**
 * Convenience functions
 */
export async function loadProjectWithEnvironment(
  type: 'java' | 'node' | 'dotnet' | 'python',
  variant?: string
): Promise<ProjectFixture | null> {
  const result = await environmentAwareLoader.loadProjectFixture(type, variant);
  return result.success ? result.data : null;
}

export async function loadDockerWithEnvironment(
  fixtureName: string,
  options?: FixtureLoadOptions
): Promise<any> {
  const result = await environmentAwareLoader.loadDockerFixture(fixtureName, options);
  return result.success ? result.data : null;
}

export async function loadKubernetesWithEnvironment(
  fixtureName: string,
  environment?: string,
  options?: FixtureLoadOptions
): Promise<any> {
  const result = await environmentAwareLoader.loadKubernetesFixture(fixtureName, environment, options);
  return result.success ? result.data : null;
}