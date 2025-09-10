#!/usr/bin/env tsx
/**
 * MCP Inspector Test Suite Runner
 * MCP Inspector Testing Infrastructure
 */

import { MCPTestRunner } from './infrastructure/test-runner';
import { createBasicToolTests } from './suites/tool-validation/basic-tool-tests';
import { createComprehensiveToolTests } from './suites/tool-validation/comprehensive-tool-tests';
import { createErrorHandlingTests } from './suites/edge-cases/error-handling-tests';
import { createSamplingValidationTests } from './suites/sampling-validation/sampling-tests';
import { createResourceManagementTests } from './suites/resource-management/resource-tests';
import { createLoadTestingTests } from './suites/load-testing/concurrent-tests';
import { createIntegrationFlowTests } from './suites/integration-flows/workflow-tests';
import { createContainerizationWorkflowTests } from './suites/integration-flows/containerization-workflow';
// import { createDeploymentPipelineTests } from './suites/integration-flows/deployment-pipeline'; // Disabled - requires K8s cluster
import { createOrchestratorEventTests } from './suites/orchestrator/event-flow-tests';
import { createPhaseGateTests } from './suites/orchestrator/phase-gate-tests';
import { createSamplingDecisionTests } from './suites/sampling/decision-tests';
import { createArtifactTests } from './suites/resources/artifact-tests';
import { createRemediationTests } from './suites/remediation/loop-tests';

async function main() {
  console.log('🧪 MCP Inspector Test Suite');
  console.log('==========================================\n');

  const testRunner = new MCPTestRunner('./scripts/mcp-start.sh');

  try {
    console.log('🔌 Initializing MCP client connection...');
    await testRunner.initialize();
    console.log('✅ Connected to MCP server\n');

    // Register all test suites
    const basicTests = createBasicToolTests(testRunner);
    const comprehensiveTests = createComprehensiveToolTests(testRunner);
    const errorHandlingTests = createErrorHandlingTests(testRunner);
    const samplingTests = createSamplingValidationTests(testRunner);
    const resourceTests = createResourceManagementTests(testRunner);
    const loadTests = createLoadTestingTests(testRunner);
    const integrationTests = createIntegrationFlowTests(testRunner);
    const containerizationTests = createContainerizationWorkflowTests(testRunner);
    // const deploymentTests = createDeploymentPipelineTests(testRunner); // Disabled - requires K8s cluster
    const orchestratorEventTests = createOrchestratorEventTests(testRunner);
    const phaseGateTests = createPhaseGateTests(testRunner);
    const samplingDecisionTests = createSamplingDecisionTests(testRunner);
    const artifactTests = createArtifactTests(testRunner);
    const remediationTests = createRemediationTests(testRunner);

    // Register tests based on category filter
    const categoryArg = args.find(arg => arg.startsWith('--category='));
    const category = categoryArg?.split('=')[1];

    let testsToRegister = [];
    
    if (category) {
      console.log(`🎯 Filtering tests for category: ${category}\n`);
      switch (category) {
        case 'tool-validation':
          testsToRegister = [...basicTests, ...comprehensiveTests, ...errorHandlingTests];
          break;
        case 'edge-cases':
          testsToRegister = errorHandlingTests;
          break;
        case 'sampling-validation':
          testsToRegister = samplingTests;
          break;
        case 'resource-management':
          testsToRegister = resourceTests;
          break;
        case 'load-testing':
          testsToRegister = loadTests;
          break;
        case 'integration-flows':
          testsToRegister = [...integrationTests, ...containerizationTests]; // deploymentTests disabled - requires K8s
          break;
        case 'orchestrator':
          testsToRegister = [...orchestratorEventTests, ...phaseGateTests];
          break;
        case 'gates':
          testsToRegister = phaseGateTests;
          break;
        case 'remediation':
          testsToRegister = remediationTests;
          break;
        default:
          console.log(`❌ Unknown category: ${category}`);
          process.exit(1);
      }
    } else {
      testsToRegister = [...basicTests, ...comprehensiveTests, ...errorHandlingTests, ...samplingTests, ...resourceTests, ...loadTests, ...integrationTests, ...containerizationTests, /* ...deploymentTests, */ ...orchestratorEventTests, ...phaseGateTests, ...samplingDecisionTests, ...artifactTests, ...remediationTests];
    }

    testsToRegister.forEach(test => testRunner.register(test));
    console.log(`📋 Registered ${testsToRegister.length} test cases\n`);

    // Run all tests
    console.log('🚀 Running test suite...\n');
    const results = await testRunner.run();

    // Display results
    console.log('\n📊 Test Results Summary');
    console.log('=======================');
    console.log(`✅ Passed: ${results.passed}`);
    console.log(`❌ Failed: ${results.failed}`);
    console.log(`⏱️  Total Duration: ${Math.round(results.totalDuration)}ms`);
    console.log(`⚡ Avg Response Time: ${Math.round(results.performance.avgResponseTime)}ms`);
    console.log(`🧠 Max Memory Usage: ${Math.round(results.performance.maxMemoryUsage / 1024)}KB`);
    console.log(`🔄 Total Operations: ${results.performance.totalOperations}`);

    if (results.failed > 0) {
      console.log('\n❌ Failed Tests:');
      results.results
        .filter(r => !r.success)
        .forEach(r => {
          console.log(`  - ${r.testName}: ${r.message}`);
        });
    }

    console.log('\n✨ Test suite completed!\n');

    // Exit with proper code
    process.exit(results.failed === 0 ? 0 : 1);

  } catch (error) {
    console.error('💥 Test suite failed to run:', error);
    process.exit(1);
  } finally {
    await testRunner.cleanup();
  }
}

// Handle CLI arguments
const args = process.argv.slice(2);
if (args.includes('--help') || args.includes('-h')) {
  console.log(`
MCP Inspector Test Runner

Usage: tsx test/mcp-inspector/runner.ts [options]

Options:
  --help, -h           Show this help message
  --category <cat>     Run tests from specific category
  --tag <tag>          Run tests with specific tag  
  --pattern <regex>    Run tests matching name pattern
  --parallel           Run tests in parallel (for load testing)

Categories:
  - tool-validation     Basic tool functionality tests (includes edge cases)
  - edge-cases          Error handling and edge case tests
  - sampling-validation Sampling algorithm tests
  - resource-management Resource system tests
  - load-testing        Concurrent operation tests
  - integration-flows   End-to-end workflow tests (includes real containerization)
  - orchestrator        Orchestrator event and phase tests
  - gates               Phase gate validation tests
  - remediation         Remediation loop and healing tests

Examples:
  tsx test/mcp-inspector/runner.ts
  tsx test/mcp-inspector/runner.ts --category tool-validation
  tsx test/mcp-inspector/runner.ts --tag basic
  tsx test/mcp-inspector/runner.ts --pattern "ping.*"
`);
  process.exit(0);
}

main().catch(console.error);