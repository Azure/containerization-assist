/**
 * Integration Test: Local Registry with Kind Cluster
 *
 * Tests the complete flow of:
 * 1. Creating a kind cluster with local registry
 * 2. Pushing an image to the local registry
 * 3. Deploying a pod that pulls from localhost:PORT
 * 4. Verifying the pod successfully runs with the image
 */

import { createToolContext } from '../dist/src/mcp/context.js';
import prepareCluster from '../dist/src/tools/prepare-cluster/tool.js';
import { execSync } from 'child_process';
import { writeFileSync } from 'fs';
import { createLogger } from '../dist/src/lib/logger.js';
import { DOCKER_PLATFORMS, DockerPlatform } from '../dist/src/tools/shared/schemas.js';

async function main() {
  console.log('🚀 Testing local registry integration...\n');
  const logger = createLogger({ name: 'containerization-assist' });
  const ctx = createToolContext(logger);
  // Step 1: Prepare cluster (creates kind + local registry)
  console.log('Step 1: Preparing cluster with local registry...');

  const envTargetPlatform = process.env.TARGET_PLATFORM;
  // Validate TARGET_PLATFORM is a DockerPlatform
  if (!envTargetPlatform || !DOCKER_PLATFORMS.includes(envTargetPlatform as DockerPlatform)) {
    console.error(`❌ Invalid TARGET_PLATFORM: ${envTargetPlatform}`);
    process.exit(1);
  }

  const prepareResult = await prepareCluster.handler({
    targetPlatform: envTargetPlatform as DockerPlatform,
    environment: 'development',
    namespace: 'default',
    strictPlatformValidation: true,
  }, ctx);

  if (!prepareResult.ok) {
    console.error('❌ Cluster preparation failed:', prepareResult.error);
    process.exit(1);
  }

  console.log('✅ Cluster prepared');
  console.log('   Registry URL:', prepareResult.value.localRegistryUrl);
  console.log('   Checks:', JSON.stringify(prepareResult.value.checks, null, 2));

  if (!prepareResult.value.localRegistry) {
    console.error('❌ Local registry not created');
    process.exit(1);
  }

  const registry = prepareResult.value.localRegistry;
  console.log('   Registry healthy:', registry.healthy);

  if (!registry.healthy) {
    console.error('❌ Registry is not healthy');
    process.exit(1);
  }

  // Extract registry port from URL (format: localhost:PORT)
  const registryUrl = prepareResult.value.localRegistryUrl!;
  const registryPort = registryUrl.split(':')[1];
  console.log('   Registry port:', registryPort);

  // Step 2: Build a test image and tag as localhost:PORT/test-app:v1.0.0
  console.log('\nStep 2: Building test image...');
  execSync('docker pull busybox:latest', { stdio: 'inherit' });
  execSync(`docker tag busybox:latest localhost:${registryPort}/test-app:v1.0.0`, { stdio: 'inherit' });
  console.log('✅ Test image built');

  // Step 3: Push to local registry
  console.log('\nStep 3: Pushing to local registry...');
  // execute docker push to localhost:PORT
  execSync(`docker push localhost:${registryPort}/test-app:v1.0.0`, { stdio: 'inherit' });

  console.log('✅ Image pushed');
  console.log('Pushed image to localhost:' + registryPort + '/test-app:v1.0.0');

  // Step 4: Verify image in registry catalog
  console.log('\nStep 4: Verifying image in registry...');
  try {
    execSync(`curl -sf http://${registryUrl}/v2/test-app/tags/list`, { stdio: 'inherit' });
    console.log('\n✅ Image verified in registry catalog');
  } catch (error) {
    console.error('❌ Failed to verify image in registry');
    throw error;
  }

  // Step 5: Create and apply test pod manifest
  console.log('\nStep 5: Creating test pod that uses local registry...');
  const podManifest = `
apiVersion: v1
kind: Pod
metadata:
  name: test-registry-pod
  namespace: default
spec:
  containers:
  - name: test-container
    image: localhost:${registryPort}/test-app:v1.0.0
    command: ['sh', '-c', 'echo "Successfully pulled from local registry!" && sleep 30']
  restartPolicy: Never
`;

  writeFileSync('test-pod.yaml', podManifest);
  console.log('✅ Pod manifest created');
  console.log(podManifest);

  // Step 6: Apply pod manifest
  console.log('\nStep 6: Applying pod to cluster...');
  // Wait for default ServiceAccount to be auto-created in the namespace
  for (let i = 0; i < 30; i++) {
    try {
      execSync('kubectl get serviceaccount default -n default', { stdio: 'pipe' });
      break;
    } catch {
      if (i === 29) throw new Error('Timed out waiting for default ServiceAccount');
      await new Promise(r => setTimeout(r, 1000));
    }
  }
  execSync('kubectl apply -f test-pod.yaml', { stdio: 'inherit' });
  console.log('✅ Pod applied');

  // Step 7: Wait for pod to pull image and start
  console.log('\nStep 7: Waiting for pod to pull image...');
  let attempts = 0;
  const maxAttempts = 30;
  let podReady = false;

  while (attempts < maxAttempts && !podReady) {
    try {
      const status = execSync('kubectl get pod test-registry-pod -o jsonpath="{.status.phase}"', {
        encoding: 'utf-8'
      });
      console.log(`   Attempt ${attempts + 1}/${maxAttempts}: Pod status = ${status}`);

      if (status === 'Running' || status === 'Succeeded') {
        podReady = true;
        break;
      }

      // Check for image pull errors
      const events = execSync('kubectl get events --field-selector involvedObject.name=test-registry-pod --sort-by=.lastTimestamp', {
        encoding: 'utf-8'
      });
      if (events.includes('ErrImagePull') || events.includes('ImagePullBackOff')) {
        console.error('❌ Pod failed to pull image from local registry');
        console.log('\nPod events:');
        console.log(events);
        process.exit(1);
      }
    } catch (error) {
      console.log(`   Pod not ready yet (attempt ${attempts + 1})`);
    }

    attempts++;
    execSync('sleep 2');
  }

  if (!podReady) {
    console.error('❌ Pod did not start within timeout');
    console.log('\nPod description:');
    execSync('kubectl describe pod test-registry-pod', { stdio: 'inherit' });
    console.log('\nPod events:');
    execSync('kubectl get events --field-selector involvedObject.name=test-registry-pod', { stdio: 'inherit' });
    process.exit(1);
  }

  console.log('✅ Pod is running - image successfully pulled from localhost:' + registryPort);

  // Step 8: Verify pod logs
  console.log('\nStep 8: Verifying pod logs...');
  try {
    const logs = execSync('kubectl logs test-registry-pod', { encoding: 'utf-8' });
    console.log('Pod logs:');
    console.log(logs);

    if (logs.includes('Successfully pulled from local registry!')) {
      console.log('✅ Pod executed successfully with image from local registry');
    } else {
      console.error('❌ Unexpected pod output');
      process.exit(1);
    }
  } catch (error) {
    console.error('❌ Failed to get pod logs');
    throw error;
  }

  console.log('\n🎉 All tests passed!');
  console.log('✅ Local registry created and accessible');
  console.log('✅ Image pushed to localhost:' + registryPort);
  console.log('✅ Pod successfully pulled image from local registry');
}

main().catch(error => {
  console.error('❌ Test failed:', error);
  process.exit(1);
});
