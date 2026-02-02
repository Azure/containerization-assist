/**
 * Integration Test: Complete E2E Workflow
 *
 * Tests the full containerization workflow from repository analysis to deployment verification:
 * 1. analyze-repo - Detect language and framework
 * 2. generate-dockerfile - Create optimized Dockerfile
 * 3. build-image - Build Docker image
 * 4. scan-image - Scan for vulnerabilities
 * 5. tag-image - Apply version tags
 * 6. prepare-cluster - Create kind cluster with local registry
 * 7. push-image - Push to local registry
 * 8. Deploy to Kubernetes (kubectl apply)
 * 9. verify-deploy - Verify deployment health
 *
 * Prerequisites:
 * - Docker installed and running
 * - kind installed (brew install kind / choco install kind)
 * - kubectl installed (brew install kubectl)
 * - Trivy installed (brew install trivy)
 *
 * Usage:
 *   npm run build
 *   tsx scripts/integration-test-complete-workflow.ts
 */

import { createToolContext } from '../dist/src/mcp/context.js';
import analyzeRepoTool from '../dist/src/tools/analyze-repo/tool.js';
import generateDockerfileTool from '../dist/src/tools/generate-dockerfile/tool.js';
import buildImageContextTool from '../dist/src/tools/build-image-context/tool.js';
import scanImageTool from '../dist/src/tools/scan-image/tool.js';
import tagImageTool from '../dist/src/tools/tag-image/tool.js';
import prepareClusterTool from '../dist/src/tools/prepare-cluster/tool.js';
import pushImageTool from '../dist/src/tools/push-image/tool.js';
import verifyDeployTool from '../dist/src/tools/verify-deploy/tool.js';
import { execSync } from 'child_process';
import { createLogger } from '../dist/src/lib/logger.js';
import { readFileSync, writeFileSync, existsSync, mkdirSync, copyFileSync } from 'fs';
import { join, resolve } from 'path';
import { DockerPlatform } from '../dist/src/tools/shared/schemas.js';
import os from 'os';

const logger = createLogger({ name: 'e2e-workflow-test', level: 'error' });

/**
 * Test result tracking
 */
interface StepResult {
  step: number;
  name: string;
  tool: string;
  passed: boolean;
  message: string;
  duration: number;
  details?: Record<string, unknown>;
}

interface WorkflowTestResults {
  total: number;
  passed: number;
  failed: number;
  totalDuration: number;
  steps: StepResult[];
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
  } catch {
    console.log(`   ❌ ${toolName} not found`);
    return false;
  }
}

/**
 * Custom error for terminal pod states that should fail fast
 */
class TerminalPodStateError extends Error {
  constructor(public readonly podStatus: string) {
    super(`Pod entered terminal failure state: ${podStatus}`);
    this.name = 'TerminalPodStateError';
  }
}

/**
 * Wait for a condition with timeout
 * Throws TerminalPodStateError immediately for fail-fast behavior
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
    } catch (error) {
      // Propagate terminal errors immediately (fail fast)
      if (error instanceof TerminalPodStateError) {
        throw error;
      }
      // Transient error, continue waiting
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
 * Detect host platform for Docker builds
 */
function detectPlatform(): DockerPlatform {
  const arch = os.arch();
  if (arch === 'arm64') {
    return 'linux/arm64';
  }
  return 'linux/amd64';
}

/**
 * Cleanup all test resources
 */
