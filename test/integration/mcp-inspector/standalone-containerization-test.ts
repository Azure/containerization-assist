#!/usr/bin/env tsx
/**
 * Standalone Containerization Workflow Test
 * Tests our containerization workflow in isolation
 */

import { MCPTestRunner } from './infrastructure/test-runner';
import { createContainerizationWorkflowTests } from './suites/integration-flows/containerization-workflow';
import { createDeploymentPipelineTests } from './suites/integration-flows/deployment-pipeline';

async function runStandaloneTest() {
  console.log('🧪 Standalone Containerization Workflow Test');
  console.log('=============================================\n');

  const testRunner = new MCPTestRunner('./scripts/mcp-start.sh');

  try {
    console.log('🔌 Initializing MCP client connection...');
    await testRunner.initialize();
    console.log('✅ Connected to MCP server\n');

    // Register our integration workflow tests
    const containerizationTests = createContainerizationWorkflowTests(testRunner);
    const deploymentTests = createDeploymentPipelineTests(testRunner);
    
    [...containerizationTests, ...deploymentTests].forEach(test => testRunner.register(test));
    
    console.log(`📋 Registered ${containerizationTests.length + deploymentTests.length} integration workflow test cases\n`);

    // Run tests
    console.log('🚀 Running integration workflow tests...\n');
    const results = await testRunner.run();

    // Display results
    console.log('\n📊 Test Results Summary');
    console.log('=======================');
    console.log(`✅ Passed: ${results.passed}`);
    console.log(`❌ Failed: ${results.failed}`);
    console.log(`⏱️  Total Duration: ${Math.round(results.totalDuration)}ms`);
    console.log(`⚡ Avg Response Time: ${Math.round(results.performance.avgResponseTime)}ms`);

    if (results.failed > 0) {
      console.log('\n❌ Failed Tests:');
      results.results
        .filter(r => !r.success)
        .forEach(r => {
          console.log(`  - ${r.testName}: ${r.message}`);
          if (r.details) {
            console.log(`    Details: ${JSON.stringify(r.details, null, 2).substring(0, 200)}...`);
          }
        });
    } else {
      console.log('\n🎉 All integration workflow tests passed!');
    }

    console.log('\n✨ Test completed!\n');

    // Exit with proper code
    process.exit(results.failed === 0 ? 0 : 1);

  } catch (error) {
    console.error('💥 Test failed to run:', error);
    process.exit(1);
  } finally {
    await testRunner.cleanup();
  }
}

// Run the standalone test
runStandaloneTest().catch(console.error);