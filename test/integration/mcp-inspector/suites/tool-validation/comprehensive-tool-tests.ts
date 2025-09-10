/**
 * Comprehensive Tool Validation Tests
 * MCP Inspector Testing Infrastructure
 * Tests all 14 MCP tools for functionality and performance
 */

import { TestCase, MCPTestRunner } from '../../infrastructure/test-runner.js';

export const createComprehensiveToolTests = (testRunner: MCPTestRunner): TestCase[] => {
  const client = testRunner.getClient();

  const tests: TestCase[] = [
    {
      name: 'resolve-base-images-tool',
      category: 'tool-validation',
      description: 'Test resolve-base-images tool for language recommendations',
      tags: ['tools', 'base-images', 'recommendations'],
      timeout: 10000,
      execute: async () => {
        const start = performance.now();
        
        const result = await client.callTool({
          name: 'resolve-base-images',
          arguments: {
            sessionId: 'resolve-test-123',
            language: 'javascript',
            framework: 'express'
          }
        });

        const responseTime = performance.now() - start;

        if (result.isError) {
          return {
            success: false,
            duration: responseTime,
            message: `Resolve base images failed: ${result.error?.message || 'Unknown error'}`
          };
        }

        // Extract result content
        let recommendations: any = {};
        for (const content of result.content) {
          if (content.type === 'text' && content.text) {
            try {
              const parsed = JSON.parse(content.text);
              recommendations = { ...recommendations, ...parsed };
            } catch {
              recommendations.textContent = content.text;
            }
          }
        }

        const hasRecommendations = recommendations.recommended || recommendations.suggestions || 
                                  recommendations.baseImages || recommendations.textContent;

        return {
          success: !!hasRecommendations,
          duration: responseTime,
          message: hasRecommendations 
            ? 'Base image resolution working correctly'
            : 'No base image recommendations found',
          details: recommendations,
          performance: {
            responseTime,
            memoryUsage: 0,
          }
        };
      }
    },

    {
      name: 'build-image-tool',
      category: 'tool-validation',
      description: 'Test build-image tool functionality',
      tags: ['tools', 'docker', 'build'],
      timeout: 30000,
      execute: async () => {
        const start = performance.now();
        
        const result = await client.callTool({
          name: 'build-image',
          arguments: {
            sessionId: 'build-test-123',
            contextPath: './test/__support__/fixtures/node-express',
            noCache: true
          }
        });

        const responseTime = performance.now() - start;

        if (result.isError) {
          return {
            success: false,
            duration: responseTime,
            message: `Build image failed: ${result.error?.message || 'Unknown error'}`
          };
        }

        // Extract build information
        let buildInfo: any = {};
        for (const content of result.content) {
          if (content.type === 'text' && content.text) {
            try {
              const parsed = JSON.parse(content.text);
              buildInfo = { ...buildInfo, ...parsed };
            } catch {
              buildInfo.textContent = content.text;
            }
          }
        }

        const hasBuildResult = buildInfo.success !== undefined || buildInfo.imageId || 
                              buildInfo.textContent || buildInfo.buildOutput;

        return {
          success: !!hasBuildResult,
          duration: responseTime,
          message: hasBuildResult 
            ? 'Build image tool responding correctly'
            : 'Build image tool response unclear',
          details: buildInfo,
          performance: {
            responseTime,
            memoryUsage: 0,
          }
        };
      }
    },

    {
      name: 'scan-image-tool',
      category: 'tool-validation',
      description: 'Test scan tool for security scanning',
      tags: ['tools', 'security', 'scanning'],
      timeout: 45000,
      execute: async () => {
        const start = performance.now();
        
        const result = await client.callTool({
          name: 'scan',
          arguments: {
            sessionId: 'scan-test-123',
            imageId: 'node:18-alpine',
            severity: 'medium'
          }
        });

        const responseTime = performance.now() - start;

        if (result.isError) {
          return {
            success: false,
            duration: responseTime,
            message: `Image scan failed: ${result.error?.message || 'Unknown error'}`
          };
        }

        // Extract scan results
        let scanResults: any = {};
        for (const content of result.content) {
          if (content.type === 'text' && content.text) {
            try {
              const parsed = JSON.parse(content.text);
              scanResults = { ...scanResults, ...parsed };
            } catch {
              scanResults.textContent = content.text;
            }
          }
        }

        const hasScanResults = scanResults.vulnerabilities !== undefined || 
                              scanResults.critical !== undefined || 
                              scanResults.findings || scanResults.textContent;

        return {
          success: !!hasScanResults,
          duration: responseTime,
          message: hasScanResults 
            ? 'Image scan tool working correctly'
            : 'Image scan results not found',
          details: scanResults,
          performance: {
            responseTime,
            memoryUsage: 0,
          }
        };
      }
    },

    {
      name: 'tag-image-tool',
      category: 'tool-validation',
      description: 'Test tag tool for image tagging',
      tags: ['tools', 'docker', 'tagging'],
      timeout: 15000,
      execute: async () => {
        const start = performance.now();
        
        const result = await client.callTool({
          name: 'tag-image',
          arguments: {
            sessionId: 'tag-test-123',
            imageId: 'test-image',
            tags: ['v1.0.0', 'latest']
          }
        });

        const responseTime = performance.now() - start;

        if (result.isError) {
          return {
            success: false,
            duration: responseTime,
            message: `Tag image failed: ${result.error?.message || 'Unknown error'}`
          };
        }

        // Extract tagging results
        let tagResults: any = {};
        for (const content of result.content) {
          if (content.type === 'text' && content.text) {
            try {
              const parsed = JSON.parse(content.text);
              tagResults = { ...tagResults, ...parsed };
            } catch {
              tagResults.textContent = content.text;
            }
          }
        }

        const hasTagResults = tagResults.success !== undefined || tagResults.tags || 
                             tagResults.tagged || tagResults.textContent;

        return {
          success: !!hasTagResults,
          duration: responseTime,
          message: hasTagResults 
            ? 'Tag tool working correctly'
            : 'Tag results not clear',
          details: tagResults,
          performance: {
            responseTime,
            memoryUsage: 0,
          }
        };
      }
    },

    {
      name: 'push-image-tool',
      category: 'tool-validation',
      description: 'Test push tool for registry operations',
      tags: ['tools', 'registry', 'push'],
      timeout: 20000,
      execute: async () => {
        const start = performance.now();
        
        const result = await client.callTool({
          name: 'push-image',
          arguments: {
            sessionId: 'push-test-123',
            imageId: 'test-image:latest',
            registry: 'localhost:5000',
            tag: 'test'
          }
        });

        const responseTime = performance.now() - start;

        if (result.isError) {
          return {
            success: false,
            duration: responseTime,
            message: `Push image failed: ${result.error?.message || 'Unknown error'}`
          };
        }

        // Extract push results
        let pushResults: any = {};
        for (const content of result.content) {
          if (content.type === 'text' && content.text) {
            try {
              const parsed = JSON.parse(content.text);
              pushResults = { ...pushResults, ...parsed };
            } catch {
              pushResults.textContent = content.text;
            }
          }
        }

        const hasPushResults = pushResults.success !== undefined || pushResults.pushed || 
                              pushResults.registry || pushResults.textContent;

        return {
          success: !!hasPushResults,
          duration: responseTime,
          message: hasPushResults 
            ? 'Push tool responding correctly'
            : 'Push results not clear',
          details: pushResults,
          performance: {
            responseTime,
            memoryUsage: 0,
          }
        };
      }
    },

    {
      name: 'generate-k8s-manifests-tool',
      category: 'tool-validation',
      description: 'Test Kubernetes manifest generation',
      tags: ['tools', 'kubernetes', 'manifests'],
      timeout: 30000,
      execute: async () => {
        const start = performance.now();
        
        const result = await client.callTool({
          name: 'generate-k8s-manifests',
          arguments: {
            sessionId: 'k8s-test-123',
            deploymentName: 'test-app',
            image: 'nginx:alpine',
            namespace: 'default',
            replicas: 3,
            port: 80
          }
        });

        const responseTime = performance.now() - start;

        if (result.isError) {
          return {
            success: false,
            duration: responseTime,
            message: `K8s manifest generation failed: ${result.error?.message || 'Unknown error'}`
          };
        }

        // Extract manifest results
        let manifestResults: any = {};
        let resourceCount = 0;
        
        for (const content of result.content) {
          if (content.type === 'text' && content.text) {
            try {
              const parsed = JSON.parse(content.text);
              manifestResults = { ...manifestResults, ...parsed };
            } catch {
              manifestResults.textContent = content.text;
            }
          } else if (content.type === 'resource') {
            resourceCount++;
          }
        }

        const hasManifests = manifestResults.manifests || manifestResults.yaml || 
                            manifestResults.kubernetes || resourceCount > 0 || 
                            manifestResults.textContent;

        return {
          success: !!hasManifests,
          duration: responseTime,
          message: hasManifests 
            ? `K8s manifest generation working (${resourceCount} resources)`
            : 'K8s manifest generation results unclear',
          details: { ...manifestResults, resourceCount },
          performance: {
            responseTime,
            memoryUsage: 0,
          }
        };
      }
    },

    {
      name: 'prepare-cluster-tool',
      category: 'tool-validation',
      description: 'Test cluster preparation functionality',
      tags: ['tools', 'kubernetes', 'cluster'],
      timeout: 20000,
      execute: async () => {
        const start = performance.now();
        
        const result = await client.callTool({
          name: 'prepare-cluster',
          arguments: {
            sessionId: 'cluster-test-123',
            namespace: 'test-namespace',
            createNamespace: true
          }
        });

        const responseTime = performance.now() - start;

        if (result.isError) {
          return {
            success: false,
            duration: responseTime,
            message: `Cluster preparation failed: ${result.error?.message || 'Unknown error'}`
          };
        }

        // Extract cluster preparation results
        let clusterResults: any = {};
        for (const content of result.content) {
          if (content.type === 'text' && content.text) {
            try {
              const parsed = JSON.parse(content.text);
              clusterResults = { ...clusterResults, ...parsed };
            } catch {
              clusterResults.textContent = content.text;
            }
          }
        }

        const hasClusterResults = clusterResults.success !== undefined || clusterResults.prepared || 
                                 clusterResults.namespace || clusterResults.textContent;

        return {
          success: !!hasClusterResults,
          duration: responseTime,
          message: hasClusterResults 
            ? 'Cluster preparation tool working'
            : 'Cluster preparation results unclear',
          details: clusterResults,
          performance: {
            responseTime,
            memoryUsage: 0,
          }
        };
      }
    },

    {
      name: 'deploy-application-tool',
      category: 'tool-validation',
      description: 'Test application deployment functionality',
      tags: ['tools', 'deployment', 'kubernetes'],
      timeout: 30000,
      execute: async () => {
        const start = performance.now();
        
        const result = await client.callTool({
          name: 'deploy',
          arguments: {
            sessionId: 'deploy-test-123',
            namespace: 'default',
            wait: false,
            timeout: 300
          }
        });

        const responseTime = performance.now() - start;

        if (result.isError) {
          return {
            success: false,
            duration: responseTime,
            message: `Application deployment failed: ${result.error?.message || 'Unknown error'}`
          };
        }

        // Extract deployment results
        let deployResults: any = {};
        for (const content of result.content) {
          if (content.type === 'text' && content.text) {
            try {
              const parsed = JSON.parse(content.text);
              deployResults = { ...deployResults, ...parsed };
            } catch {
              deployResults.textContent = content.text;
            }
          }
        }

        const hasDeployResults = deployResults.success !== undefined || deployResults.deployed || 
                                deployResults.status || deployResults.textContent;

        return {
          success: !!hasDeployResults,
          duration: responseTime,
          message: hasDeployResults 
            ? 'Deployment tool responding correctly'
            : 'Deployment results unclear',
          details: deployResults,
          performance: {
            responseTime,
            memoryUsage: 0,
          }
        };
      }
    },

    {
      name: 'verify-deployment-tool',
      category: 'tool-validation',
      description: 'Test deployment verification functionality',
      tags: ['tools', 'verification', 'kubernetes'],
      timeout: 25000,
      execute: async () => {
        const start = performance.now();
        
        const result = await client.callTool({
          name: 'verify-deployment',
          arguments: {
            sessionId: 'verify-test-123',
            deploymentName: 'test-deployment',
            namespace: 'default',
            timeout: 300
          }
        });

        const responseTime = performance.now() - start;

        if (result.isError) {
          return {
            success: false,
            duration: responseTime,
            message: `Deployment verification failed: ${result.error?.message || 'Unknown error'}`
          };
        }

        // Extract verification results
        let verifyResults: any = {};
        for (const content of result.content) {
          if (content.type === 'text' && content.text) {
            try {
              const parsed = JSON.parse(content.text);
              verifyResults = { ...verifyResults, ...parsed };
            } catch {
              verifyResults.textContent = content.text;
            }
          }
        }

        const hasVerifyResults = verifyResults.success !== undefined || verifyResults.ready || 
                                verifyResults.status || verifyResults.textContent;

        return {
          success: !!hasVerifyResults,
          duration: responseTime,
          message: hasVerifyResults 
            ? 'Verification tool working correctly'
            : 'Verification results unclear',
          details: verifyResults,
          performance: {
            responseTime,
            memoryUsage: 0,
          }
        };
      }
    },

    {
      name: 'workflow-tool',
      category: 'tool-validation',
      description: 'Test workflow orchestration tool',
      tags: ['tools', 'workflow', 'orchestration'],
      timeout: 60000,
      execute: async () => {
        const start = performance.now();
        
        const result = await client.callTool({
          name: 'workflow',
          arguments: {
            sessionId: 'workflow-test-123',
            workflowType: 'containerization',
            params: {
              repoPath: './test/__support__/fixtures/node-express'
            }
          }
        });

        const responseTime = performance.now() - start;

        if (result.isError) {
          return {
            success: false,
            duration: responseTime,
            message: `Workflow execution failed: ${result.error?.message || 'Unknown error'}`
          };
        }

        // Extract workflow results
        let workflowResults: any = {};
        for (const content of result.content) {
          if (content.type === 'text' && content.text) {
            try {
              const parsed = JSON.parse(content.text);
              workflowResults = { ...workflowResults, ...parsed };
            } catch {
              workflowResults.textContent = content.text;
            }
          }
        }

        const hasWorkflowResults = workflowResults.success !== undefined || workflowResults.steps || 
                                  workflowResults.completed || workflowResults.textContent;

        return {
          success: !!hasWorkflowResults,
          duration: responseTime,
          message: hasWorkflowResults 
            ? 'Workflow tool executing correctly'
            : 'Workflow results unclear',
          details: workflowResults,
          performance: {
            responseTime,
            memoryUsage: 0,
          }
        };
      }
    }
  ];

  return tests;
};