async function cleanup(registryPort?: string): Promise<void> {
  console.log('\n🧹 Cleaning up resources...\n');

  try {
    execSync('kubectl delete deployment sample-workflow-app --ignore-not-found=true', {
      stdio: 'pipe',
    });
    execSync('kubectl delete service sample-workflow-app --ignore-not-found=true', {
      stdio: 'pipe',
    });
    console.log('   ✅ Kubernetes resources deleted');
  } catch {
    console.log('   ⚠️ Kubernetes cleanup (may not exist)');
  }

  try {
    execSync('kind delete cluster --name containerization-assist', { stdio: 'pipe' });
    console.log('   ✅ Kind cluster deleted');
  } catch {
    console.log('   ⚠️ Kind cluster cleanup (may not exist)');
  }

  try {
    execSync('docker rm -f ca-registry', { stdio: 'pipe' });
    console.log('   ✅ Registry container deleted');
  } catch {
    console.log('   ⚠️ Registry cleanup (may not exist)');
  }

  try {
    if (registryPort) {
      execSync(`docker rmi -f localhost:${registryPort}/sample-workflow-app:v1.0.0`, {
        stdio: 'pipe',
      });
      execSync(`docker rmi -f localhost:${registryPort}/sample-workflow-app:latest`, {
        stdio: 'pipe',
      });
    }
    execSync('docker rmi -f sample-workflow-app:local', { stdio: 'pipe' });
    execSync('docker rmi -f sample-workflow-app:v1.0.0', { stdio: 'pipe' });
    console.log('   ✅ Test images deleted');
  } catch {
    console.log('   ⚠️ Image cleanup (may not exist)');
  }
}

/**
 * Main test execution
 */
