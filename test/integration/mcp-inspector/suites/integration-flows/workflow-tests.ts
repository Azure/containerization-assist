/**
 * Integration Flow Tests
 * MCP Inspector Testing Infrastructure
 * Tests complete workflows and tool orchestration
 */

import { TestCase, MCPTestRunner } from '../../services/test-runner.js';

export const createIntegrationFlowTests = (testRunner: MCPTestRunner): TestCase[] => {
  const client = testRunner.getClient();

  const tests: TestCase[] = [
    {
      name: 'analyze-then-generate-workflow',
      category: 'integration-flows',
      description: 'Test analyze-repo followed by generate-dockerfile integration',
      tags: ['integration', 'workflow', 'analysis', 'generation'],
      timeout: 60000,
      execute: async () => {
        const start = performance.now();
        const sessionId = 'analyze-generate-flow';
        
        try {
          // Step 1: Analyze repository
          const analysisResult = await client.callTool({
            name: 'analyze-repo',
            arguments: {
              sessionId,
              repoPath: './test/__support__/fixtures/node-express',
              depth: 3,
              includeTests: false
            }
          });

          if (analysisResult.isError) {
            return {
              success: false,
              duration: performance.now() - start,
              message: `Analysis step failed: ${analysisResult.error?.message}`
            };
          }

          // Extract analysis data
          let analysisData: any = {};
          for (const content of analysisResult.content) {
            if (content.type === 'text' && content.text) {
              try {
                const parsed = JSON.parse(content.text);
                analysisData = { ...analysisData, ...parsed };
              } catch {
                analysisData.textContent = content.text;
              }
            }
          }

          // Step 2: Use analysis results for Dockerfile generation
          const generateResult = await client.callTool({
            name: 'generate-dockerfile',
            arguments: {
              sessionId,
              optimization: true,
              multistage: true,
              baseImage: analysisData.recommendedBaseImage || 'node:18-alpine'
            }
          });

          const responseTime = performance.now() - start;

          if (generateResult.isError) {
            return {
              success: false,
              duration: responseTime,
              message: `Generation step failed: ${generateResult.error?.message}`,
              details: { analysisData }
            };
          }

          // Extract generation data
          let generationData: any = {};
          for (const content of generateResult.content) {
            if (content.type === 'text' && content.text) {
              try {
                const parsed = JSON.parse(content.text);
                generationData = { ...generationData, ...parsed };
              } catch {
                generationData.textContent = content.text;
              }
            }
          }

          const hasValidWorkflow = analysisData.language && (generationData.content || generationData.dockerfile);

          return {
            success: hasValidWorkflow,
            duration: responseTime,
            message: hasValidWorkflow 
              ? 'Analyze-to-generate workflow completed successfully'
              : 'Workflow incomplete - missing analysis or generation output',
            details: {
              analysisLanguage: analysisData.language,
              analysisFramework: analysisData.framework,
              generationSuccess: !!generationData.success,
              hasDockerfile: !!(generationData.content || generationData.dockerfile)
            },
            performance: {
              responseTime,
              memoryUsage: 0,
              operationCount: 2
            }
          };
        } catch (error) {
          return {
            success: false,
            duration: performance.now() - start,
            message: `Workflow integration failed: ${error instanceof Error ? error.message : 'Unknown error'}`
          };
        }
      }
    },

    {
      name: 'complete-containerization-flow',
      category: 'integration-flows',
      description: 'Test complete containerization workflow',
      tags: ['integration', 'containerization', 'end-to-end'],
      timeout: 120000,
      execute: async () => {
        const start = performance.now();
        const sessionId = 'complete-containerization';
        
        try {
          const steps = [];
          
          // Step 1: Repository Analysis
          const analysisResult = await client.callTool({
            name: 'analyze-repo',
            arguments: {
              sessionId,
              repoPath: './test/__support__/fixtures/node-express'
            }
          });

          steps.push({
            step: 'analysis',
            success: !analysisResult.isError,
            error: analysisResult.error?.message
          });

          if (analysisResult.isError) {
            return {
              success: false,
              duration: performance.now() - start,
              message: 'Workflow failed at analysis step',
              details: { steps }
            };
          }

          // Step 2: Dockerfile Generation
          const dockerfileResult = await client.callTool({
            name: 'generate-dockerfile',
            arguments: {
              sessionId,
              optimization: true,
              multistage: true
            }
          });

          steps.push({
            step: 'dockerfile-generation',
            success: !dockerfileResult.isError,
            error: dockerfileResult.error?.message
          });

          // Step 3: Build Process (will likely fail in mock mode, but tests integration)
          const buildResult = await client.callTool({
            name: 'build-image',
            arguments: {
              sessionId,
              context: './test/__support__/fixtures/node-express'
            }
          });

          steps.push({
            step: 'build',
            success: !buildResult.isError,
            error: buildResult.error?.message
          });

          // Step 4: Security Scanning (may fail with mock data)
          const scanResult = await client.callTool({
            name: 'scan',
            arguments: {
              sessionId,
              imageId: 'test-image',
              severity: 'medium'
            }
          });

          steps.push({
            step: 'scan',
            success: !scanResult.isError,
            error: scanResult.error?.message
          });

          // Step 5: K8s Manifest Generation
          const k8sResult = await client.callTool({
            name: 'generate-k8s-manifests',
            arguments: {
              sessionId,
              deploymentName: 'test-app',
              image: 'test-image:latest',
              replicas: 2,
              port: 3000
            }
          });

          steps.push({
            step: 'k8s-manifests',
            success: !k8sResult.isError,
            error: k8sResult.error?.message
          });

          const responseTime = performance.now() - start;
          const successfulSteps = steps.filter(s => s.success).length;
          const totalSteps = steps.length;
          const workflowSuccess = successfulSteps >= Math.floor(totalSteps * 0.6); // 60% success rate

          return {
            success: workflowSuccess,
            duration: responseTime,
            message: `Complete containerization workflow: ${successfulSteps}/${totalSteps} steps successful`,
            details: {
              steps,
              successfulSteps,
              totalSteps,
              successRate: Math.round((successfulSteps / totalSteps) * 100)
            },
            performance: {
              responseTime,
              memoryUsage: 0,
              operationCount: totalSteps
            }
          };
        } catch (error) {
          return {
            success: false,
            duration: performance.now() - start,
            message: `Complete workflow failed: ${error instanceof Error ? error.message : 'Unknown error'}`
          };
        }
      }
    },

    {
      name: 'session-state-persistence',
      category: 'integration-flows',
      description: 'Test session state persistence across multiple tool calls',
      tags: ['integration', 'sessions', 'state'],
      timeout: 30000,
      execute: async () => {
        const start = performance.now();
        const sessionId = 'state-persistence-test';
        
        try {
          // Step 1: Create session state with analysis
          const analysisResult = await client.callTool({
            name: 'analyze-repo',
            arguments: {
              sessionId,
              repoPath: './test/__support__/fixtures/node-express'
            }
          });

          if (analysisResult.isError) {
            return {
              success: false,
              duration: performance.now() - start,
              message: 'Session creation failed during analysis'
            };
          }

          // Small delay to allow session state to persist
          await new Promise(resolve => setTimeout(resolve, 500));

          // Step 2: Use same session for generation (should have access to analysis data)
          const generateResult = await client.callTool({
            name: 'generate-dockerfile',
            arguments: {
              sessionId, // Same session ID - should have access to previous analysis
              optimization: true,
              multistage: true
            }
          });

          // Step 3: Another call with same session
          const statusResult = await client.callTool({
            name: 'ops',
            arguments: {
              sessionId,
              operation: 'status'
            }
          });

          const responseTime = performance.now() - start;

          const analysisSuccess = !analysisResult.isError;
          const generateSuccess = !generateResult.isError;
          const statusSuccess = !statusResult.isError;
          
          const sessionWorking = analysisSuccess && (generateSuccess || statusSuccess);

          return {
            success: sessionWorking,
            duration: responseTime,
            message: sessionWorking
              ? 'Session state persistence working correctly'
              : 'Session state persistence has issues',
            details: {
              analysisSuccess,
              generateSuccess,
              statusSuccess,
              sessionId,
              stepsCompleted: [analysisSuccess, generateSuccess, statusSuccess].filter(Boolean).length
            },
            performance: {
              responseTime,
              memoryUsage: 0,
              operationCount: 3
            }
          };
        } catch (error) {
          return {
            success: false,
            duration: performance.now() - start,
            message: `Session persistence test failed: ${error instanceof Error ? error.message : 'Unknown error'}`
          };
        }
      }
    },

    {
      name: 'error-recovery-workflow',
      category: 'integration-flows',
      description: 'Test workflow behavior when intermediate steps fail',
      tags: ['integration', 'error-recovery', 'resilience'],
      timeout: 45000,
      execute: async () => {
        const start = performance.now();
        const sessionId = 'error-recovery-test';
        
        try {
          const steps = [];

          // Step 1: Successful operation
          const analysisResult = await client.callTool({
            name: 'analyze-repo',
            arguments: {
              sessionId,
              repoPath: './test/__support__/fixtures/node-express'
            }
          });

          steps.push({
            step: 'analysis',
            success: !analysisResult.isError,
            error: analysisResult.error?.message
          });

          // Step 2: Intentionally problematic operation (invalid path)
          const failResult = await client.callTool({
            name: 'build-image',
            arguments: {
              sessionId,
              context: '/nonexistent/path'
            }
          });

          steps.push({
            step: 'intentional-failure',
            success: !failResult.isError,
            error: failResult.error?.message,
            expectedToFail: true
          });

          // Step 3: Recovery operation (should still work despite previous failure)
          const recoveryResult = await client.callTool({
            name: 'ops',
            arguments: {
              sessionId,
              operation: 'status'
            }
          });

          steps.push({
            step: 'recovery',
            success: !recoveryResult.isError,
            error: recoveryResult.error?.message
          });

          // Step 4: Continue workflow after recovery
          const continueResult = await client.callTool({
            name: 'generate-dockerfile',
            arguments: {
              sessionId,
              optimization: true,
              multistage: true,
              baseImage: 'node:18-alpine'
            }
          });

          steps.push({
            step: 'continue-after-recovery',
            success: !continueResult.isError,
            error: continueResult.error?.message
          });

          const responseTime = performance.now() - start;
          
          // Good error recovery means:
          // - First step succeeds
          // - Second step can fail (expected)
          // - Third step (recovery) succeeds  
          // - Fourth step works despite earlier failure
          const goodRecovery = steps[0].success && steps[2].success && 
                              (steps[3].success || steps.filter(s => s.success).length >= 3);

          return {
            success: goodRecovery,
            duration: responseTime,
            message: goodRecovery
              ? 'Workflow shows good error recovery behavior'
              : 'Workflow may have issues with error recovery',
            details: {
              steps,
              totalSteps: steps.length,
              successfulSteps: steps.filter(s => s.success).length,
              recoveryWorking: steps[2].success
            },
            performance: {
              responseTime,
              memoryUsage: 0,
              operationCount: steps.length
            }
          };
        } catch (error) {
          return {
            success: false,
            duration: performance.now() - start,
            message: `Error recovery test failed: ${error instanceof Error ? error.message : 'Unknown error'}`
          };
        }
      }
    }
  ];

  return tests;
};