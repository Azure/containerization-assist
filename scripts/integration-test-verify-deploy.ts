/**
 * Integration Test: verify-deploy with Real Kubernetes Deployments
 *
 * Tests the complete flow of:
 * 1. Creating a kind cluster with local registry
 * 2. Building and pushing a test application
 * 3. Deploying application to Kubernetes
 * 4. Running verify-deploy tool to validate deployment
 * 5. Verifying health checks, replicas, and service endpoints
 *
 * Prerequisites:
 * - Docker installed and running
 * - kind installed (brew install kind / choco install kind)
 * - kubectl installed (brew install kubectl)
 *
 * Usage:
 *   npm run build
 *   tsx scripts/integration-test-verify-deploy.ts
 */

import { createToolContext } from '../dist/src/mcp/context.js';
import prepareCluster from '../dist/src/tools/prepare-cluster/tool.js';
import verifyDeployTool from '../dist/src/tools/verify-deploy/tool.js';
import { execSync } from 'child_process';
import { createLogger } from '../dist/src/lib/logger.js';
import { readFileSync, writeFileSync, existsSync } from 'fs';
import { join } from 'path';
import os from 'os';
import { DOCKER_PLATFORMS, DockerPlatform } from '../dist/src/tools/shared/schemas.js';

const logger = createLogger({ name: 'verify-deploy-test', level: 'error' });

/**
 * Test result tracking
 */
interface TestResult {
  name: string;
  passed: boolean;
  message: string;
  details?: Record<string, unknown>;
}

/**
 * Verify a tool is installed
 */
function verifyToolInstalled(toolName: string, versionCommand: string): boolean {
  console.log(`   Checking ${toolName}...`);
  try {
    const output = execSync(versionCommand, { encoding: 'utf-8', stdio: 'pipe' });
    const version = output.split('\n')[0].trim();
    console.log(`   ✅ ${toolName}: ${version}`);
    return true;
  } catch (error) {
    console.log(`   ❌ ${toolName} not found`);
    return false;
  }
}

/**
 * Wait for a condition with timeout
 */
async function waitForCondition(
  description: string,
  condition: () => boolean,
  timeoutMs: number = 60000,
  intervalMs: number = 2000,
): Promise<boolean> {
  const startTime = Date.now();
  let attempts = 0;
  
  while (Date.now() - startTime < timeoutMs) {
    attempts++;
    try {
      if (condition()) {
        return true;
      }
    } catch {
      // Condition check failed, continue waiting
    }
    
    if (attempts % 5 === 0) {
      const elapsed = Math.floor((Date.now() - startTime) / 1000);
      console.log(`      Still waiting for ${description}... (${elapsed}s elapsed)`);
    }
    
    await new Promise((resolve) => setTimeout(resolve, intervalMs));
  }
  
  return false;
}

/**
 * Cleanup all test resources
 * Logs errors to help diagnose issues in CI, but continues cleanup
 */
async function cleanup(registryPort?: string): Promise<void> {
  console.log('\n🧹 Cleaning up resources...\n');
  
  try {
    // Delete Kubernetes resources
    execSync('kubectl delete deployment test-web-app --ignore-not-found=true', { stdio: 'pipe' });
    execSync('kubectl delete service test-web-app --ignore-not-found=true', { stdio: 'pipe' });
    console.log('   ✅ Kubernetes resources deleted');
  } catch (error) {
    const msg = error instanceof Error ? error.message : String(error);
    console.log(`   ⚠️ Kubernetes cleanup failed: ${msg}`);
  }
  
  try {
    // Delete kind cluster
    execSync('kind delete cluster --name containerization-assist', { stdio: 'pipe' });
    console.log('   ✅ Kind cluster deleted');
  } catch (error) {
    const msg = error instanceof Error ? error.message : String(error);
    console.log(`   ⚠️ Kind cluster cleanup failed: ${msg}`);
  }
  
  try {
    // Delete registry container
    execSync('docker rm -f ca-registry', { stdio: 'pipe' });
    console.log('   ✅ Registry container deleted');
  } catch (error) {
    const msg = error instanceof Error ? error.message : String(error);
    console.log(`   ⚠️ Registry cleanup failed: ${msg}`);
  }
  
  try {
    // Clean up test images
    if (registryPort) {
      execSync(`docker rmi -f localhost:${registryPort}/test-health-app:v1.0.0`, { stdio: 'pipe' });
    }
    execSync('docker rmi -f test-health-app:local', { stdio: 'pipe' });
    console.log('   ✅ Test images deleted');
  } catch (error) {
    const msg = error instanceof Error ? error.message : String(error);
    console.log(`   ⚠️ Image cleanup failed: ${msg}`);
  }
}

