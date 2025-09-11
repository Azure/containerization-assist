/**
 * Deployment Pipeline Integration Tests
 * Tests Kubernetes deployment workflows and manifest generation
 */

import { TestCase, MCPTestRunner, TestResult } from '../../services/test-runner';
import { KubernetesUtils } from '../../lib/kubernetes-utils';
import { detectEnvironment, getCapabilities } from '../../lib/environment';

export const createDeploymentPipelineTests = (testRunner: MCPTestRunner): TestCase[] => {
  const client = testRunner.getClient();

  const tests: TestCase[] = [
    {
      name: 'deployment-pipeline-k8s-manifests',
      category: 'integration-flows',
      description: 'Complete Kubernetes deployment pipeline with manifest generation',
      tags: ['integration', 'kubernetes', 'deployment', 'manifests'],
      timeout: 90000,
      execute: async (): Promise<TestResult> => {
        const start = performance.now();
        const sessionId = `deployment-${Date.now()}`;
        const env = await detectEnvironment();
        const capabilities = getCapabilities(env);
        let k8sUtils: KubernetesUtils | null = null;

        try {
          // Initialize K8s utils if available
          if (capabilities.canValidateManifests) {
            k8sUtils = new KubernetesUtils();
          }

          // Step 1: Analyze Repository (for context)
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
              message: `Analysis failed: ${analysisResult.error?.message}`
            };
          }

          // Step 2: Generate K8s Manifests
          const manifestResult = await client.callTool({
            name: 'generate-k8s-manifests',
            arguments: {
              sessionId,
              deploymentName: 'test-node-app',
              image: 'node-express:latest',
              namespace: 'test-integration',
              replicas: 2,
              port: 3000,
              environment: 'development'
            }
          });

          if (manifestResult.isError) {
            return {
              success: false,
              duration: performance.now() - start,
              message: `Manifest generation failed: ${manifestResult.error?.message}`
            };
          }

          // Extract manifest content
          let manifestContent: any[] = [];
          let manifestsString = '';
          for (const content of manifestResult.content) {
            if (content.type === 'text' && content.text) {
              try {
                const parsed = JSON.parse(content.text);
                if (parsed.manifests) {
                  if (Array.isArray(parsed.manifests)) {
                    manifestContent = parsed.manifests;
                  } else if (typeof parsed.manifests === 'string') {
                    manifestsString = parsed.manifests;
                    // Parse the string containing the manifests
                    const manifestParts = manifestsString.split('---').filter(part => part.trim());
                    manifestContent = manifestParts.map(part => {
                      try {
                        return JSON.parse(part.trim());
                      } catch {
                        return { rawContent: part.trim() };
                      }
                    });
                  }
                } else if (parsed.resources && Array.isArray(parsed.resources)) {
                  // Use the resources array as indication of successful generation
                  manifestContent = parsed.resources;
                }
              } catch {
                // Try to parse as YAML or single manifest
                if (content.text.includes('apiVersion') && content.text.includes('kind')) {
                  manifestContent = [{ rawYaml: content.text }];
                }
              }
            }
          }

          // Step 3: Validate Manifests (if K8s available)
          let validationResults: any = { skipped: true, reason: 'Kubernetes not available' };
          
          if (capabilities.canValidateManifests && k8sUtils && manifestContent.length > 0) {
            try {
              const validations = await k8sUtils.validateManifests(manifestContent);
              validationResults = {
                skipped: false,
                validations,
                validCount: validations.filter(v => v.valid).length,
                totalCount: validations.length,
                errors: validations.flatMap(v => v.errors),
                warnings: validations.flatMap(v => v.warnings)
              };
            } catch (error) {
              validationResults = {
                skipped: false,
                error: error instanceof Error ? error.message : String(error)
              };
            }
          }

          const responseTime = performance.now() - start;
          const coreWorkflowSuccess = manifestContent.length > 0;
          const validationSuccess = validationResults.skipped || (validationResults.validCount > 0);
          const overallSuccess = coreWorkflowSuccess && validationSuccess;

          return {
            success: overallSuccess,
            duration: responseTime,
            message: overallSuccess
              ? `Deployment pipeline completed successfully${validationResults.skipped ? ' (K8s validation skipped)' : ' with K8s validation'}`
              : 'Deployment pipeline failed - check manifest generation or validation',
            details: {
              manifestsGenerated: manifestContent.length,
              manifestTypes: manifestContent.map(m => m.kind).filter(Boolean),
              validation: validationResults,
              environment: {
                kubernetesAvailable: env.kubernetesAvailable,
                clusterAvailable: env.clusterAvailable,
                canValidateManifests: capabilities.canValidateManifests,
                canDeployToCluster: capabilities.canDeployToCluster
              }
            },
            performance: {
              responseTime,
              memoryUsage: 0,
              operationCount: validationResults.skipped ? 2 : 3
            }
          };

        } finally {
          // Cleanup K8s resources
          if (k8sUtils) {
            await k8sUtils.cleanup();
          }
        }
      }
    },

    {
      name: 'deployment-pipeline-multi-environment',
      category: 'integration-flows',
      description: 'Multi-environment deployment pipeline testing',
      tags: ['integration', 'kubernetes', 'multi-environment'],
      timeout: 120000,
      execute: async (): Promise<TestResult> => {
        const start = performance.now();
        const sessionId = `multi-env-${Date.now()}`;

        try {
          const environments = ['development', 'staging', 'production'];
          const envResults: Record<string, any> = {};

          // Test manifest generation for each environment
          for (const env of environments) {
            const manifestResult = await client.callTool({
              name: 'generate-k8s-manifests',
              arguments: {
                sessionId: `${sessionId}-${env}`,
                deploymentName: 'multi-env-test',
                image: 'node-express:latest',
                namespace: `test-${env}`,
                replicas: env === 'production' ? 3 : (env === 'staging' ? 2 : 1),
                environment: env
              }
            });

            envResults[env] = {
              success: !manifestResult.isError,
              error: manifestResult.isError ? manifestResult.error?.message : undefined
            };

            if (!manifestResult.isError) {
              // Extract manifest content to verify environment-specific config
              for (const content of manifestResult.content) {
                if (content.type === 'text' && content.text) {
                  try {
                    const parsed = JSON.parse(content.text);
                    envResults[env].manifests = parsed.manifests?.length || 0;
                    envResults[env].hasDeployment = parsed.manifests?.some((m: any) => m.kind === 'Deployment') || false;
                  } catch {
                    envResults[env].rawContent = content.text.length;
                  }
                }
              }
            }
          }

          const responseTime = performance.now() - start;
          const successfulEnvironments = Object.values(envResults).filter((r: any) => r.success).length;
          const totalEnvironments = environments.length;
          const overallSuccess = successfulEnvironments === totalEnvironments;

          return {
            success: overallSuccess,
            duration: responseTime,
            message: `Multi-environment pipeline: ${successfulEnvironments}/${totalEnvironments} environments successful`,
            details: {
              environments: envResults,
              successfulEnvironments,
              totalEnvironments,
              successRate: Math.round((successfulEnvironments / totalEnvironments) * 100)
            },
            performance: {
              responseTime,
              memoryUsage: 0,
              operationCount: totalEnvironments
            }
          };

        } catch (error) {
          return {
            success: false,
            duration: performance.now() - start,
            message: `Multi-environment pipeline failed: ${error instanceof Error ? error.message : 'Unknown error'}`
          };
        }
      }
    }
  ];

  return tests;
};