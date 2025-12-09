#!/usr/bin/env tsx
/**
 * Smoke Test - End-to-End Containerization Journey
 *
 * Tests the core workflow: analyze → generate dockerfile → fix → generate k8s → build → scan → tag → prepare
 */

import { createApp } from '../src/app';
import { join } from 'path';
import { existsSync, mkdirSync, writeFileSync } from 'fs';
import { createLogger } from '../src/lib/logger';
import type { Result } from '../src/types';

const TEST_DIR = join(process.cwd(), 'test/__support__/fixtures/node-express');
const OUTPUT_DIR = join(process.cwd(), '.smoke-test');

interface JourneyStep {
  name: string;
  tool: string;
  params: Record<string, any>;
  skipOnError?: boolean;
}

async function runSmokeTest(): Promise<void> {
  console.log('🚀 Starting end-to-end containerization smoke test...\n');

  // Ensure test directory exists
  if (!existsSync(TEST_DIR)) {
    console.error(`❌ Test fixture not found: ${TEST_DIR}`);
    process.exit(1);
  }

  // Create output directory
  if (!existsSync(OUTPUT_DIR)) {
    mkdirSync(OUTPUT_DIR, { recursive: true });
  }

  // Initialize logger
  const logger = createLogger({
    name: 'smoke-journey',
    level: process.env.DEBUG ? 'debug' : 'info',
  });

  // Initialize runtime
  const runtime = createApp({
    logger,
  });

  const steps: JourneyStep[] = [
    {
      name: 'Analyze Repository',
      tool: 'analyze-repo',
      params: {
        repositoryPath: TEST_DIR,
      },
    },
    {
      name: 'Generate Dockerfile',
      tool: 'generate-dockerfile',
      params: {
        repositoryPath: TEST_DIR,
      },
    },
    {
      name: 'Fix Dockerfile (optional)',
      tool: 'fix-dockerfile',
      params: {
        // Use the existing Dockerfile in the fixture
        path: join(TEST_DIR, 'Dockerfile'),
      },
      skipOnError: true,
    },
    {
      name: 'Generate Kubernetes Manifests',
      tool: 'generate-k8s-manifests',
      params: {
        name: 'smoke-test-app',
        modulePath: TEST_DIR,
        manifestType: 'kubernetes',
      },
    },
    {
      name: 'Build Docker Image',
      tool: 'build-image',
      params: {
        path: TEST_DIR,
        // Use the existing Dockerfile in the fixture (generate-dockerfile returns a plan, doesn't write files)
        dockerfilePath: join(TEST_DIR, 'Dockerfile'),
        imageName: 'smoke-test:latest',
      },
    },
    {
      name: 'Scan Image',
      tool: 'scan-image',
      params: {
        imageId: 'smoke-test:latest',
      },
      skipOnError: true,
    },
    {
      name: 'Tag Image',
      tool: 'tag-image',
      params: {
        imageId: 'smoke-test:latest',
        tag: 'smoke-test:v1.0.0',
      },
    },
    {
      name: 'Prepare Cluster',
      tool: 'prepare-cluster',
      params: {
        namespace: 'smoke-test',
      },
      skipOnError: true,
    },
  ];

  let failedSteps = 0;
  const results: Array<{ step: string; success: boolean; error?: string }> = [];

  for (const step of steps) {
    console.log(`\n⚙️  ${step.name}...`);

    try {
      const result = await runtime.execute(step.tool as any, step.params);

      if (result.ok) {
        console.log(`✅ ${step.name} completed successfully`);
        results.push({ step: step.name, success: true });

        // Save output artifacts
        if (step.tool === 'analyze-repo' && result.data) {
          const outputPath = join(OUTPUT_DIR, 'analysis.json');
          writeFileSync(outputPath, JSON.stringify(result.data, null, 2));
          logger.debug(`Saved analysis to ${outputPath}`);
        } else if (step.tool === 'scan-image' && result.data) {
          const outputPath = join(OUTPUT_DIR, 'scan-results.json');
          writeFileSync(outputPath, JSON.stringify(result.data, null, 2));
          logger.debug(`Saved scan results to ${outputPath}`);
        }
      } else {
        // Format error with guidance if available
        let errorMessage = result.error || 'Unknown error';
        if (result.guidance) {
          errorMessage = `${result.error}\n  💡 Hint: ${result.guidance.hint || ''}\n  🔧 Resolution: ${result.guidance.resolution || ''}`;
        }

        if (step.skipOnError) {
          console.log(`⚠️  ${step.name} failed (optional): ${errorMessage}`);
          results.push({ step: step.name, success: false, error: errorMessage });
        } else {
          console.error(`❌ ${step.name} failed: ${errorMessage}`);
          results.push({ step: step.name, success: false, error: errorMessage });
          failedSteps++;
        }
      }
    } catch (error) {
      const errorMessage = error instanceof Error ? error.message : String(error);
      if (step.skipOnError) {
        console.log(`⚠️  ${step.name} failed (optional): ${errorMessage}`);
        results.push({ step: step.name, success: false, error: errorMessage });
      } else {
        console.error(`❌ ${step.name} failed with exception: ${errorMessage}`);
        results.push({ step: step.name, success: false, error: errorMessage });
        failedSteps++;
      }
    }
  }

  // Clean up
  await runtime.stop();

  // Print summary
  console.log('\n' + '='.repeat(60));
  console.log('📊 SMOKE TEST SUMMARY');
  console.log('='.repeat(60));

  for (const result of results) {
    const icon = result.success ? '✅' : result.error?.includes('optional') ? '⚠️' : '❌';
    console.log(`${icon} ${result.step}${result.error ? `: ${result.error}` : ''}`);
  }

  console.log('='.repeat(60));
  const successCount = results.filter((r) => r.success).length;
  console.log(`\n${successCount}/${results.length} steps completed successfully`);

  if (failedSteps > 0) {
    console.log(`\n❌ Smoke test failed with ${failedSteps} critical failures`);
    process.exit(1);
  } else {
    console.log('\n✅ Smoke test completed successfully!');
    console.log(`   Output saved to: ${OUTPUT_DIR}`);
    process.exit(0);
  }
}

// Run the smoke test
runSmokeTest().catch((error) => {
  console.error('Fatal error during smoke test:', error);
  process.exit(1);
});