/**
 * Main test execution
 */
async function main() {
  console.log('🚀 Testing verify-deploy with Real Kubernetes Deployment\n');
  console.log('='.repeat(60));

  const results: TestResult[] = [];
  let registryPort: string | undefined;
  
  // Determine platform
  const envTargetPlatform = process.env.TARGET_PLATFORM;
  let targetPlatform: DockerPlatform;
  
  if (envTargetPlatform && DOCKER_PLATFORMS.includes(envTargetPlatform as DockerPlatform)) {
    targetPlatform = envTargetPlatform as DockerPlatform;
  } else {
    // Auto-detect platform
    try {
      const arch = execSync('uname -m', { encoding: 'utf-8' }).trim();
      targetPlatform = arch === 'arm64' || arch === 'aarch64' ? 'linux/arm64' : 'linux/amd64';
    } catch {
      targetPlatform = 'linux/amd64';
    }
  }
  
  console.log(`\n   Target platform: ${targetPlatform}`);

  try {
    // ─────────────────────────────────────────────────────────────
    // Step 1: Verify Prerequisites
    // ─────────────────────────────────────────────────────────────
    console.log('\n📋 Step 1: Verifying prerequisites...\n');

    const dockerInstalled = verifyToolInstalled('Docker', 'docker --version');
    const kindInstalled = verifyToolInstalled('kind', 'kind --version');
    const kubectlInstalled = verifyToolInstalled('kubectl', 'kubectl version --client');

    if (!dockerInstalled || !kindInstalled || !kubectlInstalled) {
      console.error('\n❌ Missing prerequisites. Install:');
      if (!dockerInstalled) console.error('   - Docker: https://docs.docker.com/get-docker/');
      if (!kindInstalled) console.error('   - kind: brew install kind');
      if (!kubectlInstalled) console.error('   - kubectl: brew install kubectl');
      process.exit(1);
    }

    // Verify test fixtures exist
    const fixturesPath = join(process.cwd(), 'test/fixtures/kubernetes-deployments');
    const healthAppPath = join(fixturesPath, 'health-check-app');
    const manifestPath = join(fixturesPath, 'simple-web-app.yaml');

    if (!existsSync(join(healthAppPath, 'Dockerfile'))) {
      console.error(`\n❌ Missing fixture: ${healthAppPath}/Dockerfile`);
      process.exit(1);
    }
    if (!existsSync(manifestPath)) {
      console.error(`\n❌ Missing fixture: ${manifestPath}`);
      process.exit(1);
    }
    console.log('   ✅ Test fixtures verified');

    // ─────────────────────────────────────────────────────────────
    // Step 2: Prepare Cluster with Local Registry
    // ─────────────────────────────────────────────────────────────
    console.log('\n🏗️  Step 2: Preparing kind cluster with local registry...\n');

    const ctx = createToolContext(logger);
    
    const prepareResult = await prepareCluster.handler({
      targetPlatform,
      environment: 'development',
      namespace: 'default',
      strictPlatformValidation: true,
    }, ctx);

    if (!prepareResult.ok) {
      console.error('   ❌ Cluster preparation failed:', prepareResult.error);
      results.push({
        name: 'Cluster Preparation',
        passed: false,
        message: `Failed: ${prepareResult.error}`,
      });
      throw new Error('Cluster preparation failed');
    }

    const registryUrl = prepareResult.value.localRegistryUrl!;
    registryPort = registryUrl.split(':')[1];
    
    console.log('   ✅ Cluster prepared');
    console.log(`      Registry URL: ${registryUrl}`);
    console.log(`      Registry port: ${registryPort}`);

    results.push({
      name: 'Cluster Preparation',
      passed: true,
      message: 'Kind cluster with local registry created',
      details: { registryUrl },
    });

    // ─────────────────────────────────────────────────────────────
    // Step 3: Build Test Application
    // ─────────────────────────────────────────────────────────────
    console.log('\n📦 Step 3: Building test application...\n');

    const imageTag = `localhost:${registryPort}/test-health-app:v1.0.0`;
    
    try {
      execSync(`docker build -t ${imageTag} ${healthAppPath}`, {
        stdio: 'inherit',
        cwd: process.cwd(),
      });
      console.log('   ✅ Test application built');
      
      results.push({
        name: 'Build Test App',
        passed: true,
        message: 'Health check app built successfully',
        details: { imageTag },
      });
    } catch (error) {
      console.error('   ❌ Build failed');
      results.push({
        name: 'Build Test App',
        passed: false,
        message: 'Docker build failed',
      });
      throw error;
    }

    // ─────────────────────────────────────────────────────────────
    // Step 4: Push to Local Registry
    // ─────────────────────────────────────────────────────────────
    console.log('\n⬆️  Step 4: Pushing to local registry...\n');

    try {
      execSync(`docker push ${imageTag}`, { stdio: 'inherit' });
      console.log('   ✅ Image pushed to registry');
      
      results.push({
        name: 'Push to Registry',
        passed: true,
        message: 'Image pushed to local registry',
      });
    } catch (error) {
      console.error('   ❌ Push failed');
      results.push({
        name: 'Push to Registry',
        passed: false,
        message: 'Docker push failed',
      });
      throw error;
    }

    // ─────────────────────────────────────────────────────────────
    // Step 5: Deploy Application
    // ─────────────────────────────────────────────────────────────
    console.log('\n🚀 Step 5: Deploying application to cluster...\n');

    // Read and update manifest with correct registry URL
    let manifest = readFileSync(manifestPath, 'utf-8');
    manifest = manifest.replace(/localhost:5000/g, `localhost:${registryPort}`);
    
    const tempManifestPath = join(os.tmpdir(), `test-web-app-deployment-${Date.now()}.yaml`);
    writeFileSync(tempManifestPath, manifest);
    
    try {
      execSync(`kubectl apply -f ${tempManifestPath}`, { stdio: 'inherit' });
      console.log('   ✅ Application deployed');
      
      results.push({
        name: 'Deploy to Cluster',
        passed: true,
        message: 'Kubernetes resources created',
      });
    } catch (error) {
      console.error('   ❌ Deployment failed');
      results.push({
        name: 'Deploy to Cluster',
        passed: false,
        message: 'kubectl apply failed',
      });
      throw error;
    }

    // ─────────────────────────────────────────────────────────────
    // Step 6: Wait for Deployment Ready
    // ─────────────────────────────────────────────────────────────
    console.log('\n⏳ Step 6: Waiting for deployment to be ready...\n');

    const deploymentReady = await waitForCondition(
      'deployment ready',
      () => {
        try {
          const status = execSync(
            'kubectl get deployment test-web-app -o jsonpath="{.status.readyReplicas}"',
            { encoding: 'utf-8', stdio: 'pipe' },
          );
          return parseInt(status) >= 2;
        } catch {
          return false;
        }
      },
      120000, // 2 minute timeout
      3000,   // Check every 3 seconds
    );

    if (!deploymentReady) {
      console.error('   ❌ Deployment did not become ready');
      console.log('\n   Pod status:');
      execSync('kubectl get pods -l app=test-web-app', { stdio: 'inherit' });
      console.log('\n   Pod events:');
      execSync('kubectl describe pods -l app=test-web-app | tail -30', { stdio: 'inherit' });
      
      results.push({
        name: 'Deployment Ready',
        passed: false,
        message: 'Deployment did not reach ready state within timeout',
      });
      throw new Error('Deployment not ready');
    }

    console.log('   ✅ Deployment ready (2/2 replicas)');
    
    // Wait for service endpoints to be populated (can take a moment after pods are ready)
    console.log('   ⏳ Waiting for service endpoints...');
    const endpointsReady = await waitForCondition(
      'service endpoints',
      () => {
        try {
          const endpoints = execSync(
            'kubectl get endpoints test-web-app -o jsonpath="{.subsets[0].addresses}"',
            { encoding: 'utf-8', stdio: 'pipe' },
          );
          return endpoints.length > 2; // Non-empty array
        } catch {
          return false;
        }
      },
      30000, // 30 second timeout
      2000,  // Check every 2 seconds
    );
    
    if (endpointsReady) {
      console.log('   ✅ Service endpoints ready');
    } else {
      console.log('   ⚠️ Service endpoints not yet available (will continue)');
    }
    
    results.push({
      name: 'Deployment Ready',
      passed: true,
      message: 'All replicas running and healthy',
    });

    // ─────────────────────────────────────────────────────────────
    // Step 7: Run verify-deploy Tool
    // ─────────────────────────────────────────────────────────────
    console.log('\n🔍 Step 7: Running verify-deploy tool...\n');

    const verifyResult = await verifyDeployTool.handler({
      deploymentName: 'test-web-app',
      namespace: 'default',
      checks: ['pods', 'services', 'health'],
    }, ctx);

    if (!verifyResult.ok) {
      console.error('   ❌ verify-deploy failed:', verifyResult.error);
      results.push({
        name: 'Verify Deploy Tool',
        passed: false,
        message: `Tool error: ${verifyResult.error}`,
      });
      throw new Error('verify-deploy failed');
    }

    const verifyData = verifyResult.value;
    console.log('   ✅ verify-deploy completed');
    console.log(`      Deployment: ${verifyData.deploymentName}`);
    console.log(`      Namespace: ${verifyData.namespace}`);
    console.log(`      Ready: ${verifyData.ready}`);
    console.log(`      Replicas: ${verifyData.status?.readyReplicas}/${verifyData.status?.totalReplicas}`);
    console.log(`      Endpoints: ${verifyData.endpoints?.length || 0}`);
    if (verifyData.summary) {
      console.log(`      Summary: ${verifyData.summary}`);
    }

    results.push({
      name: 'Verify Deploy Tool',
      passed: true,
      message: 'Deployment verification completed',
      details: {
        ready: verifyData.ready,
        replicas: `${verifyData.status?.readyReplicas}/${verifyData.status?.totalReplicas}`,
        endpoints: verifyData.endpoints?.length || 0,
      },
    });

    // ─────────────────────────────────────────────────────────────
    // Step 8: Validate Results
    // ─────────────────────────────────────────────────────────────
    console.log('\n✅ Step 8: Validating results...\n');

    const validations = [
      {
        name: 'Deployment is ready',
        passed: verifyData.ready === true,
        actual: verifyData.ready,
        required: true,
      },
      {
        name: 'All replicas ready',
        passed: verifyData.status?.readyReplicas === 2,
        actual: verifyData.status?.readyReplicas,
        required: true,
      },
      {
        name: 'Service endpoint available',
        passed: (verifyData.endpoints?.length || 0) > 0,
        actual: verifyData.endpoints?.length,
        required: false, // Endpoints may not be immediately available
      },
      {
        name: 'Deployment name correct',
        passed: verifyData.deploymentName === 'test-web-app',
        actual: verifyData.deploymentName,
        required: true,
      },
      {
        name: 'Namespace correct',
        passed: verifyData.namespace === 'default',
        actual: verifyData.namespace,
        required: true,
      },
    ];

    let allRequiredValidationsPassed = true;
    for (const v of validations) {
      if (v.passed) {
        console.log(`   ✅ ${v.name}`);
      } else if (v.required) {
        console.log(`   ❌ ${v.name} (got: ${v.actual})`);
        allRequiredValidationsPassed = false;
      } else {
        console.log(`   ⚠️ ${v.name} (got: ${v.actual}) - optional`);
      }
    }

    results.push({
      name: 'Result Validation',
      passed: allRequiredValidationsPassed,
      message: allRequiredValidationsPassed ? 'All required validations passed' : 'Some required validations failed',
    });

  } catch (error) {
    console.error('\n❌ Test failed with error:', error instanceof Error ? error.message : error);
  } finally {
    // ─────────────────────────────────────────────────────────────
    // Cleanup
    // ─────────────────────────────────────────────────────────────
    await cleanup(registryPort);
  }

  // ─────────────────────────────────────────────────────────────
  // Generate Summary
  // ─────────────────────────────────────────────────────────────
  console.log('\n' + '='.repeat(60));
  console.log('📊 TEST SUMMARY');
  console.log('='.repeat(60));

  const passCount = results.filter((r) => r.passed).length;
  const failCount = results.filter((r) => !r.passed).length;

  console.log(`\n   Total:  ${results.length}`);
  console.log(`   Passed: ${passCount} ✅`);
  console.log(`   Failed: ${failCount} ❌`);
  console.log('\n   Results by step:');

  for (const result of results) {
    const status = result.passed ? '✅ PASS' : '❌ FAIL';
    console.log(`   ${status} ${result.name}`);
    if (!result.passed) {
      console.log(`         ${result.message}`);
    }
  }

  // Write results to JSON
  const resultsJson = {
    total: results.length,
    passed: passCount,
    failed: failCount,
    timestamp: new Date().toISOString(),
    platform: targetPlatform,
    results: results.map((r) => ({
      name: r.name,
      passed: r.passed,
      message: r.message,
      details: r.details,
    })),
  };

  writeFileSync('verify-deploy-test-results.json', JSON.stringify(resultsJson, null, 2));
  console.log('\n   Results written to verify-deploy-test-results.json');

  console.log('\n' + '='.repeat(60));

  if (failCount > 0) {
    console.log('❌ Some tests failed. See above for details.');
    process.exit(1);
  }

  console.log('✅ All tests passed!');
}

main().catch((error) => {
  console.error('❌ Test execution failed:', error);
  cleanup().finally(() => process.exit(1));
});
