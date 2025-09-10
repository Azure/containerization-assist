/**
 * Containerization Workflow Integration Tests
 * Tests complete containerization workflows using real tools and repositories
 */

import { TestCase, MCPTestRunner, TestResult } from '../../infrastructure/test-runner';
import { DockerUtils } from '../../lib/docker-utils';
import { detectEnvironment, getCapabilities } from '../../lib/environment';

export const createContainerizationWorkflowTests = (testRunner: MCPTestRunner): TestCase[] => {
  const client = testRunner.getClient();

  const tests: TestCase[] = [
    {
      name: 'containerization-workflow-node-express',
      category: 'integration-flows',
      description: 'Complete containerization workflow for Node.js Express application',
      tags: ['integration', 'containerization', 'node', 'docker'],
      timeout: 120000,
      setup: async () => {
        // Setup will be handled in the test execution
      },
      cleanup: async () => {
        // Cleanup will be handled in the test execution
      },
      execute: async (): Promise<TestResult> => {
        const start = performance.now();
        const sessionId = `containerization-node-${Date.now()}`;
        const env = await detectEnvironment();
        const capabilities = getCapabilities(env);
        let dockerUtils: DockerUtils | null = null;

        try {
          // Initialize Docker utils if available
          if (capabilities.canBuildImages) {
            dockerUtils = new DockerUtils();
          }

          // Step 1: Analyze Repository
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

          // Step 2: Generate Dockerfile  
          const dockerfileResult = await client.callTool({
            name: 'generate-dockerfile',
            arguments: {
              sessionId,
              optimization: true,
              multistage: true,
              baseImage: analysisData.recommendedBaseImage || 'node:18-alpine'
            }
          });

          if (dockerfileResult.isError) {
            return {
              success: false,
              duration: performance.now() - start,
              message: `Dockerfile generation failed: ${dockerfileResult.error?.message}`,
              details: { analysisData }
            };
          }

          // Extract Dockerfile content
          let dockerfileContent = '';
          for (const content of dockerfileResult.content) {
            if (content.type === 'text' && content.text) {
              try {
                const parsed = JSON.parse(content.text);
                dockerfileContent = parsed.dockerfile || parsed.content || '';
              } catch {
                if (content.text.includes('FROM ')) {
                  dockerfileContent = content.text;
                }
              }
            }
          }

          // Step 3: Build Docker Image (if Docker available)
          let buildSuccess = false;
          let buildDetails: any = { skipped: true, reason: 'Docker not available' };
          
          if (capabilities.canBuildImages && dockerUtils && dockerfileContent) {
            const imageTag = `test-integration-node-${Date.now()}`;
            
            const buildResult = await dockerUtils.buildImage({
              dockerfile: dockerfileContent,
              context: './test/__support__/fixtures/node-express',
              tag: imageTag
            });
            
            buildSuccess = buildResult.success;
            buildDetails = {
              skipped: false,
              success: buildResult.success,
              imageTag: buildResult.imageTag,
              imageId: buildResult.imageId,
              duration: buildResult.duration,
              error: buildResult.error,
              buildLogSize: buildResult.buildLog.length
            };

            // Step 4: Test Container Run (if build successful)
            if (buildResult.success && capabilities.canRunContainers) {
              const runResult = await dockerUtils.runContainer({
                image: imageTag,
                timeout: 10000,
                detached: true
              });
              
              buildDetails.containerTest = {
                success: runResult.success,
                containerId: runResult.containerId,
                error: runResult.error,
                duration: runResult.duration
              };
            }
          }

          const responseTime = performance.now() - start;
          const coreWorkflowSuccess = analysisData.language && dockerfileContent.length > 0;
          const overallSuccess = coreWorkflowSuccess && (!capabilities.canBuildImages || buildSuccess);

          return {
            success: overallSuccess,
            duration: responseTime,
            message: overallSuccess 
              ? `Node.js containerization workflow completed successfully${buildDetails.skipped ? ' (Docker build skipped)' : ' with Docker build'}`
              : 'Workflow failed - check analysis, Dockerfile generation, or Docker build',
            details: {
              analysisLanguage: analysisData.language,
              analysisFramework: analysisData.framework,
              dockerfileGenerated: !!dockerfileContent,
              dockerfileSize: dockerfileContent.length,
              dockerfilePreview: dockerfileContent ? dockerfileContent.substring(0, 200) + '...' : 'None',
              dockerBuild: buildDetails,
              environment: {
                dockerAvailable: env.dockerAvailable,
                canBuildImages: capabilities.canBuildImages,
                canRunContainers: capabilities.canRunContainers
              }
            },
            performance: {
              responseTime,
              memoryUsage: 0,
              operationCount: buildDetails.skipped ? 2 : 4
            }
          };

        } catch (error) {
          return {
            success: false,
            duration: performance.now() - start,
            message: `Node.js containerization workflow failed: ${error instanceof Error ? error.message : 'Unknown error'}`
          };
        } finally {
          // Cleanup Docker resources
          if (dockerUtils) {
            await dockerUtils.cleanup();
          }
        }
      }
    },

    {
      name: 'containerization-workflow-java-spring-boot',
      category: 'integration-flows',
      description: 'Complete containerization workflow for Java Spring Boot application',
      tags: ['integration', 'containerization', 'java', 'spring-boot', 'docker'],
      timeout: 120000,
      execute: async (): Promise<TestResult> => {
        const start = performance.now();
        const sessionId = `containerization-java-${Date.now()}`;

        try {
          // Step 1: Repository Analysis
          const analysisResult = await client.callTool({
            name: 'analyze-repo',
            arguments: {
              sessionId,
              repoPath: './test/__support__/fixtures/java-spring-boot-maven',
              depth: 4,
              includeTests: false
            }
          });

          if (analysisResult.isError) {
            return {
              success: false,
              duration: performance.now() - start,
              message: `Java analysis failed: ${analysisResult.error?.message}`
            };
          }

          // Step 2: Dockerfile Generation
          const dockerfileResult = await client.callTool({
            name: 'generate-dockerfile',
            arguments: {
              sessionId,
              optimization: true,
              multistage: true,
              baseImage: 'openjdk:17-jdk-alpine'
            }
          });

          const responseTime = performance.now() - start;

          if (dockerfileResult.isError) {
            return {
              success: false,
              duration: responseTime,
              message: `Java Dockerfile generation failed: ${dockerfileResult.error?.message}`
            };
          }

          // Extract and validate Dockerfile
          let dockerfileContent = '';
          for (const content of dockerfileResult.content) {
            if (content.type === 'text' && content.text) {
              try {
                const parsed = JSON.parse(content.text);
                dockerfileContent = parsed.dockerfile || parsed.content || '';
              } catch {
                if (content.text.includes('FROM ')) {
                  dockerfileContent = content.text;
                }
              }
            }
          }

          const hasJavaSpecificElements = dockerfileContent && (
            dockerfileContent.includes('openjdk') || 
            dockerfileContent.includes('maven') ||
            dockerfileContent.includes('.jar') ||
            dockerfileContent.includes('JAVA_')
          );

          const workflowSuccess = dockerfileContent.length > 0 && hasJavaSpecificElements;

          return {
            success: workflowSuccess,
            duration: responseTime,
            message: workflowSuccess
              ? 'Java Spring Boot containerization workflow completed successfully'
              : 'Java workflow incomplete - missing Dockerfile or Java-specific elements',
            details: {
              dockerfileGenerated: !!dockerfileContent,
              hasJavaElements: hasJavaSpecificElements,
              dockerfileSize: dockerfileContent.length,
              dockerfilePreview: dockerfileContent ? dockerfileContent.substring(0, 200) + '...' : 'None'
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
            message: `Java workflow failed: ${error instanceof Error ? error.message : 'Unknown error'}`
          };
        }
      }
    },

    {
      name: 'containerization-workflow-multi-language-comparison',
      category: 'integration-flows',
      description: 'Compare containerization workflows across different languages',
      tags: ['integration', 'comparison', 'multi-language'],
      timeout: 180000,
      execute: async (): Promise<TestResult> => {
        const start = performance.now();
        const sessionId = `comparison-${Date.now()}`;

        try {
          const languageTests = [
            { name: 'node-express', path: './test/__support__/fixtures/node-express', language: 'javascript' },
            { name: 'java-spring-boot-maven', path: './test/__support__/fixtures/java-spring-boot-maven', language: 'java' }
          ];
          const results: Record<string, any> = {};

          for (const test of languageTests) {
            try {
              // Analyze repository
              const analysisResult = await client.callTool({
                name: 'analyze-repo',
                arguments: {
                  sessionId: `${sessionId}-${test.name}`,
                  repoPath: test.path
                }
              });

              // Generate Dockerfile
              const dockerfileResult = await client.callTool({
                name: 'generate-dockerfile',
                arguments: {
                  sessionId: `${sessionId}-${test.name}`,
                  optimization: true,
                  multistage: true
                }
              });

              results[test.name] = {
                language: test.language,
                analysisSuccess: !analysisResult.isError,
                dockerfileSuccess: !dockerfileResult.isError,
                overallSuccess: !analysisResult.isError && !dockerfileResult.isError
              };

            } catch (error) {
              results[test.name] = {
                language: test.language,
                analysisSuccess: false,
                dockerfileSuccess: false,
                overallSuccess: false,
                error: error instanceof Error ? error.message : String(error)
              };
            }
          }

          const totalDuration = performance.now() - start;
          const successfulLanguages = Object.values(results).filter((r: any) => r.overallSuccess).length;
          const totalLanguages = Object.keys(results).length;

          return {
            success: successfulLanguages > 0,
            duration: totalDuration,
            message: `Multi-language comparison: ${successfulLanguages}/${totalLanguages} languages successful`,
            details: {
              results,
              successfulLanguages,
              totalLanguages,
              successRate: Math.round((successfulLanguages / totalLanguages) * 100),
              languagesCovered: Object.values(results).map((r: any) => r.language)
            },
            performance: {
              responseTime: totalDuration,
              memoryUsage: 0,
              operationCount: totalLanguages * 2 // analysis + dockerfile for each
            }
          };

        } catch (error) {
          return {
            success: false,
            duration: performance.now() - start,
            message: `Multi-language comparison failed: ${error instanceof Error ? error.message : 'Unknown error'}`
          };
        }
      }
    }
  ];

  return tests;
};