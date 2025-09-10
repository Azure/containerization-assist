/**
 * Environment Detection for Integration Testing
 * Detects available tools and services for testing
 */

import { DockerUtils } from './docker-utils';
import { KubernetesUtils } from './kubernetes-utils';

export interface TestEnvironment {
  dockerAvailable: boolean;
  kubernetesAvailable: boolean;
  clusterAvailable: boolean;
  ci: boolean;
  platform: NodeJS.Platform;
}

export interface EnvironmentCapabilities {
  canBuildImages: boolean;
  canRunContainers: boolean;
  canValidateManifests: boolean;
  canDeployToCluster: boolean;
  skipSlowTests: boolean;
}

/**
 * Detect the current test environment capabilities
 */
export async function detectEnvironment(): Promise<TestEnvironment> {
  const [dockerAvailable, kubernetesAvailable, clusterAvailable] = await Promise.all([
    DockerUtils.isDockerAvailable(),
    KubernetesUtils.isKubernetesAvailable(),
    KubernetesUtils.isClusterAvailable()
  ]);

  return {
    dockerAvailable,
    kubernetesAvailable,
    clusterAvailable,
    ci: process.env.CI === 'true',
    platform: process.platform
  };
}

/**
 * Determine what capabilities are available based on environment
 */
export function getCapabilities(env: TestEnvironment): EnvironmentCapabilities {
  return {
    canBuildImages: env.dockerAvailable,
    canRunContainers: env.dockerAvailable && !env.ci, // Avoid running containers in CI
    canValidateManifests: env.kubernetesAvailable,
    canDeployToCluster: env.clusterAvailable && process.env.ALLOW_CLUSTER_DEPLOY === 'true',
    skipSlowTests: env.ci || process.env.SKIP_SLOW_TESTS === 'true'
  };
}

/**
 * Create a test skip condition based on environment requirements
 */
export function createSkipCondition(requirements: {
  docker?: boolean;
  kubernetes?: boolean;
  cluster?: boolean;
  notCI?: boolean;
}) {
  return async (): Promise<{ skip: boolean; reason?: string }> => {
    const env = await detectEnvironment();
    const capabilities = getCapabilities(env);

    if (requirements.docker && !env.dockerAvailable) {
      return { skip: true, reason: 'Docker not available' };
    }

    if (requirements.kubernetes && !env.kubernetesAvailable) {
      return { skip: true, reason: 'Kubernetes not available' };
    }

    if (requirements.cluster && !env.clusterAvailable) {
      return { skip: true, reason: 'Kubernetes cluster not available' };
    }

    if (requirements.notCI && env.ci) {
      return { skip: true, reason: 'Skipped in CI environment' };
    }

    return { skip: false };
  };
}

/**
 * Environment-aware test wrapper
 */
export function environmentalTest<T>(
  requirements: {
    docker?: boolean;
    kubernetes?: boolean;
    cluster?: boolean;
    notCI?: boolean;
  },
  testFn: (env: TestEnvironment, capabilities: EnvironmentCapabilities) => Promise<T>
) {
  return async (): Promise<T | { skip: true; reason: string }> => {
    const skipCheck = await createSkipCondition(requirements)();
    
    if (skipCheck.skip) {
      return { skip: true, reason: skipCheck.reason! };
    }

    const env = await detectEnvironment();
    const capabilities = getCapabilities(env);

    return testFn(env, capabilities);
  };
}

/**
 * Log environment information for debugging
 */
export async function logEnvironmentInfo(): Promise<void> {
  const env = await detectEnvironment();
  const capabilities = getCapabilities(env);

  console.log('🌍 Test Environment Information:');
  console.log(`  Docker Available: ${env.dockerAvailable ? '✅' : '❌'}`);
  console.log(`  Kubernetes Available: ${env.kubernetesAvailable ? '✅' : '❌'}`);
  console.log(`  Cluster Available: ${env.clusterAvailable ? '✅' : '❌'}`);
  console.log(`  CI Environment: ${env.ci ? '✅' : '❌'}`);
  console.log(`  Platform: ${env.platform}`);
  console.log('');
  console.log('🚀 Capabilities:');
  console.log(`  Can Build Images: ${capabilities.canBuildImages ? '✅' : '❌'}`);
  console.log(`  Can Run Containers: ${capabilities.canRunContainers ? '✅' : '❌'}`);
  console.log(`  Can Validate Manifests: ${capabilities.canValidateManifests ? '✅' : '❌'}`);
  console.log(`  Can Deploy to Cluster: ${capabilities.canDeployToCluster ? '✅' : '❌'}`);
  console.log(`  Skip Slow Tests: ${capabilities.skipSlowTests ? '✅' : '❌'}`);
  console.log('');
}