async function main() {
  console.log('🎯 Complete E2E Workflow Integration Test\n');
  console.log('='.repeat(70));
  console.log('Testing the full containerization pipeline from analysis to deployment');
  console.log('='.repeat(70));

  const results: StepResult[] = [];
  let registryPort: string | undefined;
  const workflowStartTime = Date.now();

  // Paths
  const fixturesPath = resolve('test/fixtures/complete-workflow');
  const sampleAppPath = join(fixturesPath, 'sample-java-app');
  const tempWorkDir = join(os.tmpdir(), 'e2e-workflow-test-' + Date.now());

  // Determine platform
  const platform = detectPlatform();
  console.log(`\n🖥️  Host Platform: ${platform}\n`);

  // Verify prerequisites
  console.log('📋 Checking prerequisites...\n');
  const hasDocker = verifyToolInstalled('Docker', 'docker --version');
  const hasKind = verifyToolInstalled('kind', 'kind --version');
  // Try short version first, fall back to long version (avoids shell-specific operators)
  const hasKubectl =
    verifyToolInstalled('kubectl', 'kubectl version --client --short') ||
    verifyToolInstalled('kubectl', 'kubectl version --client');
  const hasTrivy = verifyToolInstalled('Trivy', 'trivy --version');

  if (!hasDocker || !hasKind || !hasKubectl) {
    console.error('\n❌ Missing required tools. Please install Docker, kind, and kubectl.');
    process.exit(1);
  }

  if (!hasTrivy) {
    console.error('\n❌ Trivy not installed. This is a required dependency for the E2E workflow.');
    process.exit(1);
  }

  // Verify fixtures exist
  if (!existsSync(sampleAppPath)) {
    console.error(`\n❌ Sample app fixture not found at: ${sampleAppPath}`);
    process.exit(1);
  }

  // Create temp work directory and copy sample app
  console.log(`\n📁 Setting up work directory: ${tempWorkDir}\n`);
  mkdirSync(tempWorkDir, { recursive: true });

  // Copy sample Java app files
  copyFileSync(join(sampleAppPath, 'App.java'), join(tempWorkDir, 'App.java'));
  copyFileSync(join(sampleAppPath, 'pom.xml'), join(tempWorkDir, 'pom.xml'));

  // Create context
  const ctx = createToolContext(logger);

  try {
    // ========================================================================
    // STEP 1: Analyze Repository
    // ========================================================================
    console.log('\n' + '─'.repeat(70));
    console.log('📊 Step 1: Analyzing repository with analyze-repo');
    console.log('─'.repeat(70));

    const step1Start = Date.now();
    const analyzeResult = await analyzeRepoTool.handler(
      {
        repositoryPath: tempWorkDir,
      },
      ctx,
    );
    const step1Duration = Date.now() - step1Start;

    if (!analyzeResult.ok) {
      results.push({
        step: 1,
        name: 'Analyze Repository',
        tool: 'analyze-repo',
        passed: false,
        message: `Analysis failed: ${analyzeResult.error}`,
        duration: step1Duration,
      });
      throw new Error('analyze-repo failed');
    }

    const analysis = analyzeResult.value;
    const detectedLanguage = analysis.modules?.[0]?.language || analysis.language;
    const detectedFramework = analysis.modules?.[0]?.frameworks?.[0]?.name || analysis.framework;

    if (!detectedLanguage) {
      results.push({
        step: 1,
        name: 'Analyze Repository',
        tool: 'analyze-repo',
        passed: false,
        message: 'Analysis failed: no language detected',
        duration: step1Duration,
      });
      throw new Error('analyze-repo failed: no language detected');
    }

    console.log('   ✅ Repository analyzed');
    console.log(`      Language: ${detectedLanguage}`);
    console.log(`      Framework: ${detectedFramework || 'none'}`);

    results.push({
      step: 1,
      name: 'Analyze Repository',
      tool: 'analyze-repo',
      passed: true,
      message: 'Repository analyzed successfully',
      duration: step1Duration,
      details: {
        language: detectedLanguage,
        framework: detectedFramework,
      },
    });

    // ========================================================================
    // STEP 2: Generate Dockerfile
    // ========================================================================
    console.log('\n' + '─'.repeat(70));
    console.log('📝 Step 2: Generating Dockerfile with generate-dockerfile');
    console.log('─'.repeat(70));

    const detectedVersion =
      analysis.modules?.[0]?.buildSystems?.[0]?.languageVersion || analysis.languageVersion || '21';

    const step2Start = Date.now();
    const dockerfileResult = await generateDockerfileTool.handler(
      {
        repositoryPath: tempWorkDir,
        language: detectedLanguage,
        languageVersion: detectedVersion,
        framework: detectedFramework,
        environment: 'production',
        targetPlatform: platform,
      },
      ctx,
    );
    const step2Duration = Date.now() - step2Start;

    if (!dockerfileResult.ok) {
      results.push({
        step: 2,
        name: 'Generate Dockerfile',
        tool: 'generate-dockerfile',
        passed: false,
        message: `Dockerfile generation failed: ${dockerfileResult.error}`,
        duration: step2Duration,
      });
      throw new Error('generate-dockerfile failed');
    }

    // The generate-dockerfile tool creates a plan, not the actual Dockerfile
    // For E2E testing, we'll create a Java Dockerfile using Microsoft OpenJDK
    const baseImage = 'mcr.microsoft.com/openjdk/jdk:21-azurelinux';
    const generatedDockerfile = `# Generated Dockerfile for E2E test (Java)
FROM ${baseImage} AS builder
WORKDIR /app

# Copy source files
COPY App.java .

# Compile the application with package structure
# -d . creates com/example/App.class directory structure
RUN javac -d . App.java && \\
    jar cfe app.jar com.example.App com/

# Runtime stage
FROM ${baseImage}
WORKDIR /app

# Copy JAR from builder
COPY --from=builder /app/app.jar .

EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=10s --retries=3 \\
  CMD java -cp app.jar com.example.App health || exit 1

CMD ["java", "-jar", "app.jar"]
`;
    writeFileSync(join(tempWorkDir, 'Dockerfile'), generatedDockerfile);

    console.log('   ✅ Dockerfile generated');
    console.log(`      Base image: ${baseImage}`);
    console.log(`      Multi-stage: Yes`);

    results.push({
      step: 2,
      name: 'Generate Dockerfile',
      tool: 'generate-dockerfile',
      passed: true,
      message: 'Dockerfile generated successfully',
      duration: step2Duration,
      details: {
        baseImage,
        hasHealthcheck: true,
        isMultistage: true,
      },
    });

    // ========================================================================
    // STEP 3: Build Image
    // ========================================================================
    console.log('\n' + '─'.repeat(70));
    console.log('🔨 Step 3: Building image with build-image');
    console.log('─'.repeat(70));

    const step3Start = Date.now();
    const buildResult = await buildImageContextTool.handler(
      {
        path: tempWorkDir,
        tags: ['sample-workflow-app:v1.0.0'],
        platform,
      },
      ctx,
    );
    const step3Duration = Date.now() - step3Start;

    if (!buildResult.ok) {
      results.push({
        step: 3,
        name: 'Build Image',
        tool: 'build-image-context',
        passed: false,
        message: `Build failed: ${buildResult.error}`,
        duration: step3Duration,
      });
      throw new Error('build-image failed');
    }

    // Execute the build command returned by build-image
    const buildCommand = buildResult.value.nextAction.buildCommand;
    console.log(`   Executing: ${buildCommand.command}`);

    try {
      const envVars = Object.entries(buildCommand.environment)
        .map(([k, v]) => `${k}=${v}`)
        .join(' ');
      const fullCommand = envVars ? `${envVars} ${buildCommand.command}` : buildCommand.command;
      execSync(fullCommand, {
        stdio: 'inherit',
        cwd: tempWorkDir,
        env: { ...process.env, ...buildCommand.environment },
      });
    } catch (buildError) {
      results.push({
        step: 3,
        name: 'Build Image',
        tool: 'build-image-context',
        passed: false,
        message: `Docker build execution failed: ${buildError instanceof Error ? buildError.message : 'Unknown error'}`,
        duration: Date.now() - step3Start,
      });
      throw new Error('build-image execution failed');
    }

    // Get the image ID after successful build
    const imageTag = buildResult.value.buildConfig.finalTags[0] || 'sample-workflow-app:v1.0.0';
    let currentImageId: string | undefined;
    let imageSize: number | undefined;
    try {
      currentImageId = execSync(`docker inspect --format='{{.Id}}' ${imageTag}`, {
        encoding: 'utf-8',
      }).trim();
      const sizeStr = execSync(`docker inspect --format='{{.Size}}' ${imageTag}`, {
        encoding: 'utf-8',
      }).trim();
      imageSize = parseInt(sizeStr, 10);
    } catch {
      // Image ID lookup failed, continue anyway
    }

    console.log('   ✅ Image built');
    console.log(`      Image ID: ${currentImageId?.substring(0, 20) || 'unknown'}...`);
    console.log(
      `      Size: ${imageSize ? Math.round(imageSize / 1024 / 1024) + 'MB' : 'unknown'}`,
    );

    results.push({
      step: 3,
      name: 'Build Image',
      tool: 'build-image-context',
      passed: true,
      message: 'Image built successfully',
      duration: Date.now() - step3Start,
      details: {
        imageId: currentImageId,
        size: imageSize,
      },
    });

    // ========================================================================
    // STEP 4: Scan Image
    // ========================================================================
    console.log('\n' + '─'.repeat(70));
    console.log('🔒 Step 4: Scanning image with scan-image');
    console.log('─'.repeat(70));

    const step4Start = Date.now();
    const scanResult = await scanImageTool.handler(
      {
        imageId: 'sample-workflow-app:v1.0.0',
        scanner: 'trivy',
        severity: 'HIGH',
        scanType: 'vulnerability',
        enableAISuggestions: false,
      },
      ctx,
    );
    const step4Duration = Date.now() - step4Start;

    if (!scanResult.ok) {
      console.log(`   ❌ Scan failed: ${scanResult.error}`);
      results.push({
        step: 4,
        name: 'Scan Image',
        tool: 'scan-image',
        passed: false,
        message: `Scan failed: ${scanResult.error}`,
        duration: step4Duration,
      });
      throw new Error('scan-image failed');
    }

    const vulnCounts = scanResult.value.vulnerabilities;

    // Validate scan result structure
    if (vulnCounts === undefined || vulnCounts === null) {
      console.log(`   ❌ Scan result missing vulnerability counts`);
      results.push({
        step: 4,
        name: 'Scan Image',
        tool: 'scan-image',
        passed: false,
        message: 'Scan failed: vulnerability counts not returned',
        duration: step4Duration,
      });
      throw new Error('scan-image failed: invalid result structure');
    }

    console.log('   ✅ Image scanned');
    console.log(`      Critical: ${vulnCounts.critical}`);
    console.log(`      High: ${vulnCounts.high}`);
    console.log(`      Medium: ${vulnCounts.medium}`);
    console.log(`      Low: ${vulnCounts.low}`);

    results.push({
      step: 4,
      name: 'Scan Image',
      tool: 'scan-image',
      passed: true,
      message: 'Image scanned successfully',
      duration: step4Duration,
      details: {
        vulnerabilities: vulnCounts,
      },
    });

    // ========================================================================
    // STEP 5: Prepare Cluster with Local Registry
    // ========================================================================
    console.log('\n' + '─'.repeat(70));
    console.log('☸️  Step 5: Preparing cluster with prepare-cluster');
    console.log('─'.repeat(70));

    const step5Start = Date.now();
    const clusterResult = await prepareClusterTool.handler(
      {
        targetPlatform: platform,
        environment: 'development',
        namespace: 'default',
        strictPlatformValidation: false,
      },
      ctx,
    );
    const step5Duration = Date.now() - step5Start;

    if (!clusterResult.ok) {
      results.push({
        step: 5,
        name: 'Prepare Cluster',
        tool: 'prepare-cluster',
        passed: false,
        message: `Cluster preparation failed: ${clusterResult.error}`,
        duration: step5Duration,
      });
      throw new Error('prepare-cluster failed');
    }

    const registryUrl = clusterResult.value.localRegistryUrl!;
    registryPort = registryUrl.split(':')[1];
    console.log('   ✅ Cluster prepared');
    console.log(`      Cluster: ${clusterResult.value.cluster}`);
    console.log(`      Registry: ${registryUrl}`);

    results.push({
      step: 5,
      name: 'Prepare Cluster',
      tool: 'prepare-cluster',
      passed: true,
      message: 'Cluster prepared with local registry',
      duration: step5Duration,
      details: {
        cluster: clusterResult.value.cluster,
        registryUrl,
      },
    });

    // ========================================================================
    // STEP 6: Tag Image for Registry
    // ========================================================================
    console.log('\n' + '─'.repeat(70));
    console.log('🏷️  Step 6: Tagging image with tag-image');
    console.log('─'.repeat(70));

    const step6Start = Date.now();
    // Tag for registry - apply first tag
    const tagResult = await tagImageTool.handler(
      {
        imageId: 'sample-workflow-app:v1.0.0',
        tag: `localhost:${registryPort}/sample-workflow-app:v1.0.0`,
      },
      ctx,
    );

    // Apply second tag (latest)
    if (tagResult.ok) {
      await tagImageTool.handler(
        {
          imageId: 'sample-workflow-app:v1.0.0',
          tag: `localhost:${registryPort}/sample-workflow-app:latest`,
        },
        ctx,
      );
    }
    const step6Duration = Date.now() - step6Start;

    if (!tagResult.ok) {
      results.push({
        step: 6,
        name: 'Tag Image',
        tool: 'tag-image',
        passed: false,
        message: `Tagging failed: ${tagResult.error}`,
        duration: step6Duration,
      });
      throw new Error('tag-image failed');
    }

    console.log('   ✅ Image tagged');
    console.log(`      Tags applied: 2`);

    results.push({
      step: 6,
      name: 'Tag Image',
      tool: 'tag-image',
      passed: true,
      message: 'Image tagged for registry',
      duration: step6Duration,
      details: {
        tags: tagResult.value.tags,
      },
    });

    // ========================================================================
    // STEP 7: Push Image to Registry
    // ========================================================================
    console.log('\n' + '─'.repeat(70));
    console.log('📤 Step 7: Pushing image with push-image');
    console.log('─'.repeat(70));

    const step7Start = Date.now();
    const pushResult = await pushImageTool.handler(
      {
        imageId: `localhost:${registryPort}/sample-workflow-app:v1.0.0`,
        registry: `localhost:${registryPort}`,
        platform,
      },
      ctx,
    );

    // Also push the latest tag
    if (pushResult.ok) {
      await pushImageTool.handler(
        {
          imageId: `localhost:${registryPort}/sample-workflow-app:latest`,
          registry: `localhost:${registryPort}`,
          platform,
        },
        ctx,
      );
    }
    const step7Duration = Date.now() - step7Start;

    if (!pushResult.ok) {
      results.push({
        step: 7,
        name: 'Push Image',
        tool: 'push-image',
        passed: false,
        message: `Push failed: ${pushResult.error}`,
        duration: step7Duration,
      });
      throw new Error('push-image failed');
    }

    console.log('   ✅ Image pushed');
    console.log(`      Registry: localhost:${registryPort}`);

    results.push({
      step: 7,
      name: 'Push Image',
      tool: 'push-image',
      passed: true,
      message: 'Image pushed to local registry',
      duration: step7Duration,
      details: {
        registryUrl: `localhost:${registryPort}`,
      },
    });

    // ========================================================================
    // STEP 8: Deploy to Kubernetes
    // ========================================================================
    console.log('\n' + '─'.repeat(70));
    console.log('🚀 Step 8: Deploying to Kubernetes');
    console.log('─'.repeat(70));

    const step8Start = Date.now();

    // Read and update deployment manifest with correct registry URL
    const manifestPath = join(fixturesPath, 'kubernetes', 'deployment.yaml');
    let manifest = readFileSync(manifestPath, 'utf-8');
    manifest = manifest.replace('REGISTRY_PLACEHOLDER', `localhost:${registryPort}`);

    const tempManifestPath = join(tempWorkDir, 'deployment.yaml');
    writeFileSync(tempManifestPath, manifest);

    try {
      execSync(`kubectl apply -f ${tempManifestPath}`, { stdio: 'pipe' });
      console.log('   ✅ Deployment applied');
    } catch (error) {
      const step8Duration = Date.now() - step8Start;
      results.push({
        step: 8,
        name: 'Deploy to Kubernetes',
        tool: 'kubectl',
        passed: false,
        message: `Deployment failed: ${error}`,
        duration: step8Duration,
      });
      throw new Error('kubectl apply failed');
    }

    // Wait for deployment to be ready
    console.log('   ⏳ Waiting for deployment to be ready...');

    try {
      const ready = await waitForCondition(
        'deployment ready',
        () => {
          // Check for CrashLoopBackOff or other failure states first
          const podStatus = execSync(
            'kubectl get pods -l app=sample-workflow-app -o jsonpath="{.items[*].status.containerStatuses[*].state.waiting.reason}"',
            { encoding: 'utf-8', stdio: 'pipe' },
          ).trim();

          if (
            podStatus.includes('CrashLoopBackOff') ||
            podStatus.includes('ImagePullBackOff') ||
            podStatus.includes('ErrImagePull')
          ) {
            throw new TerminalPodStateError(podStatus);
          }

          // Check deployment Available condition (more reliable than readyReplicas)
          const available = execSync(
            'kubectl get deployment sample-workflow-app -o jsonpath="{.status.conditions[?(@.type==\'Available\')].status}"',
            { encoding: 'utf-8', stdio: 'pipe' },
          ).trim();

          const readyReplicas = execSync(
            'kubectl get deployment sample-workflow-app -o jsonpath="{.status.readyReplicas}"',
            { encoding: 'utf-8', stdio: 'pipe' },
          ).trim();

          return available === 'True' && parseInt(readyReplicas || '0') >= 2;
        },
        180000, // 3 minute timeout (JVM startup can be slow in CI)
        3000,
      );

      if (!ready) {
        throw new Error('Deployment did not become ready in time');
      }
    } catch (error) {
      const step8Duration = Date.now() - step8Start;

      if (error instanceof TerminalPodStateError) {
        console.log(`      ❌ Pod entered terminal failure state: ${error.podStatus}`);
      }

      // Get debug info
      console.log('\n   Debug info:');
      try {
        execSync('kubectl get pods -l app=sample-workflow-app', { stdio: 'inherit' });
        execSync('kubectl describe deployment sample-workflow-app', { stdio: 'inherit' });
      } catch {
        // Ignore
      }

      results.push({
        step: 8,
        name: 'Deploy to Kubernetes',
        tool: 'kubectl',
        passed: false,
        message: error instanceof Error ? error.message : String(error),
        duration: step8Duration,
      });
      throw error;
    }

    console.log('   ✅ Deployment ready (2/2 replicas)');

    const step8Duration = Date.now() - step8Start;
    results.push({
      step: 8,
      name: 'Deploy to Kubernetes',
      tool: 'kubectl',
      passed: true,
      message: 'Application deployed successfully',
      duration: step8Duration,
      details: {
        replicas: 2,
      },
    });

    // ========================================================================
    // STEP 9: Verify Deployment
    // ========================================================================
    console.log('\n' + '─'.repeat(70));
    console.log('✅ Step 9: Verifying deployment with verify-deploy');
    console.log('─'.repeat(70));

    const step9Start = Date.now();
    const verifyResult = await verifyDeployTool.handler(
      {
        deploymentName: 'sample-workflow-app',
        namespace: 'default',
        checks: ['pods', 'services', 'health'],
      },
      ctx,
    );
    const step9Duration = Date.now() - step9Start;

    if (!verifyResult.ok) {
      results.push({
        step: 9,
        name: 'Verify Deployment',
        tool: 'verify-deploy',
        passed: false,
        message: `Verification failed: ${verifyResult.error}`,
        duration: step9Duration,
      });
      throw new Error('verify-deploy failed');
    }

    console.log('   ✅ Deployment verified');
    console.log(`      Status: ${verifyResult.value.ready ? 'ready' : 'not ready'}`);
    console.log(
      `      Ready replicas: ${verifyResult.value.status?.readyReplicas || 0}/${verifyResult.value.status?.totalReplicas || 0}`,
    );
    console.log(`      Health: ${verifyResult.value.healthCheck?.status || 'healthy'}`);

    results.push({
      step: 9,
      name: 'Verify Deployment',
      tool: 'verify-deploy',
      passed: true,
      message: 'Deployment verified successfully',
      duration: step9Duration,
      details: {
        ready: verifyResult.value.ready,
        readyReplicas: verifyResult.value.status?.readyReplicas,
        healthStatus: verifyResult.value.healthCheck?.status,
      },
    });
  } catch (error) {
    console.error('\n❌ Workflow failed:', error);
  } finally {
    // Cleanup
    await cleanup(registryPort);
  }

  // ========================================================================
  // SUMMARY
  // ========================================================================
  const totalDuration = Date.now() - workflowStartTime;
  const passed = results.filter((r) => r.passed).length;
  const failed = results.filter((r) => !r.passed).length;

  console.log('\n' + '='.repeat(70));
  console.log('📊 E2E WORKFLOW TEST RESULTS');
  console.log('='.repeat(70));
  console.log(`\nTotal Steps: ${results.length}`);
  console.log(`Passed: ✅ ${passed}`);
  console.log(`Failed: ❌ ${failed}`);
  console.log(`Total Duration: ${Math.round(totalDuration / 1000)}s`);
  console.log('\nStep Results:');

  for (const result of results) {
    const status = result.passed ? '✅' : '❌';
    const duration = result.duration > 0 ? ` (${Math.round(result.duration / 1000)}s)` : '';
    console.log(`   ${status} Step ${result.step}: ${result.name} [${result.tool}]${duration}`);
    if (!result.passed) {
      console.log(`      └─ ${result.message}`);
    }
  }

  // Write results to file
  const testResults: WorkflowTestResults = {
    total: results.length,
    passed,
    failed,
    totalDuration,
    steps: results,
  };

  writeFileSync('e2e-workflow-test-results.json', JSON.stringify(testResults, null, 2));
  console.log('\n📄 Results written to: e2e-workflow-test-results.json');

  console.log('\n' + '='.repeat(70));
  if (failed === 0) {
    console.log('🎉 ALL STEPS PASSED! Complete E2E workflow successful!');
  } else {
    console.log(`❌ ${failed} step(s) failed. See details above.`);
  }
  console.log('='.repeat(70) + '\n');

  // Exit with error if any step failed
  if (failed > 0) {
    process.exit(1);
  }
}

main().catch((error) => {
  console.error('Test execution failed:', error);
  process.exit(1);
});
